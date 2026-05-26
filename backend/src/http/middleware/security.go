package middleware

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
)

func HTTPSRedirect() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.GetEnv("APP_ENV") == "production" &&
			c.GetHeader("X-Forwarded-Proto") == "http" {
			target := "https://" + c.Request.Host + c.Request.RequestURI
			c.Redirect(http.StatusMovedPermanently, target)
			c.Abort()
			return
		}
		c.Next()
	}
}

func MaxBodySize(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		if err := c.Request.ParseForm(); err != nil {
			if err.Error() == "http: request body too large" {
				c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
				return
			}
		}
		// reset body so gin can re-read it
		c.Request.Body = io.NopCloser(c.Request.Body)
		c.Next()
	}
}
