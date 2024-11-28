package repository

import (
	"time"

	"github.com/nijaru/nano-link/internal/models"
)

type URLRepository interface {
	Create(url *models.URL) error
	GetByOriginalURL(originalURL string) (*models.URL, error)
	GetByCode(code string) (*models.URL, error)
	IncrementVisits(code string) error
	GetRecentURLs(limit int) ([]*models.URL, error)
	GetStats() (*models.Stats, error)
	CodeExists(code string) (bool, error)
	DeleteOldURLs(age time.Duration) error
	Close() error
}
