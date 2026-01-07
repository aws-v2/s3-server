package dto

import "time"

// RegisterInput represents user registration request
type RegisterInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required,min=2"`
}

// LoginInput represents user login request
type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse represents successful authentication response
type AuthResponse struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"` // "Bearer"
	ExpiresIn int       `json:"expires_in"` // seconds
	User      UserInfo  `json:"user"`
}

// UserInfo represents user information in auth response
type UserInfo struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// RefreshTokenInput represents token refresh request
type RefreshTokenInput struct {
	Token string `json:"token" binding:"required"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
