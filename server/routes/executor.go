package routes

import (
	"nightcord-server/server/handler"

	"github.com/gin-gonic/gin"
)

func InitExecutorRoutes(router *gin.Engine) {
	router.POST("/executor", handler.Executor)
	router.GET("/job/status", handler.GetJobStatus)
	router.GET("/run/status", handler.GetRunManagerStatus) // 添加新的运行状态查询路由
}
