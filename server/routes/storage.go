//go:build linux
// +build linux

package routes

import (
	"nightcord-server/server/handler"

	"github.com/gin-gonic/gin"
)

// InitStorageRoutes 设置存储相关路由
func InitStorageRoutes(router *gin.Engine) {
	storageHandler := handler.NewStorageHandler()

	// 存储API路由组
	storageGroup := router.Group("/storage")
	{
		// 文件上传（新建或修改）
		storageGroup.POST("/files", storageHandler.UploadFile)

		// 获取测试用例内容（JSON格式）
		storageGroup.GET("/files/:filename", storageHandler.GetTestcaseContent)

		// 下载文件（原始格式）
		storageGroup.GET("/files/:filename/download", storageHandler.DownloadFile)

		// 获取文件元数据
		storageGroup.GET("/files/:filename/metadata", storageHandler.GetFileMetadata)

		// 更新文件内容
		storageGroup.PUT("/files/:filename", storageHandler.UpdateFile)

		// 删除文件
		storageGroup.DELETE("/files/:filename", storageHandler.DeleteFile)

		// 列出所有文件
		storageGroup.GET("/files", storageHandler.ListFiles)
	}
}
