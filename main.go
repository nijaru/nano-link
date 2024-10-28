package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

var db *sql.DB
var baseURL string

var errorMessages = map[string]string{
	"InvalidRequestMethod": "Invalid request method",
	"InvalidJSON":          "Invalid JSON format",
	"InvalidURL":           "Invalid URL format",
	"NormalizeURLFailed":   "Failed to normalize URL",
	"ShortURLExists":       "Short URL already exists, please try again",
	"DatabaseError":        "Internal server error",
	"URLNotFound":          "URL not found",
}

const initialShortURLLength = 6
const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

var log = logrus.New()

func init() {
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)
}

func main() {
	initDB()
	defer closeDB()

	baseURL = getEnv("BASE_URL", "http://localhost:8080")

	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc("/shorten", shortenHandler)

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	loggedMux := loggingMiddleware(mux)
	limitedMux := rateLimitingMiddleware(loggedMux)

	port := getEnv("PORT", "8080")
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: limitedMux,
	}

	go startServer(server, port)

	gracefulShutdown(server)
}

func startServer(server *http.Server, port string) {
	log.Infof("Server started at :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v\n", port, err)
	}
}

func gracefulShutdown(server *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func initDB() {
	dataDir := "./data"
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "urls.db")
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	createTables()
	setDBConnectionPool()
}

func createTables() {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		short_url TEXT UNIQUE,
		original_url TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_short_url ON urls(short_url);`)
	if err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);`)
	if err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}
}

func setDBConnectionPool() {
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
}

func closeDB() {
	if err := db.Close(); err != nil {
		log.Fatalf("Failed to close database: %v", err)
	}
}

func insertOrUpdateURL(ctx context.Context, shortURL, originalURL string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var existingURL string
	err = tx.QueryRowContext(ctx, "SELECT original_url FROM urls WHERE short_url = ?", shortURL).Scan(&existingURL)
	if err == sql.ErrNoRows {
		return insertURL(tx, shortURL, originalURL)
	} else if err != nil {
		return fmt.Errorf("failed to query URL: %w", err)
	} else {
		return fmt.Errorf("short URL already exists")
	}
}

func insertURL(tx *sql.Tx, shortURL, originalURL string) error {
	_, err := tx.ExecContext(context.Background(), "INSERT INTO urls (short_url, original_url) VALUES (?, ?)", shortURL, originalURL)
	if err != nil {
		return fmt.Errorf("failed to insert URL: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func getOriginalURL(ctx context.Context, shortURL string) (string, error) {
	var originalURL string
	err := db.QueryRowContext(ctx, "SELECT original_url FROM urls WHERE short_url = ?", shortURL).Scan(&originalURL)
	if err != nil {
		return "", err
	}
	return originalURL, nil
}

func getShortURL(ctx context.Context, originalURL string) (string, error) {
	var shortURL string
	err := db.QueryRowContext(ctx, "SELECT short_url FROM urls WHERE original_url = ?", originalURL).Scan(&shortURL)
	if err != nil {
		return "", err
	}
	return shortURL, nil
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if r.Method != http.MethodPost {
		handleError(w, fmt.Errorf("invalid request method"), errorMessages["InvalidRequestMethod"], http.StatusMethodNotAllowed)
		return
	}

	urlRequest, err := parseRequestBody(r)
	if err != nil {
		handleError(w, err, errorMessages["InvalidJSON"], http.StatusBadRequest)
		return
	}

	normalizedURL, err := validateAndNormalizeURL(urlRequest.URL)
	if err != nil {
		handleError(w, err, errorMessages["InvalidURL"], http.StatusBadRequest)
		return
	}

	shortURL, err := generateAndStoreShortURL(r.Context(), normalizedURL)
	if err != nil {
		if err.Error() == "short URL already exists" {
			handleError(w, err, errorMessages["ShortURLExists"], http.StatusConflict)
		} else {
			handleError(w, err, errorMessages["DatabaseError"], http.StatusInternalServerError)
		}
		return
	}

	response := struct {
		ShortURL string `json:"short_url"`
	}{ShortURL: fmt.Sprintf("%s/%s", baseURL, shortURL)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	elapsed := time.Since(start)
	log.Infof("shortenHandler took %s", elapsed)
}

func parseRequestBody(r *http.Request) (struct {
	URL string `json:"url"`
}, error) {
	var urlRequest struct {
		URL string `json:"url"`
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return urlRequest, err
	}
	err = json.Unmarshal(body, &urlRequest)
	return urlRequest, err
}

func generateAndStoreShortURL(ctx context.Context, originalURL string) (string, error) {
	// Check if the original URL already exists in the database
	existingShortURL, err := getShortURL(ctx, originalURL)
	if err == nil {
		return existingShortURL, nil
	} else if err != sql.ErrNoRows {
		return "", fmt.Errorf("Database error")
	}

	shortURLLength := initialShortURLLength
	for {
		shortURL, err := generateShortURL(shortURLLength)
		if err != nil {
			return "", fmt.Errorf("Failed to generate short URL")
		}

		err = insertOrUpdateURL(ctx, shortURL, originalURL)
		if err == nil {
			return shortURL, nil
		} else if err.Error() == "short URL already exists" {
			shortURLLength++
		} else {
			return "", fmt.Errorf("Database error")
		}
	}
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	shortURL := r.URL.Path[1:]

	if shortURL == "" {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	originalURL, err := getOriginalURL(ctx, shortURL)
	if err != nil {
		if err == sql.ErrNoRows {
			handleError(w, err, errorMessages["URLNotFound"], http.StatusNotFound)
		} else {
			handleError(w, err, errorMessages["DatabaseError"], http.StatusInternalServerError)
		}
		return
	}

	http.Redirect(w, r, originalURL, http.StatusFound)

	elapsed := time.Since(start)
	log.Infof("redirectHandler took %s", elapsed)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFile(w, r, "./static/index.html")
		return
	}
	redirectHandler(w, r)
}

func generateShortURL(length int) (string, error) {
	b := make([]byte, length)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[num.Int64()]
	}
	return string(b), nil
}

func validateURL(urlStr string) error {
	parsedURL, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(parsedURL.Scheme, "http") {
		return fmt.Errorf("invalid URL scheme")
	}

	return nil
}

func normalizeURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Convert scheme and host to lowercase
	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	parsedURL.Host = strings.ToLower(parsedURL.Host)

	// Remove default ports
	if (parsedURL.Scheme == "http" && parsedURL.Port() == "80") || (parsedURL.Scheme == "https" && parsedURL.Port() == "443") {
		parsedURL.Host = parsedURL.Hostname()
	}

	// Remove trailing slash
	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/")

	// Sort query parameters (optional)
	parsedURL.RawQuery = sortQueryParams(parsedURL.Query())

	return parsedURL.String(), nil
}

func sortQueryParams(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var sortedParams []string
	for _, key := range keys {
		for _, value := range values[key] {
			sortedParams = append(sortedParams, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return strings.Join(sortedParams, "&")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Infof("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

func rateLimitingMiddleware(next http.Handler) http.Handler {
	limiter := rate.NewLimiter(5, 10) // Adjusted rate limit
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleError(w http.ResponseWriter, err error, message string, code int) {
	log.Errorf("Error: %v, Message: %s", err, message)
	http.Error(w, message, code)
}

func validateAndNormalizeURL(rawURL string) (string, error) {
	if err := validateURL(rawURL); err != nil {
		return "", err
	}
	return normalizeURL(rawURL)
}