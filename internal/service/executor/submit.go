package executor

import (
	"nightcord-server/internal/model"
)

// SubmitJob 提交评测任务到消息队列，并阻塞等待执行结果返回
func SubmitJob(req model.SubmitRequest) model.JudgeResult {
	if req.TestcaseType == model.SingleTest {
		req.Testcase = []model.TestcaseReq{
			{
				Stdin:          req.Stdin,
				ExpectedOutput: req.ExpectedOutput,
			},
		}
	}
	return GetJobManagerInstance().SubmitJob(req)
}
