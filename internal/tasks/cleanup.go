package tasks

import (
	"context"
	"sync"
	"time"

	customLogger "github.com/nijaru/nano-link/internal/logger"
	"github.com/nijaru/nano-link/internal/repository"
)

// CleanupTask represents a background task for cleaning up old URLs
type CleanupTask struct {
	repo       repository.URLRepository
	interval   time.Duration
	maxAge     time.Duration
	ticker     *time.Ticker
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	ctx        context.Context
}

// NewCleanupTask creates a new cleanup task
func NewCleanupTask(repo repository.URLRepository, interval, maxAge time.Duration) *CleanupTask {
	ctx, cancel := context.WithCancel(context.Background())
	return &CleanupTask{
		repo:       repo,
		interval:   interval,
		maxAge:     maxAge,
		ticker:     time.NewTicker(interval),
		cancelFunc: cancel,
		ctx:        ctx,
	}
}

// Start begins the cleanup task
func (t *CleanupTask) Start() {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.ticker.C:
				t.runCleanup()
			case <-t.ctx.Done():
				customLogger.Info("Cleanup task shutdown")
				return
			}
		}
	}()
	customLogger.Info("Cleanup task started", map[string]interface{}{
		"interval": t.interval.String(),
		"max_age":  t.maxAge.String(),
	})
}

// Stop gracefully stops the cleanup task
func (t *CleanupTask) Stop() {
	t.ticker.Stop()
	t.cancelFunc()
	t.wg.Wait()
}

// runCleanup performs the actual cleanup operation
func (t *CleanupTask) runCleanup() {
	// Create a context with timeout for the cleanup operation
	ctx, cancel := context.WithTimeout(t.ctx, 1*time.Minute)
	defer cancel()

	// Perform the cleanup
	deleted, err := t.repo.DeleteOldURLs(ctx, t.maxAge)
	if err != nil {
		customLogger.Error(err, "Failed to cleanup old URLs")
		return
	}

	// Log the result
	customLogger.Info("Cleanup task completed", map[string]interface{}{
		"deleted_count": deleted,
	})
}

// StartCleanupTask starts a new cleanup task (legacy wrapper)
func StartCleanupTask(repo repository.URLRepository, interval, maxAge time.Duration) *CleanupTask {
	task := NewCleanupTask(repo, interval, maxAge)
	task.Start()
	return task
}
