package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func MaxFileSizeMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)

		file, _, err := c.Request.FormFile("file")
		if err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": "File too large or missing"})
			return
		}
		file.Close() // Close immediately, actual handler will reopen
		c.Next()
	}
}
