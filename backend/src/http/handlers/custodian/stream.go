package custodian

import "github.com/gin-gonic/gin"

func StreamHandler(c *gin.Context) {
	if streamService == nil {
		c.AbortWithStatus(503)
		return
	}
	streamService.Stream(c)
}
