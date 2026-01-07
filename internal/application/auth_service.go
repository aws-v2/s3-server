package application

import (
	"context"
	"fmt"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo         domain.RepositoryPort
	jwtGenerator TokenGenerator
}

type TokenGenerator interface {
	GenerateToken(userID, email string) (string, error)
}

func NewAuthService(repo domain.RepositoryPort, jwtGenerator TokenGenerator) *AuthService {
	return &AuthService{
		repo:         repo,
		jwtGenerator: jwtGenerator,
	}
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, input dto.RegisterInput) (*dto.AuthResponse, error) {
	// Validate input
	if input.Email == "" || input.Password == "" || input.Name == "" {
		return nil, fmt.Errorf("email, password, and name are required")
	}

	if len(input.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters long")
	}

	// Check if user already exists
	existingUser, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err == nil && existingUser != nil {
		return nil, fmt.Errorf("user with email %s already exists", input.Email)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user domain object
	now := time.Now()
	user := &domain.User{
		ID:           uuid.New().String(),
		Email:        input.Email,
		PasswordHash: string(hashedPassword),
		Name:         input.Name,
		CreatedAt:    now,
		UpdatedAt:    now,
		IsActive:     true,
	}

	// Save to repository
	savedUser, err := s.repo.SaveUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate JWT token
	token, err := s.jwtGenerator.GenerateToken(savedUser.ID, savedUser.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Return auth response
	return &dto.AuthResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresIn: 60 * 60 * 24, // 24 hours in seconds (adjust based on your JWT config)
		User: dto.UserInfo{
			ID:        savedUser.ID,
			Email:     savedUser.Email,
			Name:      savedUser.Name,
			CreatedAt: savedUser.CreatedAt,
		},
	}, nil
}

// Login authenticates a user and returns a token
func (s *AuthService) Login(ctx context.Context, input dto.LoginInput) (*dto.AuthResponse, error) {
	// Validate input
	if input.Email == "" || input.Password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("user account is disabled")
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Generate JWT token
	token, err := s.jwtGenerator.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Return auth response
	return &dto.AuthResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresIn: 60 * 60 * 24, // 24 hours in seconds (adjust based on your JWT config)
		User: dto.UserInfo{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			CreatedAt: user.CreatedAt,
		},
	}, nil
}

// GetUserByID retrieves a user by their ID
func (s *AuthService) GetUserByID(ctx context.Context, userID string) (*dto.UserInfo, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &dto.UserInfo{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt,
	}, nil
}
