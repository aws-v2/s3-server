package http

import (
	"net/http"
	"s3/internal/application"
	"s3/internal/infrastructure/dto"

	"github.com/gin-gonic/gin"
)

type SearchHandler struct {
	searchService *application.SearchService
}

func NewSearchHandler(searchService *application.SearchService) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
	}
}

// SearchFiles searches files by name
// GET /search/files
func (h *SearchHandler) SearchFiles(c *gin.Context) {
	var input dto.SearchFilesInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	output, err := h.searchService.SearchFiles(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// SearchByMetadata searches by metadata
// GET /search/metadata
func (h *SearchHandler) SearchByMetadata(c *gin.Context) {
	var input dto.SearchByMetadataInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	output, err := h.searchService.SearchByMetadata(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// SearchByTags searches by tags
// GET /search/tags
func (h *SearchHandler) SearchByTags(c *gin.Context) {
	var input dto.SearchByTagsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	output, err := h.searchService.SearchByTags(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// SearchByContent searches by content
// GET /search/content
func (h *SearchHandler) SearchByContent(c *gin.Context) {
	var input dto.SearchByContentInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	output, err := h.searchService.SearchByContent(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// AdvancedSearch performs advanced search
// POST /search/advanced
func (h *SearchHandler) AdvancedSearch(c *gin.Context) {
	var input dto.AdvancedSearchInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.searchService.AdvancedSearch(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// GetSearchSuggestions gets search suggestions
// GET /search/suggestions
func (h *SearchHandler) GetSearchSuggestions(c *gin.Context) {
	var input dto.SearchSuggestionsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	output, err := h.searchService.GetSearchSuggestions(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// GetSearchHistory gets search history
// GET /search/history
func (h *SearchHandler) GetSearchHistory(c *gin.Context) {
	var input dto.SearchHistoryInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	output, err := h.searchService.GetSearchHistory(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// SaveSearch saves a search query
// POST /search/save
func (h *SearchHandler) SaveSearch(c *gin.Context) {
	var input dto.SaveSearchInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.searchService.SaveSearch(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, output)
}