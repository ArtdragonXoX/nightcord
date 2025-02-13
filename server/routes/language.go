package routes

import (
	"nightcord-server/server/handler"

	"github.com/gin-gonic/gin"
)

func InitLanguageRoutes(router *gin.Engine) {
	router.GET("/languages", handler.GetLanguages)
}
