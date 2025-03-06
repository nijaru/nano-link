package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	customLogger "github.com/nijaru/nano-link/internal/logger"
	"github.com/nijaru/nano-link/internal/errors"
)

// RateLimit creates a new rate limiting middleware
func RateLimit(max int, window time.Duration) fiber.Handler {
	// Validate parameters
	if max <= 0 {
		customLogger.Error(
			errors.NewValidationError("rate limit must be positive"),
			"Invalid rate limit configuration",
		)
		max = 100 // Default fallback
	}

	if window <= 0 {
		customLogger.Error(
			errors.NewValidationError("rate limit window must be positive"),
			"Invalid rate limit configuration",
		)
		window = time.Minute // Default fallback
	}

	customLogger.Info("Initializing rate limiter", map[string]interface{}{
		"max":    max,
		"window": window.String(),
	})

	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: window,
		// Consider using X-Forwarded-For or X-Real-IP headers for proxied requests
		KeyGenerator: func(c *fiber.Ctx) string {
			// First try X-Forwarded-For and X-Real-IP headers for proxied requests
			if ip := c.Get("X-Forwarded-For"); ip != "" {
				return ip
			}
			if ip := c.Get("X-Real-IP"); ip != "" {
				return ip
			}
			// Fall back to direct IP
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			// Log rate limit exceeded
			customLogger.Debug("Rate limit exceeded", map[string]interface{}{
				"ip":     c.IP(),
				"path":   c.Path(),
				"method": c.Method(),
			})
			
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Please try again later.",
			})
		},
		// Skip rate limiting for static resources
		SkipFailedRequests: false,
		SkipSuccessfulRequests: false,
		Next: func(c *fiber.Ctx) bool {
			// Skip rate limiting for static files
			return c.Path() == "/" || 
				   c.Path() == "/favicon.ico" || 
				   c.Method() == fiber.MethodGet && c.Path() == "/static"
		},
	})
}
