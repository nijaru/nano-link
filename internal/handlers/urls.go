package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	appErrors "github.com/nijaru/nano-link/internal/errors"
	customLogger "github.com/nijaru/nano-link/internal/logger"
	"github.com/nijaru/nano-link/internal/models"
	"github.com/nijaru/nano-link/internal/service"
)

// URLHandler handles HTTP requests related to URLs
type URLHandler struct {
	service *service.URLService
}

// NewURLHandler creates a new URL handler
func NewURLHandler(service *service.URLService) *URLHandler {
	return &URLHandler{service: service}
}

// CreateShortURL handles the creation of a new short URL
func (h *URLHandler) CreateShortURL(c *fiber.Ctx) error {
	// Extract request context
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	// Parse request body
	var request service.CreateURLRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	// Create short URL
	url, err := h.service.CreateShortURL(ctx, request.URL, request.CustomCode)
	if err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) {
			switch {
			case errors.Is(err, appErrors.ErrInvalidInput):
				return fiber.NewError(fiber.StatusBadRequest, appErr.Message)
			case errors.Is(err, appErrors.ErrInternalError):
				customLogger.Error(err, "Failed to create short URL")
				return fiber.NewError(fiber.StatusInternalServerError, "Failed to create short URL")
			default:
				return fiber.NewError(fiber.StatusBadRequest, appErr.Message)
			}
		}
		customLogger.Error(err, "Unexpected error creating short URL")
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to create short URL")
	}

	return c.JSON(models.URLResponse{
		URL:      *url,
		ShortURL: buildShortURL(c, url.ShortCode),
	})
}

// HandleRedirect handles redirecting short URLs to their original URL
func (h *URLHandler) HandleRedirect(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	
	code := c.Params("code")
	if code == "" {
		return c.Redirect("/") // Redirect to homepage if no code provided
	}

	url, err := h.service.GetURL(ctx, code)
	if err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) && errors.Is(err, appErrors.ErrNotFound) {
			return c.Redirect("/") // Redirect to homepage if URL not found
		}
		customLogger.Error(err, "Failed to retrieve URL", map[string]interface{}{"code": code})
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to process redirect")
	}

	// Increment visits with a separate context that won't be canceled when this handler returns
	backgroundCtx := context.Background()
	go func(ctx context.Context, code string) {
		if err := h.service.IncrementVisits(ctx, code); err != nil {
			customLogger.Error(err, "Failed to increment visits", map[string]interface{}{"code": code})
		}
	}(backgroundCtx, code)

	return c.Redirect(url.OriginalURL, fiber.StatusTemporaryRedirect)
}

// GetURLInfo returns information about a shortened URL
func (h *URLHandler) GetURLInfo(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	
	code := c.Params("code")
	if code == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Code parameter is required")
	}

	url, err := h.service.GetURL(ctx, code)
	if err != nil {
		var appErr *appErrors.AppError
		if errors.As(err, &appErr) && errors.Is(err, appErrors.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "URL not found")
		}
		customLogger.Error(err, "Failed to retrieve URL", map[string]interface{}{"code": code})
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to retrieve URL")
	}

	return c.JSON(models.URLResponse{
		URL:      *url,
		ShortURL: buildShortURL(c, url.ShortCode),
	})
}

// GetRecentURLs returns recently created short URLs
func (h *URLHandler) GetRecentURLs(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	
	// Parse query parameters
	limit := 10
	if c.Query("limit") != "" {
		i := c.QueryInt("limit", 10)
		if i > 0 && i <= 100 {
			limit = i
		}
	}

	urls, err := h.service.GetRecentURLs(ctx, limit)
	if err != nil {
		customLogger.Error(err, "Failed to retrieve recent URLs")
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to retrieve recent URLs")
	}

	// Convert URLs to responses with full short URLs
	responses := make([]models.URLResponse, len(urls))
	for i, url := range urls {
		responses[i] = models.URLResponse{
			URL:      *url,
			ShortURL: buildShortURL(c, url.ShortCode),
		}
	}

	return c.JSON(fiber.Map{
		"urls": responses,
	})
}

// GetStats returns usage statistics
func (h *URLHandler) GetStats(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	
	stats, err := h.service.GetStats(ctx)
	if err != nil {
		customLogger.Error(err, "Failed to retrieve stats")
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to retrieve stats")
	}

	return c.JSON(stats)
}

// buildShortURL builds the full short URL from a code
func buildShortURL(c *fiber.Ctx, code string) string {
	return c.Protocol() + "://" + c.Hostname() + "/" + code
}

// ErrorHandler handles application errors
func ErrorHandler(c *fiber.Ctx, err error) error {
	// Default to 500 Internal Server Error
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	// Check if it's a Fiber error
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	// Log non-400 errors
	if code >= 500 {
		customLogger.Error(err, "Server error", map[string]interface{}{
			"status":  code,
			"path":    c.Path(),
			"method":  c.Method(),
			"ip":      c.IP(),
			"message": message,
		})
	} else {
		customLogger.Debug("Client error", map[string]interface{}{
			"status":  code,
			"path":    c.Path(),
			"method":  c.Method(),
			"ip":      c.IP(),
			"message": message,
		})
	}

	return c.Status(code).JSON(fiber.Map{
		"error": message,
	})
}