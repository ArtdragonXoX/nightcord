package conf

type ExecutorConf struct {
	JobQueue       int     `yaml:"job_queue" json:"job_queue"`           // 任务队列大小
	JobPool        int     `yaml:"job_pool" json:"job_pool"`             // 任务协程池池数量
	RunQueue       int     `yaml:"run_queue" json:"run_queue"`           // 运行任务队列大小
	RunPool        int     `yaml:"run_pool" json:"run_pool"`             // 运行协程池数量
	ExtraCPUTime   float64 `yaml:"extra_cpu_time" json:"extra_cpu_time"` // seconds 在超出限制时间后的额外时间
	CompileTimeout float64 `yaml:"compile_timeout"`                      // seconds 最大编译时间
	CompileMemory  int     `yaml:"compile_memory"`                       // KB 最大编译内存
	CPUTimeLimit   float64 `yaml:"cpu_time_limit"`                       // seconds 默认运行时间
	MemoryLimit    uint    `yaml:"memory_limit"`                         // KB 默认运行内存
}

func (c *ExecutorConf) Default() {
	if c.JobQueue == 0 {
		c.JobQueue = 500
	}
	if c.JobPool == 0 {
		c.JobPool = 5
	}
	if c.RunQueue == 0 {
		c.RunQueue = 500
	}
	if c.RunPool == 0 {
		c.RunPool = 5
	}
	if c.ExtraCPUTime == 0 {
		c.ExtraCPUTime = 0.5
	}
	if c.CompileTimeout == 0 {
		c.CompileTimeout = 5
	}
	if c.CompileMemory == 0 {
		c.CompileMemory = 262144
	}
	if c.CPUTimeLimit == 0 {
		c.CPUTimeLimit = 5
	}
	if c.MemoryLimit == 0 {
		c.MemoryLimit = 262144
	}
}
