//go:build linux
// +build linux

package bootstrap

import (
	"log"
	"nightcord-server/internal/conf"
	"nightcord-server/internal/service/storage"
)

func initStorage() {
	// 使用配置文件中的存储配置初始化存储引擎
	storageConfig := &storage.Config{
		StoreDir: conf.Conf.Storage.StoreDir,
		DBPath:   conf.Conf.Storage.DBPath,
	}

	err := storage.InitStorageEngine(storageConfig)
	if err != nil {
		log.Fatalf("Failed to initialize storage engine: %v", err)
	}

	log.Println("Storage engine initialized successfully")
}
