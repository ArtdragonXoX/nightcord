package model

// Job 表示评测任务，由工作协程池调度执行
type Job struct {
	Request  SubmitRequest
	RespChan chan Result
}

// SubmitRequest 表示提交评测时的请求体
type SubmitRequest struct {
	SourceCode     string  `json:"source_code"`
	LanguageID     int     `json:"language_id"`
	Stdin          string  `json:"stdin"`
	ExpectedOutput string  `json:"expected_output"`
	CpuTimeLimit   float64 `json:"cpu_time_limit"`
	MemoryLimit    int     `json:"memory_limit"`
}

// Result 为评测结果返回格式
type Result struct {
	Stdout        string  `json:"stdout"`
	Time          string  `json:"time"`
	Memory        int     `json:"memory"`
	Stderr        *string `json:"stderr"`
	CompileOutput *string `json:"compile_output"`
	Message       *string `json:"message"`
	Status        struct {
		Id          int    `json:"id"`
		Description string `json:"description"`
	} `json:"status"`
}
