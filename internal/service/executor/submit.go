package executor

import (
	"nightcord-server/internal/model"
	"sync"
)

// SubmitJob 提交评测任务到消息队列，并阻塞等待执行结果返回
func SubmitJob(req model.SubmitRequest) model.JudgeResult {
	job := &model.Job{
		Request:  req,
		RespChan: make(chan model.JudgeResult),
	}
	// 将任务加入消息队列中等待协程池执行
	jobQueue <- job
	// 阻塞等待任务的执行结果
	return <-job.RespChan
}

func SubmitExeJob(runExe model.RunExe, req model.SubmitRequest) []model.TestResult {
	createJob := func(tc model.Testcase) *model.RunJob {
		return &model.RunJob{
			RunFunc:  runExe,
			RespChan: make(chan model.TestResult, 1), // 带缓冲通道防止阻塞
			Testcase: tc,
		}
	}

	switch req.TestcaseType {
	case model.SingleTest:
		job := createJob(model.Testcase{
			Stdin:          req.Stdin,
			ExpectedOutput: req.ExpectedOutput,
		})
		runQueue <- job
		return []model.TestResult{<-job.RespChan}
	case model.MultipleTest:
		res := make([]model.TestResult, len(req.Testcase))
		var wg sync.WaitGroup
		wg.Add(len(req.Testcase))

		for i := range req.Testcase {
			// 创建独立变量副本避免竞态
			idx := i
			tc := req.Testcase[idx]

			job := createJob(tc)
			runQueue <- job

			go func(j *model.RunJob) {
				defer func() {
					close(j.RespChan)
					if r := recover(); r != nil {
						res[idx] = model.TestResult{Message: "panic occurred"}
					}
					wg.Done()
				}()
				res[idx] = <-j.RespChan
			}(job)
		}
		wg.Wait()
		return res
	default:
		return nil
	}
}
