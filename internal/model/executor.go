//go:build linux
// +build linux

package model

import (
	"errors"
	"os"
	"syscall"
)

type RunExe func(testcase Testcase) TestResult

type TestcaseType int

const (
	SingleTest   TestcaseType = 0
	MultipleTest TestcaseType = 1
	FileTest     TestcaseType = 2
)

// Job 表示评测任务，由工作协程池调度执行
type Job struct {
	Request  SubmitRequest
	RespChan chan JudgeResult
}

type RunJob struct {
	RunFunc  RunExe
	Testcase Testcase
	RespChan chan TestResult
}

type Testcase struct {
	Stdin          string `json:"stdin,omitempty"`
	ExpectedOutput string `json:"expected_output,omitempty"`
}

// SubmitRequest 表示提交评测时的请求体
type SubmitRequest struct {
	SourceCode     string       `json:"source_code"`
	Stdin          string       `json:"stdin,omitempty"`
	ExpectedOutput string       `json:"expected_output,omitempty"`
	CpuTimeLimit   float64      `json:"cpu_time_limit,omitempty"`
	MemoryLimit    uint         `json:"memory_limit,omitempty"`
	LanguageID     int          `json:"language_id"`
	Testcase       []Testcase   `json:"test_case,omitempty"`
	TestcaseType   TestcaseType `json:"test_case_type,omitempty"`
}

type CompilationResult struct {
	Success     bool    `json:"success"`
	Output      string  `json:"output"`       // 编译输出（包括警告）
	CompileTime float64 `json:"compile_time"` // 编译耗时（秒）
	Message     string  `json:"message"`
}

type TestResult struct {
	Status  Status  `json:"status"`
	Stderr  string  `json:"stderr"` // 运行时错误信息
	Stdout  string  `json:"stdout"`
	Message string  `json:"message"`
	Time    float64 `json:"time"`   // 执行时间（秒）
	Memory  uint    `json:"memory"` // 内存消耗（KB）
}

type JudgeResult struct {
	Compilation CompilationResult `json:"compilation"`
	TestResult  []TestResult      `json:"test_result"`
	MaxTime     float64           `json:"max_time"`
	MaxMemory   uint              `json:"max_memory"`
	Status      Status            `json:"status"`
	Message     string            `json:"message"`
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
