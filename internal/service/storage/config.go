//go:build linux
// +build linux

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Config 存储引擎配置
type Config struct {
	StoreDir string `yaml:"store_dir"` // 存储目录
	DBPath   string `yaml:"db_path"`   // 数据库文件路径
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		StoreDir: "./storage/files",
		DBPath:   "./storage/metadata.db",
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.StoreDir == "" {
		return fmt.Errorf("store_dir cannot be empty")
	}
	if c.DBPath == "" {
		return fmt.Errorf("db_path cannot be empty")
	}
	return nil
}

var (
	globalStorageEngine *StorageEngine
	onceStorageEngine   sync.Once
)

// GetStorageEngineInstance 获取存储引擎单例实例
func GetStorageEngineInstance() *StorageEngine {
	onceStorageEngine.Do(func() {
		config := DefaultConfig()
		var err error
		globalStorageEngine, err = NewStorageEngine(config.StoreDir, config.DBPath)
		if err != nil {
			panic(fmt.Sprintf("Failed to initialize storage engine: %v", err))
		}
	})
	return globalStorageEngine
}

// InitStorageEngine 使用自定义配置初始化存储引擎
func InitStorageEngine(config *Config) error {
	if err := config.Validate(); err != nil {
		return err
	}

	// 确保数据库目录存在
	dbDir := filepath.Dir(config.DBPath)
	if dbDir != "." && dbDir != "" {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return fmt.Errorf("failed to create database directory: %v", err)
		}
	}

	var err error
	globalStorageEngine, err = NewStorageEngine(config.StoreDir, config.DBPath)
	return err
}

// CloseStorageEngine 关闭存储引擎
func CloseStorageEngine() error {
	if globalStorageEngine != nil {
		return globalStorageEngine.Close()
	}
	return nil
}
