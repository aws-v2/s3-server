package middleware

import (
	"s3/internal/domain"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ExtractUserIDMiddleware extracts the User ID from the JWT in the Authorization header.
// It assumes the JWT is a valid token (as per gateway handling) and decodes it to get the 'sub' claim.
func ExtractUserIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.Next()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Decode the token without validation (since gateway already validated it)
		// If explicit validation is needed, a JWTValidator should be used.
		parser := jwt.NewParser()
		token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
		if err != nil {
			c.Next()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			userID, _ := claims["sub"].(string)
			if userID != "" {
				c.Set("actor", domain.Actor{
					ID:   userID,
					Type: domain.ActorUser,
					Auth: "bearer",
				})
			}
		}

		c.Next()
	}
}
