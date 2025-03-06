package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"
	"regexp"
	"time"

	apperrors "github.com/nijaru/nano-link/internal/errors"
	"github.com/nijaru/nano-link/internal/models"
	"github.com/nijaru/nano-link/internal/repository"
)

// URLService provides business logic for URL operations
type URLService struct {
	repo repository.URLRepository
}

// NewURLService creates a new URL service
func NewURLService(repo repository.URLRepository) URLService {
	return URLService{repo: repo}
}

// GetURL retrieves a URL by its short code
func (s *URLService) GetURL(ctx context.Context, code string) (*models.URL, error) {
	if code == "" {
		return nil, apperrors.NewValidationError("code cannot be empty")
	}
	return s.repo.GetByCode(ctx, code)
}

// IncrementVisits increments the visit counter for a URL
func (s *URLService) IncrementVisits(ctx context.Context, code string) error {
	if code == "" {
		return apperrors.NewValidationError("code cannot be empty")
	}
	return s.repo.IncrementVisits(ctx, code)
}

// GetRecentURLs retrieves recent URLs with pagination
func (s *URLService) GetRecentURLs(ctx context.Context, limit int) ([]*models.URL, error) {
	if limit <= 0 {
		limit = 10 // Default to 10 if not specified
	}
	return s.repo.GetRecentURLs(ctx, limit)
}

// GetStats retrieves usage statistics
func (s *URLService) GetStats(ctx context.Context) (*models.Stats, error) {
	return s.repo.GetStats(ctx)
}

// CreateURLRequest represents the data needed to create a short URL
type CreateURLRequest struct {
	URL        string `json:"url"`
	CustomCode string `json:"custom_code,omitempty"`
}

// generateShortCode generates a cryptographically secure random short code
func generateShortCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", apperrors.WithMessage(err, "failed to generate random bytes")
	}
	return base64.URLEncoding.EncodeToString(b)[:6], nil
}

// validateAndSanitizeURL validates and sanitizes a URL
func (s *URLService) validateAndSanitizeURL(urlStr string) (string, error) {
	if urlStr == "" {
		return "", apperrors.NewValidationError("URL cannot be empty")
	}

	if len(urlStr) > 2048 {
		return "", apperrors.NewValidationError("URL is too long (max 2048 characters)")
	}

	// Add scheme if missing
	if !regexp.MustCompile(`^https?://`).MatchString(urlStr) {
		urlStr = "http://" + urlStr
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return "", apperrors.NewValidationError("Invalid URL format")
	}

	if u.Host == "" {
		return "", apperrors.NewValidationError("URL must have a host")
	}

	// Ensure URL has TLD or is localhost
	hostParts := regexp.MustCompile(`\.`).Split(u.Host, -1)
	if len(hostParts) < 2 && u.Host != "localhost" {
		return "", apperrors.NewValidationError("URL must have a valid domain")
	}

	return urlStr, nil
}

// isValidCustomCode validates a custom short code
func isValidCustomCode(code string) bool {
	if code == "" {
		return false
	}

	if len(code) < 4 || len(code) > 12 {
		return false
	}

	// Only allow alphanumeric characters and some safe symbols
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, code)
	return match
}

// CreateShortURL creates a new short URL
func (s *URLService) CreateShortURL(ctx context.Context, urlStr string, customCode string) (*models.URL, error) {
	// Validate URL
	cleanURL, err := s.validateAndSanitizeURL(urlStr)
	if err != nil {
		return nil, err
	}

	// Validate custom code if provided
	var shortCode string
	if customCode != "" {
		if !isValidCustomCode(customCode) {
			return nil, apperrors.NewValidationError("Invalid custom code format")
		}

		// Check if custom code already exists
		exists, err := s.repo.CodeExists(ctx, customCode)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, apperrors.NewValidationError("Custom code already in use")
		}
		shortCode = customCode
	} else {
		// Generate a random short code
		code, err := generateShortCode()
		if err != nil {
			return nil, err
		}
		shortCode = code
	}

	// Check if URL already exists
	existingURL, err := s.repo.GetByOriginalURL(ctx, cleanURL)
	if err != nil {
		// Only return error if it's not a NotFound error
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && !errors.Is(err, apperrors.ErrNotFound) {
			return nil, err
		}
	}

	// Return existing URL if found
	if existingURL != nil {
		return existingURL, nil
	}

	// Create new short URL
	url := &models.URL{
		OriginalURL: cleanURL,
		ShortCode:   shortCode,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, url); err != nil {
		return nil, err
	}

	return url, nil
}