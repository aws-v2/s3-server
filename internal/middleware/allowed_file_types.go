package middleware

import (
 
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func AllowedFileTypesMiddleware() gin.HandlerFunc {
	var allowedTypes = map[string]bool{
		"image/png":                true,
		"image/jpeg":               true,
		"application/pdf":          true,
		"application/octet-stream": true,
	}
	return func(c *gin.Context) {
		file, _, err := c.Request.FormFile("file")
		if err != nil {
 
			c.AbortWithStatusJSON(400, gin.H{"error": "No file uploaded"})
			return
		}
		defer file.Close()

		buffer := make([]byte, 512)
		_, err = file.Read(buffer)
		if err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": "Cannot read file"})
			return
		}
		file.Seek(0, io.SeekStart)
		contentType := http.DetectContentType(buffer)

		if !allowedTypes[contentType] {
			c.AbortWithStatusJSON(400, gin.H{"error": "Invalid file type"})
			return
		}

		c.Next()
	}
}
