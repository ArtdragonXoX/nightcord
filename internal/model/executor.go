//go:build linux
// +build linux

package model

import (
	"errors"
	"os"
	"syscall"
)

// Job 表示评测任务，由工作协程池调度执行
type Job struct {
	Request  SubmitRequest
	RespChan chan Result
}

// SubmitRequest 表示提交评测时的请求体
type SubmitRequest struct {
	SourceCode     string  `json:"source_code"`
	Stdin          string  `json:"stdin"`
	ExpectedOutput string  `json:"expected_output"`
	CpuTimeLimit   float64 `json:"cpu_time_limit"`
	MemoryLimit    uint    `json:"memory_limit"`
	LanguageID     int     `json:"language_id"`
}

// Result 为评测结果返回格式
type Result struct {
	Stdout        string  `json:"stdout"`
	Stderr        string  `json:"stderr"`
	CompileOutput string  `json:"compile_output"`
	Message       string  `json:"message"`
	Status        Status  `json:"status"`
	Time          float64 `json:"time"`
	Memory        uint    `json:"memory"`
}

type Limiter struct {
	CpuTime float64
	Memory  uint
}

type ExecutorResult struct {
	ExitCode int
	Memory   uint
	Time     float64
	Signal   syscall.Signal
}

type Executor struct {
	Command string
	Limiter Limiter
	Dir     string
	Stdin   *os.File
	Stdout  *os.File
	Stderr  *os.File
	RunFlag bool
}

type ExecutorPipe struct {
	In  *Pipe
	Out *Pipe
	Err *Pipe
}

func (p *ExecutorPipe) Close() error {
	var err error
	if terr := p.In.Close(); terr != nil {
		err = errors.Join(err, terr)
	}
	if terr := p.Out.Close(); terr != nil {
		err = errors.Join(err, terr)
	}
	if terr := p.Err.Close(); terr != nil {
		err = errors.Join(err, terr)
	}
	return err
}

func NewExecutorPipe() (*ExecutorPipe, error) {
	var err error
	in, err := NewPipe()
	if err != nil {
		defer in.Close()
		return nil, err
	}
	out, err := NewPipe()
	if err != nil {
		defer out.Close()
		return nil, err
	}
	errPipe, err := NewPipe()
	if err != nil {
		defer errPipe.Close()
		return nil, err
	}
	return &ExecutorPipe{
		In:  in,
		Out: out,
		Err: errPipe,
	}, nil
}
