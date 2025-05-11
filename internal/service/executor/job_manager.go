//go:build linux
// +build linux

package executor

import (
	"fmt"
	"nightcord-server/internal/model"
)

// Job 表示评测任务，由任务管理器调度执行
type Job struct {
	Request  model.SubmitRequest
	RespChan chan model.JudgeResult
}

type JobManager struct {
	JobQueue      chan *Job          // 任务队列
	JobQueueNum   int                // 任务队列大小
	JobNum        int                // 任务总数
	JobPoolNum    int                // 任务池大小
	JobRunners    map[int]*JobRunner // 任务运行器
	JobStatusChan chan struct{}      // 任务状态通道
}

func NewJobManager(jobNum, jobPoolNum, jobQueueNum int) *JobManager {
	var jm = &JobManager{
		JobQueue:    make(chan *Job, jobQueueNum),
		JobQueueNum: jobQueueNum,
		JobNum:      jobNum,
		JobPoolNum:  jobPoolNum,
		JobRunners:  make(map[int]*JobRunner),
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

// SubmitJob 提交一个新任务到任务队列。
// 如果任务队列已满，则会立即返回一个表示队列已满的 JudgeResult。
// 否则，任务会被添加到队列中，并阻塞等待任务执行完成后的结果。
func (jm *JobManager) SubmitJob(req model.SubmitRequest) model.JudgeResult {
	job := &Job{
		Request:  req,
		RespChan: make(chan model.JudgeResult),
	}

	select {
	case jm.JobQueue <- job:
		// 任务成功提交到队列，等待执行结果
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

func (jm *JobManager) GetJobRunnerStatus(id int) (JobRunnerStatus, error) {
	if jobRunner, ok := jm.JobRunners[id]; ok {
		return jobRunner.Status, nil
	}
	return JobRunnerStatusUnknown, fmt.Errorf("job runner %d not found", id)
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
