//go:build linux
// +build linux

package executor

// #cgo LDFLAGS: -lseccomp
/*
#include "executor.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"nightcord-server/internal/conf"
	"nightcord-server/internal/model"
	"nightcord-server/internal/service/language"
	"nightcord-server/utils"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func Init() {
	// 初始化任务队列
	jobQueue = make(chan *model.Job, 100)

	// 启动任务协程池，数量由配置决定
	for range conf.Conf.Executor.JobPool {
		go worker(jobQueue)
	}

	// 初始化运行队列
	runQueue = make(chan *model.RunJob, 500)
	// 启动运行协程池，数量由配置决定
	for range conf.Conf.Executor.RunPool {
		go runWorker(runQueue)
	}
}

// worker 不断从jobQueue中取任务执行
func worker(jobs <-chan *model.Job) {
	for job := range jobs {
		result := ProcessJob(job.Request)
		job.RespChan <- result
	}
}

// runWorker 不断从runQueue中取任务执行
func runWorker(jobs <-chan *model.RunJob) {
	for job := range jobs {
		res := job.RunFunc(job.Testcase)
		job.RespChan <- res
	}
}

// ProcessJob 处理代码判题任务，包括编译、执行和结果收集
// 参数 req: SubmitRequest 包含用户提交的代码、语言ID、资源限制等信息
// 返回值 model.JudgeResult: 包含编译结果、测试用例执行结果、最大资源消耗等判题结果
func ProcessJob(req model.SubmitRequest) (res model.JudgeResult) {
	var err error

	// 语言配置查找阶段：遍历所有支持的语言配置，匹配请求中的语言ID
	var lang model.Language
	found := false
	for _, l := range language.GetLanguages() {
		if l.ID == req.LanguageID {
			lang = l
			found = true
			break
		}
	}
	if !found {
		res.Status = model.StatusIE.GetStatus()
		msg := "language not found"
		res.Message = msg
		return
	}

	// 临时目录创建阶段：使用互斥锁保证目录创建的原子性，创建随机名称的临时工作目录
	folderLock.Lock()
	var folderName string
	folderName = random.String(6)
	err = utils.EnsureDir("tem")
	if err != nil {
		res.Status = model.StatusIE.GetStatus()
		res.Message = err.Error()
		folderLock.Unlock()
		return
	}
	folderName = fmt.Sprintf("%s/%s", "tem", folderName)
	err = os.Mkdir(folderName, 0755)
	if err != nil {
		res.Status = model.StatusIE.GetStatus()
		res.Message = err.Error()
		folderLock.Unlock()
		return
	}
	defer os.RemoveAll(folderName)
	folderLock.Unlock()

	// 源码写入阶段：将用户提交的源代码写入配置指定的文件中
	sourceFilePath := filepath.Join(folderName, lang.SourceFile)
	if err := os.WriteFile(sourceFilePath, []byte(req.SourceCode), 0644); err != nil {
		res.Status = model.StatusIE.GetStatus()
		res.Message = err.Error()
		return
	}

	// 编译阶段：如果语言需要编译，执行编译命令并处理编译结果
	if strings.TrimSpace(lang.CompileCmd) != "" {
		compileCmdStr := fmt.Sprintf(lang.CompileCmd, "")
		complieRes := CompileExecutor(compileCmdStr, folderName)
		res.Compilation = complieRes
		if !complieRes.Success {
			if complieRes.Message != "" {
				res.Message = complieRes.Message
				res.Status = model.StatusIE.GetStatus()
				return
			} else {
				res.Status = model.StatusCE.GetStatus()
				return
			}
		}
	}

	// 执行阶段：创建带有资源限制的执行器并提交测试任务
	runExe := GetRunExecutor(lang.RunCmd,
		model.Limiter{
			CpuTime: req.CpuTimeLimit,
			Memory:  req.MemoryLimit,
		},
		folderName)

	testreses := SubmitExeJob(runExe, req)

	// 结果处理阶段：收集所有测试结果，计算最终状态和最大资源消耗
	res.TestResult = testreses
	for _, testres := range res.TestResult {
		if testres.Status.Id > res.Status.Id {
			res.Status = testres.Status
		}
		if testres.Time > res.MaxTime {
			res.MaxTime = testres.Time
		}
		if testres.Memory > res.MaxMemory {
			res.MaxMemory = testres.Memory
		}
	}

	return
}

// GetRunExecutor 生成并返回一个执行器运行函数，用于处理测试用例并返回测试结果
// 参数:
//   - command: 需要执行的命令字符串
//   - limiter: 资源限制器，包含CPU时间和内存限制（单位：KB）
//   - dir: 执行命令的工作目录
//
// 返回值:
//   - model.RunExe: 接收测试用例返回测试结果的函数类型
func GetRunExecutor(command string, limiter model.Limiter, dir string) model.RunExe {
	// 设置默认资源限制值（当未指定时使用配置中的默认值）
	if limiter.CpuTime == 0 {
		limiter.CpuTime = conf.Conf.Executor.CPUTimeLimit
	}
	if limiter.Memory == 0 {
		limiter.Memory = conf.Conf.Executor.MemoryLimit
	}

	// 创建基础执行器模板
	exeTemplate := model.Executor{
		Command: command,
		Dir:     dir,
		Limiter: limiter,
		RunFlag: true,
	}

	// 返回实际执行测试用例的闭包函数
	return func(testcase model.Testcase) (res model.TestResult) {
		// 创建管道用于进程间通信
		exePipe, err := model.NewExecutorPipe()
		defer exePipe.Close()
		if err != nil {
			res.Status = model.StatusIE.GetStatus()
			res.Message = fmt.Sprintf("new executor pipe failed: %v", err.Error())
			return
		}

		// 配置执行器的输入输出管道
		runExe := exeTemplate
		runExe.Stdin = exePipe.In.Reader
		runExe.Stdout = exePipe.Out.Writer
		runExe.Stderr = exePipe.Err.Writer

		// 将测试用例输入写入管道
		_, err = exePipe.In.Write(testcase.Stdin)
		if err != nil {
			res.Status = model.StatusIE.GetStatus()
			res.Message = fmt.Sprintf("write stdin pipe failed: %v", err.Error())
			return
		}
		exePipe.In.Writer.Close()

		// 执行目标程序并获取结果
		exeRes, err := ProcessExecutor(runExe)
		if err != nil {
			res.Status = model.StatusIE.GetStatus()
			res.Message = fmt.Sprintf("run executor failed: %v", err.Error())
			return
		}

		// 关闭输出管道并读取结果
		exePipe.Out.Writer.Close()
		exePipe.Err.Writer.Close()

		// 处理错误输出
		res.Stderr, err = exePipe.Err.Read()
		if err != nil {
			res.Status = model.StatusIE.GetStatus()
			res.Message = fmt.Sprintf("read stderr pipe failed: %v", err.Error())
		}

		// 处理标准输出
		res.Stdout, err = exePipe.Out.Read()
		if err != nil {
			res.Status = model.StatusIE.GetStatus()
			res.Message = fmt.Sprintf("read stdout pipe failed: %v", err.Error())
			return
		}

		// 设置执行时间和内存消耗
		res.Time = exeRes.Time
		res.Memory = exeRes.Memory

		// 根据退出码和信号判断执行状态
		switch {
		case exeRes.ExitCode == 3:
			res.Status = model.StatusIE.GetStatus()
			res.Message = "stderr pipe setup failed."
		case exeRes.ExitCode == 2:
			res.Status = model.StatusIE.GetStatus()
			res.Message = res.Stderr
		case exeRes.Time > runExe.Limiter.CpuTime:
			res.Status = model.StatusTLE.GetStatus()
		case exeRes.Memory > runExe.Limiter.Memory*1024:
			res.Status = model.StatusRESIGSEGV.GetStatus()
		case exeRes.Signal != 0:
			res.Status = SignalStatus(exeRes.Signal).GetStatus()
			res.Message = SignalMessage(exeRes.Signal)
		default:
			res.Status = model.StatusAC.GetStatus()
		}

		// 验证输出结果是否符合预期
		if testcase.ExpectedOutput != "" && res.Status.Id == model.StatusAC {
			if !utils.StringsEqualIgnoreFinalNewline(res.Stdout, testcase.ExpectedOutput) {
				res.Status = model.StatusWA.GetStatus()
			}
		}

		return
	}
}

// CompileExecutor 执行编译命令并返回编译结果
// 参数:
//   - compileCmd: 字符串类型，需要执行的编译命令
//   - dir: 字符串类型，执行命令的工作目录
//
// 返回值:
//   - model.CompilationResult: 包含编译时间、输出信息、状态等结果的结构体
func CompileExecutor(compileCmd, dir string) (res model.CompilationResult) {
	// 创建执行器通信管道，用于捕获标准输出/错误
	comPipe, err := model.NewExecutorPipe()
	defer comPipe.Close()
	if err != nil {
		res.Message = err.Error()
		return
	}

	// 检查编译命令有效性
	if strings.TrimSpace(compileCmd) == "" {
		res.Message = "compile command is empty"
		return
	}

	// 构建执行器配置
	executor := model.Executor{
		Command: compileCmd,
		Dir:     dir,
		Limiter: model.Limiter{
			CpuTime: conf.Conf.Executor.CompileTimeout,
			Memory:  uint(conf.Conf.Executor.CompileMemory),
		},
		Stdin:   comPipe.In.Reader,
		Stdout:  comPipe.Out.Writer,
		Stderr:  comPipe.Err.Writer,
		RunFlag: false,
	}

	// 执行编译命令并获取结果
	exeRes, err := ProcessExecutor(executor)
	if err != nil {
		return
	}

	// 关闭错误管道写入端以结束读取
	comPipe.Err.Writer.Close()
	stderr, err := comPipe.Err.Read()

	// 设置编译时间并处理不同退出状态
	res.CompileTime = exeRes.Time
	switch {
	case exeRes.ExitCode == 3:
		res.Message = "stderr pipe setup failed."
	case exeRes.ExitCode == 2:
		res.Message = stderr
	case exeRes.ExitCode != 0:
		res.Output = stderr
	case stderr != "":
		res.Output = stderr
	case exeRes.Signal != 0:
		res.Output = SignalMessage(exeRes.Signal)
	default:
		res.Success = true
	}
	return
}

// ProcessExecutor 执行运行器
func ProcessExecutor(executor model.Executor) (model.ExecutorResult, error) {
	cExe := ExecutorGo2C(executor)
	defer C.free(unsafe.Pointer(cExe.Dir))
	defer C.free(unsafe.Pointer(cExe.Command))
	exitCode := C.Execute(cExe)
	if int32(exitCode) != 0 {
		return model.ExecutorResult{}, errors.New("executor error")
	}
	return ResultC2GO(&cExe.Result), nil
}

// ExecutorGo2C 将运行器的go结构体转换为c结构体
func ExecutorGo2C(executor model.Executor) *C.Executor {
	return &C.Executor{
		Command: C.CString(executor.Command),
		Dir:     C.CString(executor.Dir),
		Limit: C.Limiter{
			CpuTime_cur: C.float(executor.Limiter.CpuTime),
			CpuTime_max: C.float(executor.Limiter.CpuTime + conf.Conf.Executor.ExtraCPUTime),
			Memory_cur:  C.int(executor.Limiter.Memory),
			Memory_max:  C.int(executor.Limiter.Memory),
		},
		StdinFd:  C.int(executor.Stdin.Fd()),
		StdoutFd: C.int(executor.Stdout.Fd()),
		StderrFd: C.int(executor.Stderr.Fd()),
		RunFlag:  C.int(utils.BoolToInt(executor.RunFlag)),
	}
}

// ResultC2GO 将运行器结果c结构体转换为go结构体
func ResultC2GO(result *C.Result) model.ExecutorResult {
	return model.ExecutorResult{
		ExitCode: int(result.ExitCode),
		Memory:   uint(result.Memory),
		Signal:   syscall.Signal(result.Signal),
		Time:     float64(result.Time),
	}
}

func init() {
	// 初始化过滤器
	C.InitFilter()
	// 删除临时文件夹
	os.RemoveAll("tem")
}
