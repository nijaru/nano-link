package main

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/helmet"

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

	// Load configuration
	config, err := config.LoadConfig()
	if err != nil {
		customLogger.Error(err, "Failed to load configuration")
		os.Exit(1)
	}

	// Setup Fiber
	app := fiber.New(fiber.Config{
		ErrorHandler: handlers.ErrorHandler,
	})

	// Security middleware
	app.Use(helmet.New())
	app.Use(middleware.RateLimit(config.RateLimit, config.RateLimitWindow))

	// Initialize repository
	repo, err := repository.NewSQLiteRepository(config.DBPath)
	if err != nil {
		customLogger.Error(err, "Failed to initialize repository")
		os.Exit(1)
	}
	defer repo.Close()

	// Start cleanup task
	tasks.StartCleanupTask(repo, config.CleanupInterval, config.MaxURLAge)

	// Initialize service and handlers
	urlService := service.NewURLService(repo)
	urlHandler := handlers.NewURLHandler(&urlService)

	// Setup routes
	setupRoutes(app, urlHandler)

	// Start server
	customLogger.Info("Starting server", map[string]interface{}{"port": config.Port})
	if err := app.Listen(":" + config.Port); err != nil {
		customLogger.Error(err, "Server failed to start")
		os.Exit(1)
	}
}

func setupRoutes(app *fiber.App, handler *handlers.URLHandler) {
	// API routes
	api := app.Group("/api")
	api.Post("/shorten", handler.CreateShortURL)
	api.Get("/urls/:code", handler.GetURLInfo)
	api.Get("/urls", handler.GetRecentURLs)
	api.Get("/stats", handler.GetStats)

	// Main redirect route
	app.Get("/:code", handler.HandleRedirect)

	// Serve static files
	app.Static("/", "./static")
}
