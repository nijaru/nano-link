package repository

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nijaru/nano-link/internal/models"
)

type SQLiteRepository struct {
	db *sql.DB
}

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
	insertURLSQL       = `INSERT INTO urls (original_url, short_code) VALUES (?, ?)`
	getURLByCodeSQL    = `SELECT id, original_url, short_code, visits, created_at FROM urls WHERE short_code = ?`
	incrementVisitsSQL = `UPDATE urls SET visits = visits + 1 WHERE short_code = ?`
	getRecentURLsSQL   = `
        SELECT id, original_url, short_code, visits, created_at
        FROM urls
        ORDER BY created_at DESC
        LIMIT ?
    `
	deleteOldURLsSQL = `DELETE FROM urls WHERE created_at < ?`
	checkCodeSQL     = `SELECT EXISTS(SELECT 1 FROM urls WHERE short_code = ?)`
)

func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, err
	}

	return &SQLiteRepository{db: db}, nil
}

func (r *SQLiteRepository) Create(url *models.URL) error {
	url.CreatedAt = time.Now()
	result, err := r.db.Exec(
		"INSERT INTO urls (original_url, short_code, created_at) VALUES (?, ?, datetime(?))",
		url.OriginalURL,
		url.ShortCode,
		url.CreatedAt.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	url.ID = id
	return nil
}

func (r *SQLiteRepository) GetByOriginalURL(originalURL string) (*models.URL, error) {
	url := &models.URL{}
	err := r.db.QueryRow(
		"SELECT id, original_url, short_code, visits, created_at FROM urls WHERE original_url = ?",
		originalURL,
	).Scan(
		&url.ID,
		&url.OriginalURL,
		&url.ShortCode,
		&url.Visits,
		&url.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return url, nil
}

func (r *SQLiteRepository) GetByCode(code string) (*models.URL, error) {
	url := &models.URL{}
	err := r.db.QueryRow(getURLByCodeSQL, code).Scan(
		&url.ID,
		&url.OriginalURL,
		&url.ShortCode,
		&url.Visits,
		&url.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return url, nil
}

func (r *SQLiteRepository) IncrementVisits(code string) error {
	_, err := r.db.Exec(incrementVisitsSQL, code)
	return err
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

func (r *SQLiteRepository) GetRecentURLs(limit int) ([]*models.URL, error) {
	const query = `
        SELECT id, original_url, short_code, visits, created_at
        FROM urls
        ORDER BY created_at DESC
        LIMIT ?
    `

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		urls = append(urls, url)
	}

	return urls, rows.Err()
}

func (r *SQLiteRepository) GetStats() (*models.Stats, error) {
	stats := &models.Stats{}

	// Get total URLs
	err := r.db.QueryRow("SELECT COUNT(*) FROM urls").Scan(&stats.TotalURLs)
	if err != nil {
		return nil, err
	}

	// Get total visits - handle NULL case
	var totalVisits sql.NullInt64
	err = r.db.QueryRow("SELECT COALESCE(SUM(visits), 0) FROM urls").Scan(&totalVisits)
	if err != nil {
		return nil, err
	}
	stats.TotalVisits = totalVisits.Int64 // Will be 0 if NULL

	// Get most recent URL date
	var lastCreated sql.NullString
	err = r.db.QueryRow("SELECT MAX(datetime(created_at)) FROM urls").Scan(&lastCreated)
	if err != nil {
		return nil, err
	}
	if lastCreated.Valid {
		stats.LastCreated = lastCreated.String
	}

	return stats, nil
}

// For cleanup/maintenance
func (r *SQLiteRepository) DeleteOldURLs(age time.Duration) error {
	cutoff := time.Now().Add(-age)
	_, err := r.db.Exec("DELETE FROM urls WHERE created_at < ?", cutoff)
	return err
}

// For checking if a short code already exists
func (r *SQLiteRepository) CodeExists(code string) (bool, error) {
	var exists bool
	err := r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM urls WHERE short_code = ?)", code).Scan(&exists)
	return exists, err
}
