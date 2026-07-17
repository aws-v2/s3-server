package http

import (
	"s3/internal/application"

	"github.com/gin-gonic/gin"
)


type DocsHandler struct {
	service *application.DocsService
}
func NewDocsHandler(service *application.DocsService) *DocsHandler {
	return &DocsHandler{service: service}
}
func (h *DocsHandler) GetManifest(c *gin.Context) {
	role := c.GetString("role")

	if role == "USER" {
		data, err := h.service.GetManifest(false)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": data})
		return
	}

	// For administrative/internal roles, merge public and internal manifests
	publicData, err := h.service.GetManifest(false)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	internalData, err := h.service.GetManifest(true)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Combine categories from both manifests
	var combinedCategories []application.DocCategory
	if publicData != nil {
		combinedCategories = append(combinedCategories, publicData.Categories...)
	}
	if internalData != nil {
		combinedCategories = append(combinedCategories, internalData.Categories...)
	}

	combined := &application.DocManifest{
		Service:    publicData.Service,
		Version:    publicData.Version,
		Categories: combinedCategories,
	}

	c.JSON(200, gin.H{"data": combined})
}

func (h *DocsHandler) GetDoc(c *gin.Context) {
	slug := c.Param("slug")
	role := c.GetString("role")

	if role == "USER" {
		doc, err := h.service.GetDoc(slug, false)
		if err != nil {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.JSON(200, gin.H{"data": doc})
		return
	}

	// First try internal doc, fallback to public doc if not found
	doc, err := h.service.GetDoc(slug, true)
	if err != nil {
		doc, err = h.service.GetDoc(slug, false)
		if err != nil {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
	}

	c.JSON(200, gin.H{"data": doc})
}