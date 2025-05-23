package conf

import (
	"nightcord-server/utils"
)

type Config struct {
	Server   ServerConf   `yaml:"server" json:"server"`
	Executor ExecutorConf `yaml:"executor" json:"executor"`
	Storage  StorageConf  `yaml:"storage" json:"storage"`
}

func (c *Config) Default() {
	c.Server.Default()
	c.Executor.Default()
	c.Storage.Default()
}

func (c *Config) ReadYaml() error {
	v, err := utils.IsFileExists("config.yaml")
	if err != nil {
		return err
	}
	if !v {
		c.Default()
	}
	err = utils.ReadYaml(c, "config.yaml")
	if err != nil {
		return err
	}
	return nil
}

func (c *Config) WriteYaml() error {
	return utils.WriteYaml(c, "config.yaml")
}

func InitConfig() error {
	return Conf.ReadYaml()
}
