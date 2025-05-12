package handler

import (
	"net/http"
	"nightcord-server/internal/model"
	"nightcord-server/internal/service/executor"

	"github.com/gin-gonic/gin"
)

// Executor 处理 POST /run 的请求
func Executor(c *gin.Context) {
	var req model.SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	// 将任务加入消息队列中等待协程池执行
	result := executor.SubmitJob(req)
	c.JSON(http.StatusOK, result)
}

func GetJobStatus(c *gin.Context) {
	status := executor.GetJobManagerInstance().GetStatus()
	c.JSON(http.StatusOK, status)
}
