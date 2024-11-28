package tasks

import (
	"time"

	customLogger "github.com/nijaru/nano-link/internal/logger"
	"github.com/nijaru/nano-link/internal/repository"
)

func StartCleanupTask(repo repository.URLRepository, interval, maxAge time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := repo.DeleteOldURLs(maxAge); err != nil {
				customLogger.Error(err, "Failed to cleanup old URLs")
			} else {
				customLogger.Info("Cleanup task completed successfully")
			}
		}
	}()
}
