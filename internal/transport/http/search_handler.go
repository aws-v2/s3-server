package http

import (
	"fmt"
	"log"
	"net/http"
	"s3/internal/application"
	"s3/internal/infrastructure/dto"
	"s3/internal/utils"

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
	requestID := c.GetString("requestId")
	var input dto.SearchFilesInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:SearchFiles] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	output, err := h.searchService.SearchFiles(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:SearchFiles] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("file search failed"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "search completed successfully", output)
}

// SearchByMetadata searches by metadata
// GET /search/metadata
func (h *SearchHandler) SearchByMetadata(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.SearchByMetadataInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:SearchByMetadata] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	output, err := h.searchService.SearchByMetadata(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:SearchByMetadata] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("metadata search failed"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "search completed successfully", output)
}

// SearchByTags searches by tags
// GET /search/tags
func (h *SearchHandler) SearchByTags(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.SearchByTagsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:SearchByTags] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	output, err := h.searchService.SearchByTags(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:SearchByTags] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("tag search failed"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "search completed successfully", output)
}

// SearchByContent searches by content
// GET /search/content
func (h *SearchHandler) SearchByContent(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.SearchByContentInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:SearchByContent] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	output, err := h.searchService.SearchByContent(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:SearchByContent] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("content search failed"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "search completed successfully", output)
}

// AdvancedSearch performs advanced search
// POST /search/advanced
func (h *SearchHandler) AdvancedSearch(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.AdvancedSearchInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:AdvancedSearch] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.searchService.AdvancedSearch(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:AdvancedSearch] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("advanced search failed"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "search completed successfully", output)
}

// GetSearchSuggestions gets search suggestions
// GET /search/suggestions
func (h *SearchHandler) GetSearchSuggestions(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.SearchSuggestionsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:GetSearchSuggestions] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	output, err := h.searchService.GetSearchSuggestions(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:GetSearchSuggestions] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get search suggestions"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "suggestions retrieved successfully", output)
}

// GetSearchHistory gets search history
// GET /search/history
func (h *SearchHandler) GetSearchHistory(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.SearchHistoryInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:GetSearchHistory] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	output, err := h.searchService.GetSearchHistory(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:GetSearchHistory] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve search history"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "search history retrieved successfully", output)
}

// SaveSearch saves a search query
// POST /search/save
func (h *SearchHandler) SaveSearch(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.SaveSearchInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:SaveSearch] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.searchService.SaveSearch(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:SaveSearch] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to save search"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "search saved successfully", output)
}
