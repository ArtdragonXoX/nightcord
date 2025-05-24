//go:build linux
// +build linux

package executor

import (
	"context"
	"fmt"
	"io"
	"nightcord-server/internal/conf"
	"nightcord-server/internal/model"
	"nightcord-server/internal/service/storage"
	"nightcord-server/utils"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type JobStatus uint8

const (
	JobStatusIdle JobStatus = iota
	JobStatusRunning
	JobStatusRelease
	JobStatusFinished
)

// Job 表示评测任务，由任务管理器调度执行
type Job struct {
	Request    model.SubmitRequest
	RespChan   chan model.JudgeResult
	ctx        context.Context
	cancelFunc context.CancelFunc
}

func NewJob(req model.SubmitRequest) *Job {
	ctx, cancel := context.WithCancel(context.Background())
	return &Job{
		Request:    req,
		RespChan:   make(chan model.JudgeResult),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

type JobManager struct {
	JobQueue      chan *Job          // 任务队列
	JobQueueNum   int                // 任务队列大小
	JobNum        int32              // 任务数量
	JobPoolNum    int                // 任务池大小
	JobRunners    map[int]*JobRunner // 任务运行器
	JobStatusChan chan JobStatus     // 任务状态通道
}

var (
	globalJobManager *JobManager
	onceJobManager   sync.Once
)

func GetJobManagerInstance() *JobManager {
	onceJobManager.Do(func() {
		globalJobManager = NewJobManager(
			conf.Conf.Executor.JobPool, // 使用配置中的 JobPool 数量
			conf.Conf.Executor.JobQueue,
		)
		globalJobManager.Start()
	})
	return globalJobManager
}

func NewJobManager(jobPoolNum, jobQueueNum int) *JobManager {
	var jm = &JobManager{
		JobQueue:      make(chan *Job, jobQueueNum),
		JobQueueNum:   jobQueueNum,
		JobPoolNum:    jobPoolNum,
		JobRunners:    make(map[int]*JobRunner),
		JobStatusChan: make(chan JobStatus),
	}
	for i := 0; i < jobPoolNum; i++ {
		jobRunner := NewJobRunner(i, jm.JobQueue, jm.JobStatusChan)
		jm.JobRunners[i] = jobRunner
	}
	return jm
}

func (jm *JobManager) Start() {
	for _, jobRunner := range jm.JobRunners {
		jobRunner.Start()
	}
	go jm.JobNumManager()
}

func (jm *JobManager) JobNumManager() {
	for {
		select {
		case status := <-jm.JobStatusChan:
			switch status {
			case JobStatusIdle:
				atomic.AddInt32(&jm.JobNum, 1)
			case JobStatusRelease:
				atomic.AddInt32(&jm.JobNum, 1)
			case JobStatusFinished:
				atomic.AddInt32(&jm.JobNum, -1)
			}
		}
	}
}

func (jm *JobManager) GetJobNum() int32 {
	return atomic.LoadInt32(&jm.JobNum)
}

// SubmitJob 提交一个新任务到任务队列。
// 如果任务队列已满，则会立即返回一个表示队列已满的 JudgeResult。
// 否则，任务会被添加到队列中，并阻塞等待任务执行完成后的结果。
func (jm *JobManager) SubmitJob(req model.SubmitRequest) model.JudgeResult {
	job := NewJob(req)

	select {
	case jm.JobQueue <- job:
		// 任务成功提交到队列，等待执行结果
		jm.JobStatusChan <- JobStatusIdle
		return <-job.RespChan
	default:
		// 任务队列已满，返回拒绝信息
		return model.JudgeResult{
			Status:  model.StatusIE.GetStatus(),
			Message: "Job queue is full, please try again later.",
		}
	}
}

func (jm *JobManager) StopJobRunner(id int) {
	if jobRunner, ok := jm.JobRunners[id]; ok {
		jobRunner.Stop()
	}
}

func (jm *JobManager) ReleaseJobRunner(id int) {
	if jobRunner, ok := jm.JobRunners[id]; ok {
		jobRunner.Release()
	}
}

func (jm *JobManager) Stop() {
	for _, jobRunner := range jm.JobRunners {
		jobRunner.Stop()
	}
}

func (jm *JobManager) Release() {
	for _, jobRunner := range jm.JobRunners {
		jobRunner.Release()
	}
}

func (jm *JobManager) GetJobRunnerStatusAll() map[int]JobRunnerStatus {
	statusMap := make(map[int]JobRunnerStatus)
	for id, jobRunner := range jm.JobRunners {
		statusMap[id] = jobRunner.Status
	}
	return statusMap
}

func (jm *JobManager) GetJobRunnerJob(id int) (*Job, error) {
	if jobRunner, ok := jm.JobRunners[id]; ok {
		return jobRunner.Job, nil
	}
	return nil, fmt.Errorf("job runner %d not found", id)
}

func (jm *JobManager) GetJobRunnerJobAll() map[int]*Job {
	jobMap := make(map[int]*Job)
	for id, jobRunner := range jm.JobRunners {
		jobMap[id] = jobRunner.Job
	}
	return jobMap
}

func (jm *JobManager) GetJobRunnerStatus() []model.JobRunnerStatus {
	var status []model.JobRunnerStatus
	for i := range jm.JobPoolNum {
		runner, ok := jm.JobRunners[i]
		if ok {
			status = append(status, runner.GetStatus())
		}
	}
	return status
}

func (jm *JobManager) GetStatus() model.JobManagerStatus {
	return model.JobManagerStatus{
		JobQueueNum:  jm.JobQueueNum,
		JobPoolNum:   jm.JobPoolNum,
		JobNum:       jm.GetJobNum(),
		RunnerStatus: jm.GetJobRunnerStatus(),
	}
}

type JobRunnerStatus uint8

const (
	JobRunnerStatusIdle JobRunnerStatus = iota
	JobRunnerStatusRunning
	JobRunnerStatusStopped
	JobRunnerStatusUnknown
)

func (s JobRunnerStatus) String() string {
	switch s {
	case JobRunnerStatusIdle:
		return "Idle"
	case JobRunnerStatusRunning:
		return "Running"
	case JobRunnerStatusStopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}

type JobControlCommand uint8

const (
	JobControlCommandStop JobControlCommand = iota
	JobControlCommandRelease
)

// JobRunner 表示任务运行器，负责执行具体的任务
type JobRunner struct {
	Id            int
	Job           *Job
	JobQueue      <-chan *Job
	Status        JobRunnerStatus
	controlChan   chan JobControlCommand // 控制通道，用于接收控制命令
	jobFinish     chan struct{}          // 任务完成通道，当任务完成时，向该通道发送信号
	jobStartTime  time.Time              // 任务开始时间
	jobStatusChan chan<- JobStatus       // 任务状态通道，用于向任务管理器报告任务状态
}

func NewJobRunner(id int, jobQueue <-chan *Job, jobStatusChan chan<- JobStatus) *JobRunner {
	return &JobRunner{
		Id:            id,
		JobQueue:      jobQueue,
		Status:        JobRunnerStatusStopped,
		controlChan:   make(chan JobControlCommand),
		jobFinish:     make(chan struct{}),
		jobStatusChan: jobStatusChan,
	}
}

func (jr *JobRunner) Start() {
	jr.Status = JobRunnerStatusIdle
	go jr.Run()
}

func (jr *JobRunner) Stop() {
	jr.controlChan <- JobControlCommandStop
}

func (jr *JobRunner) Release() {
	jr.controlChan <- JobControlCommandRelease
}

func (jr *JobRunner) GetTimeUsed() time.Duration {
	if jr.Status == JobRunnerStatusRunning {
		return time.Since(jr.jobStartTime)
	}
	return 0
}

func (jr *JobRunner) GetStatus() model.JobRunnerStatus {
	return model.JobRunnerStatus{
		Id:       jr.Id,
		Status:   jr.Status.String(),
		TimeUsed: jr.GetTimeUsed().Seconds(),
	}
}

func (jr *JobRunner) Run() {
	for jr.Status != JobRunnerStatusStopped {
		// 根据当前状态动态设置可用的通道
		var jobChan <-chan *Job
		if jr.Status == JobRunnerStatusIdle {
			jobChan = jr.JobQueue
		}

		select {
		case job := <-jobChan: // 仅在Idle状态监听任务队列
			jr.handleJob(job)
		case cmd := <-jr.controlChan:
			jr.handleControl(cmd)
			if jr.Status == JobRunnerStatusStopped {
				return
			}
		case <-jr.jobFinish:
			jr.Status = JobRunnerStatusIdle
			jr.jobStatusChan <- JobStatusFinished
			jr.Job = nil
		}
	}
}

func (jr *JobRunner) handleJob(job *Job) {
	jr.Status = JobRunnerStatusRunning
	jr.jobStartTime = time.Now()
	jr.Job = job
	go func() {
		var result model.JudgeResult
		var workDir string // 用于确保defer中可以访问到workDir

		defer func() {
			// 检查上下文是否已取消
			select {
			case <-job.ctx.Done():
				result.Status = model.StatusIE.GetStatus()
				result.Message = "Job was canceled before execution"
				return
			default:
			}
			job.cancelFunc()
			defer close(job.RespChan) // 确保关闭 RespChan
			if workDir != "" {
				os.RemoveAll(workDir) // 清理临时工作目录
			}
			if r := recover(); r != nil {
				// 记录panic错误
				job.RespChan <- model.JudgeResult{
					Status:  model.StatusIE.GetStatus(),
					Message: fmt.Sprintf("JobRunner panic: %v", r),
				}
			} else {
				job.RespChan <- result // 发送最终结果
			}
			jr.jobFinish <- struct{}{}
		}()

		// 1. 调用 PrepareEnvironmentAndCompile
		lang, wd, compileRes, err := PrepareEnvironmentAndCompile(job.ctx, job.Request)
		workDir = wd // 赋值给外层变量以便defer可以清理

		if err != nil {
			result.Status = model.StatusIE.GetStatus()
			result.Message = fmt.Sprintf("Environment preparation failed: %v", err.Error())
			// 不需要手动关闭 job.RespChan，因为 defer 中会发送结果
			return // 提前返回，defer 会执行清理和发送结果
		}

		result.Compilation = compileRes
		if !compileRes.Success {
			if compileRes.Message != "" {
				result.Message = compileRes.Message
				result.Status = model.StatusIE.GetStatus() // 编译错误信息通常表示内部或配置问题
			} else {
				result.Status = model.StatusCE.GetStatus() // 标准编译错误
			}
			return // 编译失败，提前返回
		}

		// 2. 迭代测试用例并提交给 RunManager
		runManager := GetRunManagerInstance()
		numTestCases := len(job.Request.Testcase)
		result.TestResult = make([]model.TestResult, numTestCases)
		var overallStatus model.Status // 用于跟踪整体评测状态
		var maxTime float64
		var maxMemory uint

		if numTestCases == 0 {
			result.Status = model.StatusIE.GetStatus()
			result.Message = "No testcases provided."
			return // 没有测试用例，提前返回
		}

		// 初始化 overallStatus 为 Accepted，如果后续有任何非AC状态，则更新
		overallStatus = model.StatusAC.GetStatus()

		var wg sync.WaitGroup
		var mu sync.Mutex // 用于保护共享资源的互斥锁
		wg.Add(numTestCases)

		var limiter = model.Limiter{
			CpuTime: job.Request.CpuTimeLimit,
			Memory:  job.Request.MemoryLimit,
		}

		for i, tc := range job.Request.Testcase {
			// 为每个测试用例启动一个 goroutine
			go func(index int, currentTestcase model.TestcaseReq) {
				defer wg.Done() // goroutine 完成后减少等待组计数器

				var testcase model.Testcase

				if job.Request.TestcaseType <= model.MultipleTest {
					testcase = model.Testcase{
						Stdin:          strings.NewReader(currentTestcase.Stdin),
						ExpectedOutput: strings.NewReader(currentTestcase.ExpectedOutput),
					}
				} else if job.Request.TestcaseType == model.FileTest {
					// 从storage读取测试数据文件
					storageEngine := storage.GetStorageEngineInstance()

					// 读取输入文件
					var stdinReader io.Reader
					if currentTestcase.Stdin != "" {
						inputFile, err := storageEngine.ReadFile(currentTestcase.Stdin)
						if err != nil {
							mu.Lock()
							result.TestResult[index] = model.TestResult{
								Status:  model.StatusIE.GetStatus(),
								Message: fmt.Sprintf("读取输入文件失败: %v", err),
							}
							mu.Unlock()
							return
						}
						stdinReader = inputFile
						defer inputFile.Close()
					}

					// 读取期望输出文件
					var expectedOutputReader io.Reader
					if currentTestcase.ExpectedOutput != "" {
						outputFile, err := storageEngine.ReadFile(currentTestcase.ExpectedOutput)
						if err != nil {
							mu.Lock()
							result.TestResult[index] = model.TestResult{
								Status:  model.StatusIE.GetStatus(),
								Message: fmt.Sprintf("读取期望输出文件失败: %v", err),
							}
							mu.Unlock()
							return
						}
						expectedOutputReader = outputFile
						defer outputFile.Close()
					}

					testcase = model.Testcase{
						Stdin:          stdinReader,
						ExpectedOutput: expectedOutputReader,
					}
				}

				runExe := GetRunExecutor(lang.RunCmd, limiter, workDir, true, testcase.Stdin)

				runJob := NewRunJob(runExe, job.ctx)
				// runManager 变量从外部作用域捕获
				testCaseResult := runManager.SubmitRunJob(runJob)

				if testCaseResult.Status.Id == model.StatusAC && testcase.ExpectedOutput != nil {
					var expectedOutput string
					expectedBytes, err := io.ReadAll(testcase.ExpectedOutput)
					if err != nil {
						testCaseResult.Status = model.StatusIE.GetStatus()
						testCaseResult.Message = fmt.Sprintf("读取预期输出失败: %v", err)
						return
					}
					expectedOutput = string(expectedBytes)

					// 验证输出结果是否符合预期
					if expectedOutput != "" {
						if !utils.StringsEqualIgnoreFinalNewline(testCaseResult.Stdout, expectedOutput) {
							testCaseResult.Status = model.StatusWA.GetStatus()
						}
					}
				}

				mu.Lock() // 获取互斥锁以安全地更新共享资源
				result.TestResult[index] = testCaseResult

				// 更新最大时间和内存
				if testCaseResult.Time > maxTime {
					maxTime = testCaseResult.Time
				}
				if testCaseResult.Memory > maxMemory {
					maxMemory = testCaseResult.Memory
				}

				// 更新整体状态，取所有测试用例中优先级最高（ID值最大）的状态
				// 如果当前测试用例的状态优先级高于 overallStatus，则更新 overallStatus
				if testCaseResult.Status.Id > overallStatus.Id {
					overallStatus = testCaseResult.Status
				}
				mu.Unlock() // 释放互斥锁
			}(i, tc) // 将循环变量作为参数传递给 goroutine
		}
		wg.Wait() // 等待所有测试用例的 goroutine 完成

		result.Status = overallStatus
		result.MaxTime = maxTime
		result.MaxMemory = maxMemory

		// 如果所有测试用例都通过 (overallStatus 仍然是 AC)，且有测试用例
		// 再次确认所有测试用例都是AC，因为 overallStatus 可能因为某个严重错误（如IE）而变大，但并非所有测试用例都失败
		if overallStatus.Id == model.StatusAC.GetStatus().Id && numTestCases > 0 {
			allAC := true
			for _, tr := range result.TestResult {
				if tr.Status.Id != model.StatusAC.GetStatus().Id {
					allAC = false
					// 如果发现非AC，则最终状态应该是第一个非AC的状态或者优先级最高的那个
					// 上面的循环已经保证了overallStatus是优先级最高的，所以这里不需要再次修改overallStatus
					break
				}
			}
			if allAC {
				result.Status = model.StatusAC.GetStatus()
			} // else: overallStatus 已经是正确的非AC状态了
		} else if overallStatus.Id != model.StatusAC.GetStatus().Id {
			// 如果 overallStatus 不是 AC，它已经是所有测试用例中优先级最高的错误状态了
			// 不需要额外处理，result.Status 已经被正确设置
		} else if numTestCases > 0 && overallStatus.Id == 0 { // 理论上不会到这里，因为 overallStatus 初始化为 AC
			result.Status = model.StatusIE.GetStatus()
			result.Message = "Unknown error during testcase aggregation."
		}

		// workDir 的清理已在 defer 中处理
		// 结果的发送也已在 defer 中处理
	}()
}

func (jr *JobRunner) handleControl(cmd JobControlCommand) {
	// 取消任务上下文
	jr.Job.cancelFunc()
	jr.Job.RespChan <- model.JudgeResult{
		Status:  model.StatusIE.GetStatus(), // 内部错误状态
		Message: "JobRunner is stopping or releasing.",
	}
	close(jr.Job.RespChan)
	jr.Job = nil
	switch cmd {
	case JobControlCommandStop:
		jr.Status = JobRunnerStatusStopped
	case JobControlCommandRelease:
		jr.Status = JobRunnerStatusIdle
	}
	jr.jobStatusChan <- JobStatusRelease
}
