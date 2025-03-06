package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/nijaru/nano-link/internal/config"
	"github.com/nijaru/nano-link/internal/handlers"
	customLogger "github.com/nijaru/nano-link/internal/logger"
	"github.com/nijaru/nano-link/internal/middleware"
	"github.com/nijaru/nano-link/internal/repository"
	"github.com/nijaru/nano-link/internal/service"
	"github.com/nijaru/nano-link/internal/tasks"
)

func main() {
	// Initialize logger
	customLogger.Init()
	customLogger.Info("Starting nano-link service")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		customLogger.Error(err, "Failed to load configuration")
		os.Exit(1)
	}
	customLogger.Info("Configuration loaded successfully")

	// Setup Fiber with optimized settings
	app := fiber.New(fiber.Config{
		ErrorHandler:          handlers.ErrorHandler,
		DisableStartupMessage: true,
		Prefork:               false, // Set to true in production for better performance
		ReadTimeout:           5 * time.Second,
		WriteTimeout:          10 * time.Second,
		IdleTimeout:           120 * time.Second,
	})

	// Apply middleware
	app.Use(recover.New())      // Recover from panics
	app.Use(helmet.New())       // Security headers
	app.Use(compress.New())     // Compression
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,OPTIONS",
	}))
	app.Use(middleware.RateLimit(cfg.RateLimit, cfg.RateLimitWindow))

	// Initialize repository with context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	repo, err := repository.NewSQLiteRepository(cfg.DBPath)
	cancel()
	if err != nil {
		customLogger.Error(err, "Failed to initialize repository")
		os.Exit(1)
	}
	customLogger.Info("Repository initialized successfully")

	// Initialize service and handlers
	urlService := service.NewURLService(repo)
	urlHandler := handlers.NewURLHandler(&urlService)

	// Start cleanup task
	cleanupTask := tasks.NewCleanupTask(repo, cfg.CleanupInterval, cfg.MaxURLAge)
	cleanupTask.Start()

	// Setup routes
	setupRoutes(app, urlHandler)

	// Start server in a goroutine
	go func() {
		customLogger.Info("Starting server", map[string]interface{}{
			"port": cfg.Port,
			"base_url": cfg.BaseURL,
		})
		
		if err := app.Listen(":" + cfg.Port); err != nil {
			customLogger.Error(err, "Server failed to start")
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	customLogger.Info("Shutting down server...")

	// Stop the cleanup task
	cleanupTask.Stop()
	customLogger.Info("Cleanup task stopped")

	// Shutdown server with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		customLogger.Error(err, "Error during server shutdown")
	}

	// Close repository
	if err := repo.Close(); err != nil {
		customLogger.Error(err, "Error closing repository")
	}

	customLogger.Info("Server gracefully stopped")
}

// setupRoutes defines all the API routes
func setupRoutes(app *fiber.App, handler *handlers.URLHandler) {
	// API routes
	api := app.Group("/api")
	{
		api.Post("/shorten", handler.CreateShortURL)
		api.Get("/urls/:code", handler.GetURLInfo)
		api.Get("/urls", handler.GetRecentURLs)
		api.Get("/stats", handler.GetStats)
	}

	// Status endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Main redirect route
	app.Get("/:code", handler.HandleRedirect)

	// Serve static files with caching
	app.Static("/", "./static", fiber.Static{
		Compress:      true,
		ByteRange:     true,
		CacheDuration: 24 * time.Hour,
	})

	// 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Endpoint not found",
		})
	})
}
