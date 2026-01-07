package http

import (
	"net/http"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"strings"

	"github.com/gin-gonic/gin"
)

type AuthService interface {
	Register(ctx gin.Context, input dto.RegisterInput) (*dto.AuthResponse, error)
	Login(ctx gin.Context, input dto.LoginInput) (*dto.AuthResponse, error)
	GetUserByID(ctx gin.Context, userID string) (*dto.UserInfo, error)
}

type AuthHandler struct {
	authService AuthService
}

func NewAuthHandler(authService AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Register godoc
// @Summary Register a new user
// @Description Create a new user account
// @Tags auth
// @Accept json
// @Produce json
// @Param input body dto.RegisterInput true "Registration details"
// @Success 201 {object} dto.AuthResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var input dto.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	response, err := h.authService.Register(*c, input)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
		} else if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "must be") {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, dto.ErrorResponse{
			Error:   "registration_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// Login godoc
// @Summary Login user
// @Description Authenticate user and return JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param input body dto.LoginInput true "Login credentials"
// @Success 200 {object} dto.AuthResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var input dto.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	response, err := h.authService.Login(*c, input)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid email or password") || strings.Contains(err.Error(), "disabled") {
			statusCode = http.StatusUnauthorized
		}

		c.JSON(statusCode, dto.ErrorResponse{
			Error:   "authentication_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetCurrentUser godoc
// @Summary Get current user
// @Description Get the currently authenticated user's information
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.UserInfo
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /auth/me [get]
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	// Get actor from context (set by auth middleware)
	actorValue, exists := c.Get("actor")
	if !exists {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
			Error:   "unauthorized",
			Message: "authentication required",
		})
		return
	}

	// Type assertion to get Actor
	actor, ok := actorValue.(domain.Actor)
	if !ok {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "invalid actor type",
		})
		return
	}

	// Get user info
	userInfo, err := h.authService.GetUserByID(*c, actor.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error:   "user_not_found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, userInfo)
}
