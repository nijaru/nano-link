package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/nijaru/nano-link/internal/errors"
	"github.com/nijaru/nano-link/internal/models"
	"github.com/nijaru/nano-link/internal/service"
)

type URLHandler struct {
	service *service.URLService
}

func NewURLHandler(service *service.URLService) *URLHandler {
	return &URLHandler{service: service}
}

func (h *URLHandler) CreateShortURL(c *fiber.Ctx) error {
	var request struct {
		URL string `json:"url"`
	}

	if err := c.BodyParser(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	url, err := h.service.CreateShortURL(request.URL)
	if err != nil {
		switch err.(type) {
		case *errors.AppError:
			appErr := err.(*errors.AppError)
			return fiber.NewError(fiber.StatusBadRequest, appErr.Message)
		default:
			return fiber.NewError(fiber.StatusInternalServerError, "Failed to create short URL")
		}
	}

	return c.JSON(models.URLResponse{
		URL:      *url,
		ShortURL: buildShortURL(c, url.ShortCode),
	})
}

// HandleRedirect handles redirecting short URLs to their original URL
func (h *URLHandler) HandleRedirect(c *fiber.Ctx) error {
	code := c.Params("code")

	url, err := h.service.GetURL(code)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to retrieve URL")
	}

	if url == nil {
		return c.Redirect("/") // Redirect to homepage if URL not found
	}

	// Increment visits asynchronously
	go func() {
		if err := h.service.IncrementVisits(code); err != nil {
			log.Printf("Failed to increment visits: %v", err)
		}
	}()

	return c.Redirect(url.OriginalURL, fiber.StatusTemporaryRedirect)
}

// GetURLInfo returns information about a shortened URL
func (h *URLHandler) GetURLInfo(c *fiber.Ctx) error {
	code := c.Params("code")

	url, err := h.service.GetURL(code)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to retrieve URL")
	}

	if url == nil {
		return fiber.NewError(fiber.StatusNotFound, "URL not found")
	}

	return c.JSON(models.URLResponse{
		URL:      *url,
		ShortURL: buildShortURL(c, url.ShortCode),
	})
}

// GetRecentURLs returns recently created short URLs
func (h *URLHandler) GetRecentURLs(c *fiber.Ctx) error {
	limit := 10
	urls, err := h.service.GetRecentURLs(limit)
	if err != nil {
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
	stats, err := h.service.GetStats()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to retrieve stats")
	}

	return c.JSON(stats)
}

// Utility function to build the full short URL
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
		log.Printf("Error: %v", err)
	}

	return c.Status(code).JSON(fiber.Map{
		"error": message,
	})
}
