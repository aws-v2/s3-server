package middleware

import "github.com/gin-gonic/gin"

// Helper middleware for query parameter routing
func QueryParam(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.GetQuery(param); !exists {
			c.Next() // Skip if query param not present
		}
	}
}