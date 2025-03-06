package repository

import (
	"context"
	"time"

	"github.com/nijaru/nano-link/internal/models"
)

// URLRepository defines the interface for URL storage operations
type URLRepository interface {
	// Create stores a new URL
	Create(ctx context.Context, url *models.URL) error

	// GetByOriginalURL retrieves a URL by its original URL
	GetByOriginalURL(ctx context.Context, originalURL string) (*models.URL, error)

	// GetByCode retrieves a URL by its short code
	GetByCode(ctx context.Context, code string) (*models.URL, error)

	// IncrementVisits increments the visit counter for a URL
	IncrementVisits(ctx context.Context, code string) error

	// GetRecentURLs retrieves recent URLs with pagination
	GetRecentURLs(ctx context.Context, limit int) ([]*models.URL, error)

	// GetStats retrieves usage statistics
	GetStats(ctx context.Context) (*models.Stats, error)

	// CodeExists checks if a short code already exists
	CodeExists(ctx context.Context, code string) (bool, error)

	// DeleteOldURLs deletes URLs older than the specified age
	DeleteOldURLs(ctx context.Context, age time.Duration) (int64, error)

	// Close closes the repository connection
	Close() error
}
