//go:build linux
// +build linux

package executor

// #cgo LDFLAGS: -lseccomp
/*
#include "executor.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"io"
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

// PrepareEnvironmentAndCompile 处理语言检查、临时目录创建、源代码写入和编译。
// 返回语言配置、工作目录、编译结果以及任何发生的错误。
// @param req model.SubmitRequest 包含用户提交的代码和相关信息
// @return lang model.Language 匹配到的语言配置
// @return workDir string 创建的临时工作目录路径
// @return compileRes model.CompilationResult 编译结果
// @return err error 执行过程中发生的错误
func PrepareEnvironmentAndCompile(ctx context.Context, req model.SubmitRequest) (
	lang model.Language,
	workDir string,
	compileRes model.CompilationResult,
	err error,
) {
	// 语言配置查找阶段：遍历所有支持的语言配置，匹配请求中的语言ID
	found := false
	for _, l := range language.GetLanguages() {
		if l.ID == req.LanguageID {
			lang = l
			found = true
			break
		}
	}
	if !found {
		err = errors.New("language not found")
		return
	}

	// 临时目录创建阶段：使用互斥锁保证目录创建的原子性，创建随机名称的临时工作目录
	folderLock.Lock()
	// 假设 utils.RandomString 存在，类似于 ProcessJob 中使用的 random.String。
	// 如果 utils.RandomString 不可用，则需要替换为项目实际的随机字符串生成器。
	var folderName string
	// 注意: 这里的 random.String(6) 需要确认是否为 utils.RandomString(6)
	// 根据 ProcessJob 的上下文，应该是 utils.RandomString
	// 如果 random 是一个标准库或者项目内定义的包，请相应调整
	// 为保持与 ProcessJob 中逻辑的一致性，暂时保留 random.String，但建议后续确认为 utils.RandomString
	// 修正：根据上下文，应该是 utils.RandomString
	folderName = utils.RandomString(6) // 使用 utils 包下的 RandomString
	err = utils.EnsureDir("tem")
	if err != nil {
		folderLock.Unlock()
		workDir = "" // 确保在错误时 workDir 为空，以便 defer os.RemoveAll 不会误删
		return
	}
	workDir = filepath.Join("tem", folderName)
	err = os.Mkdir(workDir, 0755)
	if err != nil {
		folderLock.Unlock()
		workDir = "" // 确保在错误时 workDir 为空
		return
	}
	folderLock.Unlock() // 在成功创建或 Mkdir 错误处理后解锁

	// 源码写入阶段：将用户提交的源代码写入配置指定的文件中
	sourceFilePath := filepath.Join(workDir, lang.SourceFile)
	if err = os.WriteFile(sourceFilePath, []byte(req.SourceCode), 0644); err != nil {
		// 如果写入失败，workDir 仍然有效，调用者应该负责清理
		return
	}

	// 编译阶段：如果语言需要编译，执行编译命令并处理编译结果
	if strings.TrimSpace(lang.CompileCmd) != "" {
		compileCmdStr := fmt.Sprintf(lang.CompileCmd, "") // 假设 CompileCmd 可能为将来使用留有占位符
		limiter := model.Limiter{
			CpuTime: conf.Conf.Executor.CompileTimeout,
			Memory:  uint(conf.Conf.Executor.CompileMemory),
		}
		compileRunExe := GetRunExecutor(compileCmdStr, limiter, workDir, false)

		runJob := NewRunJob(compileRunExe, ctx)

		exeRes := GetRunManagerInstance().SubmitRunJob(runJob)

		compileRes = model.CompilationResult{
			Success:     exeRes.Status.Id == model.StatusAC, // 根据编译结果设置编译成功状态
			Message:     exeRes.Message,                     // 编译消息
			Output:      exeRes.Stdout,
			CompileTime: exeRes.Time,
		}

		if !compileRes.Success {
			// 如果编译失败，根据 compileRes.Message 设置错误
			// 调用者将检查 compileRes.Success
			if compileRes.Message != "" {
				err = errors.New(compileRes.Message) // 将编译消息作为错误传播
			} else {
				err = errors.New("compilation failed without specific message") // 通用编译错误
			}
			return // 在此返回，workDir 已设置，供调用者清理
		}
	} else {
		compileRes.Success = true // 不需要编译，因此视为“成功”
	}

	return lang, workDir, compileRes, nil
}

// GetRunExecutor 创建并返回一个执行命令的闭包函数，该闭包会：
//   - 根据配置创建带资源限制的执行器
//   - 处理标准输入输出管道
//   - 执行命令并监控资源使用
//   - 根据执行结果设置对应的状态码和消息
//
// 参数:
//   - command: 需要执行的完整命令字符串
//   - limiter: 资源限制配置（CPU时间单位：秒，内存单位：KB）
//   - dir: 命令执行的工作目录
//   - runFlag: 标识是否运行模式（影响CGO执行器行为）
//   - stdin: 可变参数，可传递单个io.Reader作为标准输入
//
// 返回值:
//   - func(context.Context) model.RunResult: 接收上下文返回执行结果的闭包函数
func GetRunExecutor(command string, limiter model.Limiter, dir string, runFlag bool, stdin ...io.Reader) func(context.Context) model.RunResult {
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
		RunFlag: runFlag,
	}

	// 返回实际执行测试用例的闭包函数
	return func(ctx context.Context) (res model.RunResult) {
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
		if len(stdin) > 0 {
			_, err = exePipe.In.CopyFrom(stdin[0])
		}
		if err != nil {
			res.Status = model.StatusIE.GetStatus()
			res.Message = fmt.Sprintf("write stdin pipe failed: %v", err.Error())
			return
		}
		exePipe.In.Writer.Close()

		// 执行目标程序并获取结果
		pid, err := ProcessExecutor(runExe)

		if err != nil {
			res.Status = model.StatusIE.GetStatus()
			res.Message = fmt.Sprintf("run executor failed: %v", err.Error())
			return
		}

		var exeRes model.ExecutorResult

		monitorProcess(ctx, pid, &exeRes)

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
		case exeRes.ExitCode == -1:
			res.Status = model.StatusIE.GetStatus()
			res.Message = "context canceled"
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

		return
	}
}

func monitorProcess(ctx context.Context, pid int, result *model.ExecutorResult) {
	done := make(chan struct{})
	var status syscall.WaitStatus
	var rusage syscall.Rusage

	// 启动goroutine等待进程结束
	go func() {
		_, _ = syscall.Wait4(pid, &status, 0, &rusage)
		close(done)
	}()

	select {
	case <-ctx.Done():
		// 上下文被取消时发送SIGKILL
		_ = syscall.Kill(pid, syscall.SIGKILL)
		<-done // 确保进程状态被正确回收
		result.ExitCode = -1
		result.Signal = syscall.SIGKILL
	case <-done:
		if status.Exited() {
			result.ExitCode = status.ExitStatus()
		} else if status.Signaled() {
			result.Signal = status.Signal()
		}
	}
	// 正常处理结果
	userTime := float64(rusage.Utime.Sec) + float64(rusage.Utime.Usec)/1e6
	sysTime := float64(rusage.Stime.Sec) + float64(rusage.Stime.Usec)/1e6
	result.Time = userTime + sysTime
	result.Memory = uint(rusage.Maxrss)

}

// ProcessExecutor 执行运行器
func ProcessExecutor(executor model.Executor) (int, error) {
	cExe := ExecutorGo2C(executor)
	defer C.free(unsafe.Pointer(cExe.Dir))
	defer C.free(unsafe.Pointer(cExe.Command))
	exitCode := C.Execute(cExe)
	if int32(exitCode) == 0 {
		return 0, errors.New("executor error")
	}
	return int(exitCode), nil
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

func init() {
	// 初始化过滤器
	C.InitFilter()
	// 删除临时文件夹
	os.RemoveAll("tem")
}
