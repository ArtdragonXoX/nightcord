package executor

import (
	"fmt"
	"nightcord-server/internal/model"
	"sync"
)

// Job 表示评测任务，由任务管理器调度执行
type Job struct {
	Request  model.SubmitRequest
	RespChan chan model.JudgeResult
}

type JobManager struct {
	JobQueue       chan *Job          // 任务队列
	JobNum         int                // 任务总数
	JobPoolNum     int                // 任务池大小
	JobRunners     map[int]*JobRunner // 任务运行器
	JobRunnersLock sync.Mutex         // 任务运行器锁
	JobStatusChan  chan struct{}      // 任务状态通道
}

func NewJobManager(jobNum, jobPoolNum int) *JobManager {
	var jm = &JobManager{
		JobQueue:   make(chan *Job, jobNum),
		JobNum:     jobNum,
		JobPoolNum: jobPoolNum,
		JobRunners: make(map[int]*JobRunner),
	}
	for i := 0; i < jobPoolNum; i++ {
		jobRunner := NewJobRunner(i, jm.JobQueue)
		jm.JobRunners[i] = jobRunner
	}
	return jm
}

func (jm *JobManager) Start() {
	for _, jobRunner := range jm.JobRunners {
		jobRunner.Start()
	}
}

func (jm *JobManager) SubmitJob(req model.SubmitRequest) model.JudgeResult {
	job := &Job{
		Request:  req,
		RespChan: make(chan model.JudgeResult),
	}
	jm.JobQueue <- job
	return <-job.RespChan
}

type JobRunnerStatus uint8

const (
	JobRunnerStatusIdle JobRunnerStatus = iota
	JobRunnerStatusRunning
	JobRunnerStatusStopped
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
	Id          int
	Job         *Job
	JobQueue    <-chan *Job
	Status      JobRunnerStatus
	controlChan chan JobControlCommand // 控制通道，用于接收控制命令
	jobFinish   chan struct{}          // 任务完成通道，当任务完成时，向该通道发送信号
}

func NewJobRunner(id int, jobQueue <-chan *Job) *JobRunner {
	return &JobRunner{
		Id:          id,
		JobQueue:    jobQueue,
		Status:      JobRunnerStatusStopped,
		controlChan: make(chan JobControlCommand),
		jobFinish:   make(chan struct{}),
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
			jr.Job = nil
		}
	}
}

func (jr *JobRunner) handleJob(job *Job) {
	jr.Status = JobRunnerStatusRunning
	jr.Job = job
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 记录panic错误
				job.RespChan <- model.JudgeResult{
					Status:  model.StatusIE.GetStatus(),
					Message: fmt.Sprintf("Internal error: %v", r),
				}
			}
			jr.jobFinish <- struct{}{}
		}()
		res := ProcessJob(job.Request)
		job.RespChan <- res
	}()
}

func (jr *JobRunner) handleControl(cmd JobControlCommand) {
	switch cmd {
	case JobControlCommandStop:
		jr.Status = JobRunnerStatusStopped
	case JobControlCommandRelease:
		jr.Status = JobRunnerStatusIdle
	}
	jr.Job = nil
}
