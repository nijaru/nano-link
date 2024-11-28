package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"

	"github.com/nijaru/nano-link/internal/models"
	"github.com/nijaru/nano-link/internal/repository"
)

var ErrInvalidURL = errors.New("invalid URL")

type URLService struct {
	repo repository.URLRepository
}

func NewURLService(repo repository.URLRepository) URLService {
	return URLService{repo: repo}
}

func (s *URLService) GetURL(code string) (*models.URL, error) {
	return s.repo.GetByCode(code)
}

func (s *URLService) IncrementVisits(code string) error {
	return s.repo.IncrementVisits(code)
}

// Utility functions
func generateShortCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:6]
}

func isValidURL(urlStr string) bool {
	if len(urlStr) == 0 || len(urlStr) > 2048 {
		return false
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	return u.Scheme != "" && u.Host != ""
}

func (s URLService) GetRecentURLs(limit int) ([]*models.URL, error) {
	return s.repo.GetRecentURLs(limit)
}

func (s URLService) GetStats() (*models.Stats, error) {
	return s.repo.GetStats()
}

type CreateURLRequest struct {
	URL        string `json:"url"`
	CustomCode string `json:"custom_code,omitempty"`
}

func (s *URLService) validateAndSanitizeURL(urlStr string) (string, error) {
	if len(urlStr) == 0 || len(urlStr) > 2048 {
		return "", ErrInvalidURL
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return "", ErrInvalidURL
	}

	if u.Scheme == "" {
		urlStr = "http://" + urlStr
		u, err = url.Parse(urlStr)
		if err != nil {
			return "", ErrInvalidURL
		}
	}

	if u.Host == "" {
		return "", ErrInvalidURL
	}

	return urlStr, nil
}

func (s *URLService) CreateShortURL(urlStr string) (*models.URL, error) {
	// Validate URL
	cleanURL, err := s.validateAndSanitizeURL(urlStr)
	if err != nil {
		return nil, err
	}

	// Check if URL already exists
	existingURL, err := s.repo.GetByOriginalURL(cleanURL)
	if err != nil {
		return nil, err
	}
	if existingURL != nil {
		return existingURL, nil
	}

	// Create new short URL if it doesn't exist
	shortCode := generateShortCode()
	url := &models.URL{
		OriginalURL: cleanURL,
		ShortCode:   shortCode,
	}

	if err := s.repo.Create(url); err != nil {
		return nil, err
	}

	return url, nil
}

func isValidCustomCode(code string) bool {
	if len(code) < 4 || len(code) > 12 {
		return false
	}
	// Add more validation as needed
	return true
}
