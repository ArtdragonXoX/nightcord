//go:build linux
// +build linux

package model

import (
	"context"
	"io"
	"os/exec"
	"syscall"
	"time"
)

// Job 表示评测任务，由工作协程池调度执行
type Job struct {
	Request  SubmitRequest
	RespChan chan Result
}

// SubmitRequest 表示提交评测时的请求体
type SubmitRequest struct {
	SourceCode     string  `json:"source_code"`
	LanguageID     int     `json:"language_id"`
	Stdin          string  `json:"stdin"`
	ExpectedOutput string  `json:"expected_output"`
	CpuTimeLimit   float64 `json:"cpu_time_limit"`
	MemoryLimit    uint    `json:"memory_limit"`
}

// Result 为评测结果返回格式
type Result struct {
	Stdout        string  `json:"stdout"`
	Time          float64 `json:"time"`
	Memory        uint    `json:"memory"`
	Stderr        string  `json:"stderr"`
	CompileOutput string  `json:"compile_output"`
	Message       string  `json:"message"`
	Status        Status  `json:"status"`
}

type Limiter struct {
	CpuTime float64 // s
	Memory  uint    // kb
}

type ExecutorResult struct {
	Stdout   string
	Stderr   string
	Time     float64
	Memory   uint
	Signal   syscall.Signal
	ExitCode int
}

// 以单次运行为颗粒度执行程序
type Executor struct {
	Cmd         *exec.Cmd
	Limiter     Limiter
	RunCmdStr   string
	Dir         string
	Stdin       io.Reader
	Filter      *syscall.SockFprog
	Result      ExecutorResult
	cancel      context.CancelFunc
	stdoutBytes []byte
	stderrBytes []byte
}

func (e *Executor) Start() error {
	timeoutDuration := time.Duration(e.Limiter.CpuTime * float64(time.Second))
	var ctx context.Context
	ctx, e.cancel = context.WithTimeout(context.Background(), timeoutDuration)
	e.Cmd = exec.CommandContext(ctx, "bash", "-c", e.RunCmdStr)
	e.Cmd.Dir = e.Dir
	e.Cmd.Stdin = e.Stdin

	stdoutPipe, err := e.Cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderrPipe, err := e.Cmd.StderrPipe()
	if err != nil {
		return err
	}

	err = e.Cmd.Start()
	if err != nil {
		return err
	}

	e.stdoutBytes, err = io.ReadAll(stdoutPipe)
	if err != nil {
		return err
	}
	e.stderrBytes, err = io.ReadAll(stderrPipe)
	if err != nil {
		return err
	}

	return nil
}

func (e *Executor) Wait() error {
	err := e.Cmd.Wait()
	if err != nil {
		return err
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				e.Result.Signal = status.Signal()
			}
		}
	}

	e.Result.Stderr = string(e.stderrBytes)
	e.Result.Stdout = string(e.stdoutBytes)
	return nil
}

func (e *Executor) Cancel() {
	e.cancel()
}
