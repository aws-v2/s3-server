package application

import (
	"context"
	"fmt"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SearchService struct {
	repo domain.RepositoryPort
}

func NewSearchService(repo domain.RepositoryPort) *SearchService {
	return &SearchService{
		repo: repo,
	}
}

// SearchFiles searches files by name/key
func (s *SearchService) SearchFiles(ctx context.Context, input dto.SearchFilesInput) (*dto.SearchResultOutput, error) {
	files, err := s.repo.SearchFilesByName(ctx, input.BucketID, input.Query, input.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	results := make([]dto.SearchResult, len(files))
	for i, file := range files {
		results[i] = dto.SearchResult{
			ID:          file.ID,
			BucketID:    file.BucketID,
			Key:         file.Key,
			Size:        file.Size,
			ContentType: file.ContentType,
			Metadata:    file.Metadata,
			CreatedAt:   file.CreatedAt,
			Relevance:   calculateRelevance(file.Key, input.Query),
		}
	}

	s.saveSearchHistory(ctx, input.Query, len(results))

	return &dto.SearchResultOutput{
		Results: results,
		Total:   len(results),
	}, nil
}

// SearchByMetadata searches by metadata
func (s *SearchService) SearchByMetadata(ctx context.Context, input dto.SearchByMetadataInput) (*dto.SearchResultOutput, error) {
	files, err := s.repo.SearchFilesByMetadata(ctx, input.BucketID, input.Metadata, input.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search by metadata: %w", err)
	}

	results := make([]dto.SearchResult, len(files))
	for i, file := range files {
		results[i] = dto.SearchResult{
			ID:          file.ID,
			BucketID:    file.BucketID,
			Key:         file.Key,
			Size:        file.Size,
			ContentType: file.ContentType,
			Metadata:    file.Metadata,
			CreatedAt:   file.CreatedAt,
		}
	}

	return &dto.SearchResultOutput{
		Results: results,
		Total:   len(results),
	}, nil
}

// SearchByTags searches by tags
func (s *SearchService) SearchByTags(ctx context.Context, input dto.SearchByTagsInput) (*dto.SearchResultOutput, error) {
	files, err := s.repo.SearchFilesByTags(ctx, input.BucketID, input.Tags, input.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search by tags: %w", err)
	}

	results := make([]dto.SearchResult, len(files))
	for i, file := range files {
		tags := []string{}
		if tagsStr, ok := file.Metadata["tags"]; ok {
			tags = strings.Split(tagsStr, ",")
		}

		results[i] = dto.SearchResult{
			ID:          file.ID,
			BucketID:    file.BucketID,
			Key:         file.Key,
			Size:        file.Size,
			ContentType: file.ContentType,
			Metadata:    file.Metadata,
			Tags:        tags,
			CreatedAt:   file.CreatedAt,
		}
	}

	return &dto.SearchResultOutput{
		Results: results,
		Total:   len(results),
	}, nil
}

// SearchByContent searches by content (placeholder for full-text search)
func (s *SearchService) SearchByContent(ctx context.Context, input dto.SearchByContentInput) (*dto.SearchResultOutput, error) {
	// This is a placeholder. Full-text search would require indexing file contents
	// For now, search by filename as fallback
	return s.SearchFiles(ctx, dto.SearchFilesInput{
		Query:    input.Query,
		BucketID: input.BucketID,
		Limit:    input.Limit,
	})
}

// AdvancedSearch performs advanced search with multiple filters
func (s *SearchService) AdvancedSearch(ctx context.Context, input dto.AdvancedSearchInput) (*dto.SearchResultOutput, error) {
	files, err := s.repo.AdvancedSearchFiles(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to perform advanced search: %w", err)
	}

	results := make([]dto.SearchResult, len(files))
	for i, file := range files {
		results[i] = dto.SearchResult{
			ID:          file.ID,
			BucketID:    file.BucketID,
			Key:         file.Key,
			Size:        file.Size,
			ContentType: file.ContentType,
			Metadata:    file.Metadata,
			CreatedAt:   file.CreatedAt,
		}
	}

	if input.Query != "" {
		s.saveSearchHistory(ctx, input.Query, len(results))
	}

	return &dto.SearchResultOutput{
		Results: results,
		Total:   len(results),
	}, nil
}

// GetSearchSuggestions gets search suggestions
func (s *SearchService) GetSearchSuggestions(ctx context.Context, input dto.SearchSuggestionsInput) (*dto.SearchSuggestionsOutput, error) {
	suggestions, err := s.repo.GetSearchSuggestions(ctx, input.BucketID, input.Query, input.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get suggestions: %w", err)
	}

	return &dto.SearchSuggestionsOutput{
		Suggestions: suggestions,
	}, nil
}

// GetSearchHistory gets search history
func (s *SearchService) GetSearchHistory(ctx context.Context, input dto.SearchHistoryInput) (*dto.SearchHistoryOutput, error) {
	history, err := s.repo.GetSearchHistory(ctx, input.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get search history: %w", err)
	}

	historyItems := make([]dto.SearchHistoryItem, len(history))
	for i, item := range history {
		historyItems[i] = dto.SearchHistoryItem{
			Query:     item.Query,
			Timestamp: item.Timestamp,
			Results:   item.Results,
		}
	}

	return &dto.SearchHistoryOutput{
		History: historyItems,
	}, nil
}

// SaveSearch saves a search query
func (s *SearchService) SaveSearch(ctx context.Context, input dto.SaveSearchInput) (*dto.SaveSearchOutput, error) {
	savedSearch := &domain.SavedSearch{
		ID:          uuid.New().String(),
		Name:        input.Name,
		Query:       input.Query,
		Filters:     input.Filters,
		Description: input.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.SaveSearchQuery(ctx, savedSearch); err != nil {
		return nil, fmt.Errorf("failed to save search: %w", err)
	}

	return &dto.SaveSearchOutput{
		ID:        savedSearch.ID,
		Name:      savedSearch.Name,
		CreatedAt: savedSearch.CreatedAt,
	}, nil
}

func (s *SearchService) saveSearchHistory(ctx context.Context, query string, results int) {
	history := &domain.SearchHistory{
		ID:        uuid.New().String(),
		Query:     query,
		Results:   results,
		Timestamp: time.Now(),
	}
	s.repo.SaveSearchHistory(ctx, history)
}

func calculateRelevance(key, query string) float64 {
	keyLower := strings.ToLower(key)
	queryLower := strings.ToLower(query)

	if keyLower == queryLower {
		return 1.0
	}
	if strings.HasPrefix(keyLower, queryLower) {
		return 0.9
	}
	if strings.Contains(keyLower, queryLower) {
		return 0.7
	}
	return 0.5
}