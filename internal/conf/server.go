package conf

type ServerConf struct {
	Port  string `yaml:"port" json:"port"`
	Token string `yaml:"token" json:"token"`
}

func (c *ServerConf) Default() {
	if c.Port == "" {
		c.Port = "25000"
	}
	if c.Token == "" {
		c.Token = "secret-token"
	}
}
