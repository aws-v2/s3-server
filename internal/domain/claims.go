package domain

import "github.com/golang-jwt/jwt/v5"

type Claims struct {
	UserID string `json:"sub"` // subject = user id
	Email  string `json:"email,omitempty"`
	jwt.RegisteredClaims
}
