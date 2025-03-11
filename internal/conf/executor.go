package conf

type ExecutorConf struct {
	JobPool        int     `yaml:"job_pool" json:"job_pool"`
	RunPool        int     `yaml:"run_pool" json:"run_pool"`
	ExtraCPUTime   float64 `yaml:"extra_cpu_time" json:"extra_cpu_time"`
	CompileTimeout float64 `yaml:"compile_timeout"` // seconds
	CompileMemory  int     `yaml:"compile_memory"`  // KB
	CPUTimeLimit   float64 `yaml:"cpu_time_limit"`  // seconds
	MemoryLimit    uint    `yaml:"memory_limit"`    // KB
}

func (c *ExecutorConf) Default() {
	if c.JobPool == 0 {
		c.JobPool = 5
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
