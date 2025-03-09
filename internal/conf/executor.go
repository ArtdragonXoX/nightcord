package conf

type ExecutorConf struct {
	Pool           int     `yaml:"pool" json:"pool"`
	ExtraCPUTime   float64 `yaml:"extra_cpu_time" json:"extra_cpu_time"`
	CompileTimeout float64 `yaml:"compile_timeout"`
	CompileMemory  int     `yaml:"compile_memory"`
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
}
