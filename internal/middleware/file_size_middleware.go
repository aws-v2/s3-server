package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func MaxFileSizeMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use MaxBytesReader to limit the size of the whole request body
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}
