package handler

import (
	"net/http"
	"nightcord-server/internal/service/language"

	"github.com/gin-gonic/gin"
)

// GetLanguages 返回语言列表（仅包含id和name），用于客户端展示
func GetLanguages(c *gin.Context) {
	var langs []gin.H
	languages := language.GetLanguages()
	for _, lang := range languages {
		langs = append(langs, gin.H{
			"id":   lang.ID,
			"name": lang.Name,
		})
	}
	c.JSON(http.StatusOK, langs)
}
