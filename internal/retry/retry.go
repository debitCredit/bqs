package retry

import (
	"context"
	"fmt"
	"time"

	"bqs/internal/errors"
)

// Config holds retry configuration
type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultConfig returns sensible retry defaults for BigQuery operations
func DefaultConfig() *Config {
	return &Config{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
	}
}

// QuickConfig returns faster retry settings for interactive operations
func QuickConfig() *Config {
	return &Config{
		MaxAttempts: 2,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
	}
}

// WithRetry executes a function with exponential backoff retry logic
func WithRetry(ctx context.Context, config *Config, operation string, fn func() error) error {
	if config == nil {
		config = DefaultConfig()
	}

	var lastErr error
	
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Execute the operation
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if this is a BQS error and if it's retryable
		if bqsErr, ok := err.(*errors.BQSError); ok {
			if !bqsErr.IsRetryable() {
				return bqsErr // Don't retry non-retryable errors
			}
			
			// Use error-specific retry delay if available
			if retryAfter := bqsErr.GetRetryAfter(); retryAfter > 0 {
				if attempt < config.MaxAttempts {
					select {
					case <-time.After(retryAfter):
						continue
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				continue
			}
		}

		// Don't sleep after the last attempt
		if attempt >= config.MaxAttempts {
			break
		}

		// Calculate exponential backoff delay
		delay := time.Duration(float64(config.BaseDelay) * pow(config.Multiplier, float64(attempt-1)))
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}

		// Wait before retrying, respecting context cancellation
		select {
		case <-time.After(delay):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// All attempts failed, return the last error with retry context
	if bqsErr, ok := lastErr.(*errors.BQSError); ok {
		bqsErr.Message = fmt.Sprintf("%s (failed after %d attempts)", bqsErr.Message, config.MaxAttempts)
		return bqsErr
	}

	return fmt.Errorf("%s failed after %d attempts: %w", operation, config.MaxAttempts, lastErr)
}

// WithQuickRetry is a convenience function for interactive operations that need fast retry
func WithQuickRetry(ctx context.Context, operation string, fn func() error) error {
	return WithRetry(ctx, QuickConfig(), operation, fn)
}

// WithDefaultRetry is a convenience function using default retry settings
func WithDefaultRetry(ctx context.Context, operation string, fn func() error) error {
	return WithRetry(ctx, DefaultConfig(), operation, fn)
}

// pow is a simple integer power function for exponential backoff
func pow(base float64, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// RetryableOperation wraps an operation with automatic retry logic and user feedback
type RetryableOperation struct {
	Name         string
	Config       *Config
	StatusUpdate func(attempt int, err error) // Optional callback for UI updates
}

// Execute runs the operation with retry logic and optional status updates
func (ro *RetryableOperation) Execute(ctx context.Context, fn func() error) error {
	config := ro.Config
	if config == nil {
		config = DefaultConfig()
	}

	var lastErr error
	
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Notify about retry attempt if callback provided
		if ro.StatusUpdate != nil && attempt > 1 {
			ro.StatusUpdate(attempt, lastErr)
		}

		// Execute the operation
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if this is a BQS error and if it's retryable
		if bqsErr, ok := err.(*errors.BQSError); ok {
			if !bqsErr.IsRetryable() {
				return bqsErr // Don't retry non-retryable errors
			}
		}

		// Don't sleep after the last attempt
		if attempt >= config.MaxAttempts {
			break
		}

		// Calculate delay and wait
		delay := time.Duration(float64(config.BaseDelay) * pow(config.Multiplier, float64(attempt-1)))
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}

		select {
		case <-time.After(delay):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// All attempts failed
	if bqsErr, ok := lastErr.(*errors.BQSError); ok {
		bqsErr.Message = fmt.Sprintf("%s (failed after %d attempts)", bqsErr.Message, config.MaxAttempts)
		return bqsErr
	}

	return fmt.Errorf("%s failed after %d attempts: %w", ro.Name, config.MaxAttempts, lastErr)
}