//go:build linux
// +build linux

package model

import (
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
