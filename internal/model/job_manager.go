package model

type JobManagerStatus struct {
	JobQueueNum  int               `json:"job_queue_num"`
	JobPoolNum   int               `json:"job_pool_num"`
	JobNum       int32             `json:"job_num"`
	RunnerStatus []JobRunnerStatus `json:"runner_status"`
}

type JobRunnerStatus struct {
	Id       int     `json:"id"`
	Status   string  `json:"status"`
	TimeUsed float64 `json:"time_used"`
}
