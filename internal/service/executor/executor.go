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
	"sync"
	"syscall"
	"time"
	"unsafe"
)

func Init() {
	// 初始化任务队列
	jobQueue = make(chan *model.Job, 100)

	// 启动协程池，数量由配置决定
	for i := 0; i < conf.Conf.Executor.Pool; i++ {
		go worker(i, jobQueue)
	}
}

// worker 不断从jobQueue中取任务执行
func worker(id int, jobs <-chan *model.Job) {
	for job := range jobs {
		result := ProcessJob(job.Request)
		job.RespChan <- result
	}
}

// processJob 执行一次代码评测
func ProcessJob(req model.SubmitRequest) (res model.Result) {
	var err error
	// 查找对应的语言配置
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

	var folderName string
	folderName = utils.RandomString(6)
	err = os.Mkdir(folderName, 0755)
	if err != nil {
		res.Status = model.StatusIE.GetStatus()
		res.Message = err.Error()
		return
	}
	defer os.RemoveAll(folderName)

	// 将源代码写入文件
	sourceFilePath := filepath.Join(folderName, lang.SourceFile)
	if err := os.WriteFile(sourceFilePath, []byte(req.SourceCode), 0644); err != nil {
		res.Status = model.StatusIE.GetStatus()
		res.Message = err.Error()
		return
	}

	// 如语言配置中有编译命令，则先进行编译
	if strings.TrimSpace(lang.CompileCmd) != "" {
		// 这里将编译命令中的 %s 替换为-w（忽略warnings）
		compileCmdStr := fmt.Sprintf(lang.CompileCmd, "-w")
		if err := CompileExecutor(compileCmdStr, folderName); err != nil {
			res.Status = model.StatusIE.GetStatus()
			res.CompileOutput = err.Error()
			return
		}
	}

	runExe := GetRunExecutor(lang.RunCmd,
		model.Limiter{
			CpuTime: conf.Conf.Executor.CPUTimeLimit,
			Memory:  conf.Conf.Executor.MemoryLimit,
		},
		folderName)
	res = runExe(req.Stdin, req.ExpectedOutput)
	return
}

func GetRunExecutor(command string, limiter model.Limiter, dir string) func(stdin, expectedOutput string) model.Result {
	if limiter.CpuTime == 0 {
		limiter.CpuTime = conf.Conf.Executor.CPUTimeLimit
	}
	if limiter.Memory == 0 {
		limiter.Memory = conf.Conf.Executor.MemoryLimit
	}
	exeTemplate := model.Executor{
		Command: command,
		Dir:     dir,
		Limiter: limiter,
		RunFlag: true,
	}
	return func(stdin, expectedOutput string) (res model.Result) {
		exePipe, err := model.NewExecutorPipe()
		defer exePipe.Close()
		if err != nil {
			res.Status = model.StatusIE.GetStatus()
			res.Message = fmt.Sprintf("new executor pipe failed: %v", err.Error())
			return
		}
		runExe := exeTemplate
		runExe.Stdin = exePipe.In.Reader
		runExe.Stdout = exePipe.Out.Writer
		runExe.Stderr = exePipe.Err.Writer
		_, err = exePipe.In.Write(stdin)
		if err != nil {
			res.Status = model.StatusIE.GetStatus()
			res.Message = fmt.Sprintf("write stdin pipe failed: %v", err.Error())
			return
		}
		defer exePipe.In.Writer.Close()
		var stdout, stderr string
		var wg sync.WaitGroup
		var ch = make(chan bool)
		wg.Add(2)

		// 异步读取标准输出
		go func() {
			defer wg.Done()
			stdout, err = exePipe.Out.Read()
			if err != nil {
				res.Status = model.StatusIE.GetStatus()
				res.Message = fmt.Sprintf("read stdout pipe failed: %v", err.Error())
				return
			}
		}()

		// 异步读取标准错误
		go func() {
			defer wg.Done()
			stderr, err = exePipe.Err.Read()
			if err != nil {
				res.Status = model.StatusIE.GetStatus()
				res.Message = fmt.Sprintf("read stderr pipe failed: %v", err.Error())
			}
		}()

		var exeRes model.ExecutorResult

		go func() {
			exeRes, err = ProcessExecutor(runExe)
			if err != nil {
				res.Status = model.StatusIE.GetStatus()
				res.Message = fmt.Sprintf("run executor failed: %v", err.Error())
				return
			}

			wg.Wait()
			ch <- false
		}()
		select {
		case <-time.After(time.Duration(runExe.Limiter.CpuTime+conf.Conf.Executor.ExtraCPUTime) * time.Second):
		case <-ch:
		}
		res.Stdout = stdout
		res.Stderr = stderr
		res.Time = exeRes.Time
		res.Memory = exeRes.Memory
		if exeRes.ExitCode == 3 {
			res.Status = model.StatusIE.GetStatus()
			res.Message = "stderr pipe setup failed."
			return
		}
		if exeRes.ExitCode == 2 {
			res.Status = model.StatusIE.GetStatus()
			res.Message = stderr
			return
		}
		if exeRes.Time > runExe.Limiter.CpuTime {
			res.Status = model.StatusTLE.GetStatus()
			return
		}
		if exeRes.Memory > runExe.Limiter.Memory*1024 {
			res.Status = model.StatusRESIGSEGV.GetStatus()
			return
		}
		if exeRes.Signal != 0 {
			res.Status = SignalStatus(exeRes.Signal).GetStatus()
			res.Message = SignalMessage(exeRes.Signal)
			return
		}

		res.Status = model.StatusAC.GetStatus()

		if expectedOutput != "" {
			if !utils.StringsEqualIgnoreFinalNewline(stdout, expectedOutput) {
				res.Status = model.StatusWA.GetStatus()
				return
			}
		}

		return
	}
}

func CompileExecutor(compileCmd, dir string) (err error) {
	comPipe, err := model.NewExecutorPipe()
	defer comPipe.Close()
	if err != nil {
		return
	}

	if strings.TrimSpace(compileCmd) == "" {
		err = errors.New("compile command is empty")
		return
	}
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

	stderr := make([]byte, 1024)
	var stderrN int
	stdchan := make(chan bool)

	// 启动标准错误读取协程
	go func() {
		stderrN, _ = comPipe.Err.Reader.Read(stderr)
		stdchan <- true
	}()

	res, err := ProcessExecutor(executor)

	// 使用 select 实现超时控制
	select {
	case <-stdchan:
		// 标准错误读取完成
	case <-time.After(time.Second * time.Duration(conf.Conf.Executor.CompileTimeout)):
		// return errors.New("timeout reading from stderr")
	}

	if err != nil {
		return
	}

	if res.ExitCode != 0 {
		return errors.New(string(stderr[:stderrN]))
	}
	if res.Signal != 0 {
		return errors.New(SignalMessage(res.Signal))
	}
	return
}

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

func ResultC2GO(result *C.Result) model.ExecutorResult {
	return model.ExecutorResult{
		ExitCode: int(result.ExitCode),
		Memory:   uint(result.Memory),
		Signal:   syscall.Signal(result.Signal),
		Time:     float64(result.Time),
	}
}

func init() {
	C.InitFilter()
}
