package executor

import (
	"nightcord-server/internal/model"
)

// SubmitJob 提交评测任务到消息队列，并阻塞等待执行结果返回
func SubmitJob(req model.SubmitRequest) model.JudgeResult {
	return GetJobManagerInstance().SubmitJob(req)
}
