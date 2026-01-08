package middleware

import (
	"github.com/gin-gonic/gin"
)

func AllowedFileTypesMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// All file types allowed as requested
		c.Next()
	}
}
