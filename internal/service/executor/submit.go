package executor

import "nightcord-server/internal/model"

// SubmitJob 提交评测任务到消息队列，并阻塞等待执行结果返回
func SubmitJob(req model.SubmitRequest) model.Result {
	job := &model.Job{
		Request:  req,
		RespChan: make(chan model.Result),
	}
	// 将任务加入消息队列中等待协程池执行
	jobQueue <- job
	// 阻塞等待任务的执行结果
	return <-job.RespChan
}
