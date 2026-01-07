package middleware

import (
	"net/http"
	"s3/internal/domain"
	"strings"

	"github.com/gin-gonic/gin"
)

type APIKeyValidator interface {
	ValidateAPIKey(key string) (string, error)
}

type JWTValidator = domain.JWTValidator

// func APIKeyAuthMiddleware(validator APIKeyValidator) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		apiKey := c.GetHeader("x-api-key")
// 		if apiKey == "" {
// 			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing api key"})
// 			return
// 		}

// 		userID, err := validator.ValidateAPIKey(apiKey)
// 		if err != nil {
// 			fmt.Printf("AUTH_ERROR: %v\n", err)
// 			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
// 			return
// 		}

// 		// Store actor in context for policy system
// 		c.Set("actor", "user:"+userID)
// 		c.Next()
// 	}
// }





func APIKeyAuthMiddleware(validator APIKeyValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("x-api-key")
		if apiKey == "" {
			c.Next()
			return // allow other auth methods
		}

		userID, err := validator.ValidateAPIKey(apiKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}

		c.Set("actor", domain.Actor{
			ID:   userID,
			Type: domain.ActorService,
			Auth: "api_key",
		})

		c.Next()
	}
}




func BearerAuthMiddleware(jwtValidator JWTValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid auth header"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := jwtValidator.Validate(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set("actor", domain.Actor{
			ID:   claims.UserID,
			Type: domain.ActorUser,
			Auth: "bearer",
		})

		c.Next()
	}
}


func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get("actor"); !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		c.Next()
	}
}
