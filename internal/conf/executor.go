package conf

type ExecutorConf struct {
	Pool           int     `yaml:"pool" json:"pool"`
	ExtraCPUTime   float64 `yaml:"extra_cpu_time" json:"extra_cpu_time"`
	CompileTimeout float64 `yaml:"compile_timeout"`
	CompileMemory  int     `yaml:"compile_memory"`
	CPUTimeLimit   float64 `yaml:"cpu_time_limit"`
	MemoryLimit    uint    `yaml:"memory_limit"`
}

func (c *ExecutorConf) Default() {
	if c.Pool == 0 {
		c.Pool = 5
	}
	if c.ExtraCPUTime == 0 {
		c.ExtraCPUTime = 0.5
	}
	if c.CompileTimeout == 0 {
		c.CompileTimeout = 5
	}
	if c.CompileMemory == 0 {
		c.CompileMemory = 256
	}
	if c.CPUTimeLimit == 0 {
		c.CPUTimeLimit = 5
	}
	if c.MemoryLimit == 0 {
		c.MemoryLimit = 256
	}
}
