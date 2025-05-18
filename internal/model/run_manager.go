package model

type RunWorkerStatus struct {
	Id       int     `json:"id"`
	Status   string  `json:"status"`
	TimeUsed float64 `json:"timeUsed"`
}

type RunManagerStatus struct {
	RunQueueNum  int               `json:"runQueueNum"`
	RunPoolNum   int               `json:"runPoolNum"`
	RunnerStatus []RunWorkerStatus `json:"runnerStatus"`
}
