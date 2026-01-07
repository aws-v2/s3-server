package auth

import (
	"fmt"
	"s3/internal/domain"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secretKey     []byte
	issuer        string
	expiryMinutes int
}

func NewJWTService(secretKey string, issuer string, expiryMinutes int) *JWTService {
	return &JWTService{
		secretKey:     []byte(secretKey),
		issuer:        issuer,
		expiryMinutes: expiryMinutes,
	}
}

// GenerateToken creates a new JWT token for a user
func (j *JWTService) GenerateToken(userID, email string) (string, error) {
	claims := &domain.Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(j.expiryMinutes) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    j.issuer,
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// Validate validates a JWT token and returns the claims
func (j *JWTService) Validate(tokenString string) (*domain.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &domain.Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*domain.Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Additional validation
	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("token has expired")
	}

	return claims, nil
}

// RefreshToken generates a new token with extended expiry
func (j *JWTService) RefreshToken(oldToken string) (string, error) {
	claims, err := j.Validate(oldToken)
	if err != nil {
		return "", fmt.Errorf("cannot refresh invalid token: %w", err)
	}

	// Generate new token with the same user info
	return j.GenerateToken(claims.UserID, claims.Email)
}
