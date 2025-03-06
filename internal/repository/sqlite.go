package repository

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nijaru/nano-link/internal/errors"
	"github.com/nijaru/nano-link/internal/models"
)

// SQLiteRepository implements URLRepository interface using SQLite database
type SQLiteRepository struct {
	db *sql.DB
}

// SQL queries as constants to avoid duplication and enforce consistency
const (
	createTableSQL = `
		CREATE TABLE IF NOT EXISTS urls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			original_url TEXT NOT NULL,
			short_code TEXT UNIQUE NOT NULL,
			visits INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_short_code ON urls(short_code);
		CREATE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);
	`
	insertURLSQL       = `INSERT INTO urls (original_url, short_code, created_at) VALUES (?, ?, datetime(?))`
	getURLByCodeSQL    = `SELECT id, original_url, short_code, visits, created_at FROM urls WHERE short_code = ?`
	getURLByOriginalSQL = `SELECT id, original_url, short_code, visits, created_at FROM urls WHERE original_url = ?`
	incrementVisitsSQL = `UPDATE urls SET visits = visits + 1 WHERE short_code = ?`
	getRecentURLsSQL   = `
		SELECT id, original_url, short_code, visits, created_at
		FROM urls
		ORDER BY created_at DESC
		LIMIT ?
	`
	deleteOldURLsSQL = `DELETE FROM urls WHERE created_at < datetime(?)`
	checkCodeSQL     = `SELECT EXISTS(SELECT 1 FROM urls WHERE short_code = ?)`
	getStatsCountSQL = `SELECT COUNT(*) FROM urls`
	getStatsSumSQL   = `SELECT COALESCE(SUM(visits), 0) FROM urls`
	getStatsLatestSQL = `SELECT MAX(datetime(created_at)) FROM urls`
)

// NewSQLiteRepository creates a new SQLite repository
func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	// Initialize database schema
	if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	return &SQLiteRepository{db: db}, nil
}

// Create stores a new URL in the database
func (r *SQLiteRepository) Create(ctx context.Context, url *models.URL) error {
	if url == nil {
		return errors.NewValidationError("url cannot be nil")
	}

	// Ensure created time is set
	if url.CreatedAt.IsZero() {
		url.CreatedAt = time.Now()
	}

	result, err := r.db.ExecContext(
		ctx,
		insertURLSQL,
		url.OriginalURL,
		url.ShortCode,
		url.CreatedAt.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return errors.NewDatabaseError(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return errors.NewDatabaseError(err)
	}

	url.ID = id
	return nil
}

// GetByOriginalURL retrieves a URL by its original URL
func (r *SQLiteRepository) GetByOriginalURL(ctx context.Context, originalURL string) (*models.URL, error) {
	if originalURL == "" {
		return nil, errors.NewValidationError("original URL cannot be empty")
	}

	url := &models.URL{}
	err := r.db.QueryRowContext(
		ctx,
		getURLByOriginalSQL,
		originalURL,
	).Scan(
		&url.ID,
		&url.OriginalURL,
		&url.ShortCode,
		&url.Visits,
		&url.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.NewNotFoundError("URL not found")
	}
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	return url, nil
}

// GetByCode retrieves a URL by its short code
func (r *SQLiteRepository) GetByCode(ctx context.Context, code string) (*models.URL, error) {
	if code == "" {
		return nil, errors.NewValidationError("code cannot be empty")
	}

	url := &models.URL{}
	err := r.db.QueryRowContext(ctx, getURLByCodeSQL, code).Scan(
		&url.ID,
		&url.OriginalURL,
		&url.ShortCode,
		&url.Visits,
		&url.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.NewNotFoundError("URL not found")
	}
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	return url, nil
}

// IncrementVisits increments the visit counter for a URL
func (r *SQLiteRepository) IncrementVisits(ctx context.Context, code string) error {
	if code == "" {
		return errors.NewValidationError("code cannot be empty")
	}

	result, err := r.db.ExecContext(ctx, incrementVisitsSQL, code)
	if err != nil {
		return errors.NewDatabaseError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.NewDatabaseError(err)
	}

	if rowsAffected == 0 {
		return errors.NewNotFoundError("URL not found")
	}

	return nil
}

// Close closes the repository connection
func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

// GetRecentURLs retrieves recent URLs with pagination
func (r *SQLiteRepository) GetRecentURLs(ctx context.Context, limit int) ([]*models.URL, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}

	rows, err := r.db.QueryContext(ctx, getRecentURLsSQL, limit)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	defer rows.Close()

	var urls []*models.URL
	for rows.Next() {
		url := &models.URL{}
		err := rows.Scan(
			&url.ID,
			&url.OriginalURL,
			&url.ShortCode,
			&url.Visits,
			&url.CreatedAt,
		)
		if err != nil {
			return nil, errors.NewDatabaseError(err)
		}
		urls = append(urls, url)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	return urls, nil
}

// GetStats retrieves usage statistics
func (r *SQLiteRepository) GetStats(ctx context.Context) (*models.Stats, error) {
	stats := &models.Stats{}

	// Get total URLs
	err := r.db.QueryRowContext(ctx, getStatsCountSQL).Scan(&stats.TotalURLs)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	// Get total visits - handle NULL case
	var totalVisits sql.NullInt64
	err = r.db.QueryRowContext(ctx, getStatsSumSQL).Scan(&totalVisits)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	stats.TotalVisits = totalVisits.Int64 // Will be 0 if NULL

	// Get most recent URL date
	var lastCreated sql.NullString
	err = r.db.QueryRowContext(ctx, getStatsLatestSQL).Scan(&lastCreated)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	if lastCreated.Valid {
		stats.LastCreated = lastCreated.String
	}

	return stats, nil
}

// DeleteOldURLs deletes URLs older than the specified age
func (r *SQLiteRepository) DeleteOldURLs(ctx context.Context, age time.Duration) (int64, error) {
	if age <= 0 {
		return 0, errors.NewValidationError("age must be positive")
	}

	cutoff := time.Now().Add(-age).UTC().Format("2006-01-02 15:04:05")
	result, err := r.db.ExecContext(ctx, deleteOldURLsSQL, cutoff)
	if err != nil {
		return 0, errors.NewDatabaseError(err)
	}

	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return 0, errors.NewDatabaseError(err)
	}

	return rowsDeleted, nil
}

// CodeExists checks if a short code already exists
func (r *SQLiteRepository) CodeExists(ctx context.Context, code string) (bool, error) {
	if code == "" {
		return false, errors.NewValidationError("code cannot be empty")
	}

	var exists bool
	err := r.db.QueryRowContext(ctx, checkCodeSQL, code).Scan(&exists)
	if err != nil {
		return false, errors.NewDatabaseError(err)
	}
	
	return exists, nil
}
