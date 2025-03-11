//go:build linux
// +build linux

package model

import (
	"errors"
	"os"
	"syscall"
)

// 代码运行器闭包，内含一个 Executor 结构体模版，可以以这个模版为基础运行不同的Testcase
type RunExe func(testcase Testcase) TestResult

// TestcaseType 表示测试数据类型
type TestcaseType int

const (
	SingleTest   TestcaseType = 0 // 单个测试数据，使用SubmitRequest下的Stdin与ExpectedOutput字段
	MultipleTest TestcaseType = 1 // 多个测试数据，使用SubmitRequest下的Testcase字段
	FileTest     TestcaseType = 2 // 文件测试数据，暂未实现
)

// Job 表示评测任务，由任务协程池调度执行
type Job struct {
	Request  SubmitRequest
	RespChan chan JudgeResult
}

// RunJob 表示运行任务，由运行协程池调度执行
type RunJob struct {
	RunFunc  RunExe
	Testcase Testcase
	RespChan chan TestResult
}

// Testcase 表示测试数据
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

// CompilationResult 表示编译结果
type CompilationResult struct {
	Success     bool    `json:"success"`
	Output      string  `json:"output"`       // 编译输出（包括警告）
	CompileTime float64 `json:"compile_time"` // 编译耗时（秒）
	Message     string  `json:"message"`
}

// TestResult 表示单个测试结果
type TestResult struct {
	Status  Status  `json:"status"`
	Stderr  string  `json:"stderr"` // 运行时错误信息
	Stdout  string  `json:"stdout"`
	Message string  `json:"message"`
	Time    float64 `json:"time"`   // 执行时间（秒）
	Memory  uint    `json:"memory"` // 内存消耗（KB）
}

// JudgeResult 表示一次任务的评测结果
type JudgeResult struct {
	Compilation CompilationResult `json:"compilation"`
	TestResult  []TestResult      `json:"test_result"`
	MaxTime     float64           `json:"max_time"`
	MaxMemory   uint              `json:"max_memory"`
	Status      Status            `json:"status"`
	Message     string            `json:"message"`
}

// Limiter 表示评测限制
type Limiter struct {
	CpuTime float64
	Memory  uint
}

// ExecutorResult 表示运行结果
type ExecutorResult struct {
	ExitCode int
	Memory   uint
	Time     float64
	Signal   syscall.Signal
}

// Executor 表示运行器
type Executor struct {
	Command string
	Limiter Limiter
	Dir     string
	Stdin   *os.File
	Stdout  *os.File
	Stderr  *os.File
	RunFlag bool
}

// ExecutorPipe 表示运行器的管道组
type ExecutorPipe struct {
	In  *Pipe
	Out *Pipe
	Err *Pipe
}

// Close 关闭管道组
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

// NewExecutorPipe 创建一个运行器的管道组
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
