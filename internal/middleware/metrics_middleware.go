package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

type test struct {
	Type string
}

// handle batch uploadsand donwloads
func EmitMetricsMiddleware(route string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("billRequest", true)

		switch route {
		case "files":
			fmt.Printf("Emitted metrics for /files routes")
		case "batch":
			fmt.Printf("Emitted metrics for /batch routes")

		case "prefix":
			fmt.Printf("Emitted metrics for /prefix routes")

		case "multipart":
			fmt.Printf("Emitted metrics for /multipart routes")

		default:
			fmt.Printf("Emitted metrics for default routes")

		}

	}
}
