package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetResponseCacheStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    service.GetResponseCacheStats(),
	})
}

func ClearResponseCache(c *gin.Context) {
	if err := service.ClearResponseCache(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	service.ResetResponseCacheStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
