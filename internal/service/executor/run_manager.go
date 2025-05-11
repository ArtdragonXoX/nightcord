//go:build linux
// +build linux

package executor

import (
	"fmt"
	"nightcord-server/internal/conf"
	"nightcord-server/internal/model"
	"sync"
)

// RunJob 表示一个运行任务，包含单个测试用例、执行命令和资源限制等
// 它由 RunManager 调度执行
// @Description RunJob 包含单个测试用例、执行命令和资源限制等
// @Description 由 RunManager 调度执行
type RunJob struct {
	Testcase   model.Testcase        // 单个测试用例
	RunCommand string                // 实际执行的命令
	WorkDir    string                // 工作目录
	Limiter    model.Limiter         // 资源限制器
	RespChan   chan model.TestResult // 用于返回单个测试用例的判题结果
}

// RunManager 管理运行任务的执行
// @Description RunManager 管理运行任务的执行
type RunManager struct {
	RunQueue      chan *RunJob       // 运行任务队列
	RunQueueNum   int                // 运行任务队列大小
	RunPoolNum    int                // 运行器池大小
	RunWorkers    map[int]*RunWorker // 运行器实例
	GlobalRunLock sync.Mutex         // 用于确保 RunManager 的全局实例
}

var (
	globalRunManager *RunManager
	onceRunManager   sync.Once
)

// GetRunManagerInstance 获取 RunManager 的单例
// @Description 获取 RunManager 的单例
// @Return *RunManager RunManager 的单例
func GetRunManagerInstance() *RunManager {
	onceRunManager.Do(func() {
		globalRunManager = NewRunManager(
			conf.Conf.Executor.RunPool, // 使用配置中的 RunPool 数量
			conf.Conf.Executor.RunQueue,
		)
		globalRunManager.Start()
	})
	return globalRunManager
}

// NewRunManager 创建一个新的 RunManager
// @Description 创建一个新的 RunManager
// @Param runPoolNum int 运行器池大小
// @Param runQueueNum int 运行任务队列大小
// @Return *RunManager 新的 RunManager 实例
func NewRunManager(runPoolNum, runQueueNum int) *RunManager {
	rm := &RunManager{
		RunQueue:    make(chan *RunJob, runQueueNum),
		RunQueueNum: runQueueNum,
		RunPoolNum:  runPoolNum,
		RunWorkers:  make(map[int]*RunWorker),
	}
	for i := 0; i < runPoolNum; i++ {
		runWorker := NewRunWorker(i, rm.RunQueue)
		rm.RunWorkers[i] = runWorker
	}
	return rm
}

// Start 启动 RunManager 的所有运行器
// @Description 启动 RunManager 的所有运行器
func (rm *RunManager) Start() {
	for _, runWorker := range rm.RunWorkers {
		runWorker.Start()
	}
}

// SubmitRunJob 提交一个新的运行任务到队列
// @Description 提交一个新的运行任务到队列
// @Param runJob *RunJob 要提交的运行任务
// @Return model.TestResult 单个测试用例的判题结果，如果队列满则返回错误信息
func (rm *RunManager) SubmitRunJob(runJob *RunJob) model.TestResult {
	select {
	case rm.RunQueue <- runJob:
		// 任务成功提交到队列，等待执行结果
		return <-runJob.RespChan
	default:
		// 任务队列已满，返回拒绝信息
		return model.TestResult{
			Status:  model.StatusIE.GetStatus(),
			Message: "Run job queue is full, please try again later.",
		}
	}
}

// Stop 停止 RunManager 的所有运行器
// @Description 停止 RunManager 的所有运行器
func (rm *RunManager) Stop() {
	for _, runWorker := range rm.RunWorkers {
		runWorker.Stop()
	}
}

// RunWorkerStatus 表示运行器的状态
// @Description RunWorkerStatus 表示运行器的状态
type RunWorkerStatus uint8

const (
	RunWorkerStatusIdle RunWorkerStatus = iota
	RunWorkerStatusRunning
	RunWorkerStatusStopped
)

// String 将 RunWorkerStatus 转换为字符串表示
// @Description 将 RunWorkerStatus 转换为字符串表示
// @Return string 状态的字符串表示
func (s RunWorkerStatus) String() string {
	switch s {
	case RunWorkerStatusIdle:
		return "Idle"
	case RunWorkerStatusRunning:
		return "Running"
	case RunWorkerStatusStopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}

// RunWorker 表示一个运行器，负责执行具体的运行任务
// @Description RunWorker 表示一个运行器，负责执行具体的运行任务
type RunWorker struct {
	Id          int
	CurrentJob  *RunJob
	RunQueue    <-chan *RunJob
	Status      RunWorkerStatus
	controlChan chan struct{} // 用于停止 worker
	jobFinish   chan struct{} // 任务完成信号
}

// NewRunWorker 创建一个新的 RunWorker
// @Description 创建一个新的 RunWorker
// @Param id int 运行器 ID
// @Param runQueue <-chan *RunJob 运行任务队列
// @Return *RunWorker 新的 RunWorker 实例
func NewRunWorker(id int, runQueue <-chan *RunJob) *RunWorker {
	return &RunWorker{
		Id:          id,
		RunQueue:    runQueue,
		Status:      RunWorkerStatusStopped,
		controlChan: make(chan struct{}),
		jobFinish:   make(chan struct{}),
	}
}

// Start 启动 RunWorker
// @Description 启动 RunWorker
func (rw *RunWorker) Start() {
	rw.Status = RunWorkerStatusIdle
	go rw.Run()
}

// Stop 停止 RunWorker
// @Description 停止 RunWorker
func (rw *RunWorker) Stop() {
	close(rw.controlChan) // 关闭控制通道以通知 Run 协程停止
}

// Run 是 RunWorker 的主循环，监听任务和控制信号
// @Description Run 是 RunWorker 的主循环，监听任务和控制信号
func (rw *RunWorker) Run() {
	for {
		var currentRunQueue <-chan *RunJob
		if rw.Status == RunWorkerStatusIdle {
			currentRunQueue = rw.RunQueue
		}

		select {
		case runJob, ok := <-currentRunQueue:
			if !ok { // 队列关闭
				rw.Status = RunWorkerStatusStopped
				return
			}
			rw.handleRunJob(runJob)
		case <-rw.controlChan:
			rw.Status = RunWorkerStatusStopped
			return
		case <-rw.jobFinish:
			rw.Status = RunWorkerStatusIdle
			rw.CurrentJob = nil
		}
	}
}

// handleRunJob 处理单个测试用例的运行任务
// @Description handleRunJob 处理单个测试用例的运行任务，并返回其结果
// @Param runJob *RunJob 要处理的运行任务（包含单个测试用例）
func (rw *RunWorker) handleRunJob(runJob *RunJob) {
	rw.Status = RunWorkerStatusRunning
	rw.CurrentJob = runJob

	go func() {
		defer func() {
			if r := recover(); r != nil {
				runJob.RespChan <- model.TestResult{
					Status:  model.StatusIE.GetStatus(),
					Message: fmt.Sprintf("RunWorker panic: %v", r),
				}
			}
			rw.jobFinish <- struct{}{}
		}()

		// 获取执行器函数
		runExe := GetRunExecutor(runJob.RunCommand, runJob.Limiter, runJob.WorkDir)

		// 执行单个测试用例
		testRes := runExe(runJob.Testcase) // 这里调用 executor.go 中的 GetRunExecutor 返回的函数

		runJob.RespChan <- testRes
	}()
}
