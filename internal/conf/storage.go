package conf

// StorageConf 存储引擎配置
type StorageConf struct {
	StoreDir string `yaml:"store_dir" json:"store_dir"` // 存储目录
	DBPath   string `yaml:"db_path" json:"db_path"`     // 数据库文件路径
}

// Default 设置默认配置
func (s *StorageConf) Default() {
	s.StoreDir = "./storage/files"
	s.DBPath = "./storage/metadata.db"
}
