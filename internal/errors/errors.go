package errors

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ErrorType represents different categories of errors
type ErrorType int

const (
	ErrorTypeNetwork ErrorType = iota
	ErrorTypeAuth
	ErrorTypePermission
	ErrorTypeNotFound
	ErrorTypeQuota
	ErrorTypeAPI
	ErrorTypeCache
	ErrorTypeValidation
	ErrorTypeUnknown
)

// BQSError represents a structured error with context and retry information
type BQSError struct {
	Type        ErrorType
	Message     string
	Underlying  error
	Retryable   bool
	RetryAfter  time.Duration
	Context     map[string]string
}

// Error implements the error interface
func (e *BQSError) Error() string {
	if e.Context != nil && len(e.Context) > 0 {
		var parts []string
		for k, v := range e.Context {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		return fmt.Sprintf("%s (%s)", e.Message, strings.Join(parts, ", "))
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *BQSError) Unwrap() error {
	return e.Underlying
}

// IsRetryable returns whether this error can be retried
func (e *BQSError) IsRetryable() bool {
	return e.Retryable
}

// GetRetryAfter returns the duration to wait before retrying
func (e *BQSError) GetRetryAfter() time.Duration {
	if e.RetryAfter > 0 {
		return e.RetryAfter
	}
	// Default retry backoff based on error type
	switch e.Type {
	case ErrorTypeNetwork:
		return 2 * time.Second
	case ErrorTypeQuota:
		return 30 * time.Second
	case ErrorTypeAPI:
		return 5 * time.Second
	default:
		return 1 * time.Second
	}
}

// WrapBigQueryError wraps a BigQuery command error with context and classification
func WrapBigQueryError(err error, operation, project, dataset, table string) *BQSError {
	if err == nil {
		return nil
	}

	context := map[string]string{
		"operation": operation,
		"project":   project,
		"dataset":   dataset,
	}
	if table != "" {
		context["table"] = table
	}

	// Analyze the error to determine type and message
	errorText := err.Error()
	lowerError := strings.ToLower(errorText)

	// Check for specific BigQuery error patterns
	switch {
	case strings.Contains(lowerError, "not found"):
		return &BQSError{
			Type:       ErrorTypeNotFound,
			Message:    determineNotFoundMessage(operation, project, dataset, table),
			Underlying: err,
			Retryable:  false,
			Context:    context,
		}

	case strings.Contains(lowerError, "permission denied") || strings.Contains(lowerError, "access denied"):
		return &BQSError{
			Type:       ErrorTypePermission,
			Message:    fmt.Sprintf("Access denied to %s.%s - check BigQuery permissions", project, dataset),
			Underlying: err,
			Retryable:  false,
			Context:    context,
		}

	case strings.Contains(lowerError, "authentication") || strings.Contains(lowerError, "credentials"):
		return &BQSError{
			Type:       ErrorTypeAuth,
			Message:    "Authentication failed - run 'gcloud auth login' or check service account credentials",
			Underlying: err,
			Retryable:  false,
			Context:    context,
		}

	case strings.Contains(lowerError, "quota") || strings.Contains(lowerError, "rate limit"):
		return &BQSError{
			Type:       ErrorTypeQuota,
			Message:    "BigQuery quota exceeded - retrying with backoff",
			Underlying: err,
			Retryable:  true,
			RetryAfter: 30 * time.Second,
			Context:    context,
		}

	case strings.Contains(lowerError, "timeout") || strings.Contains(lowerError, "deadline"):
		return &BQSError{
			Type:       ErrorTypeNetwork,
			Message:    "BigQuery request timed out - retrying",
			Underlying: err,
			Retryable:  true,
			RetryAfter: 5 * time.Second,
			Context:    context,
		}

	case strings.Contains(lowerError, "connection") || strings.Contains(lowerError, "network"):
		return &BQSError{
			Type:       ErrorTypeNetwork,
			Message:    "Network error connecting to BigQuery - retrying",
			Underlying: err,
			Retryable:  true,
			RetryAfter: 2 * time.Second,
			Context:    context,
		}

	case isExitError(err):
		// Handle bq command exit errors
		exitErr := err.(*exec.ExitError)
		stderr := string(exitErr.Stderr)
		
		if strings.Contains(strings.ToLower(stderr), "not found") {
			return &BQSError{
				Type:       ErrorTypeNotFound,
				Message:    determineNotFoundMessage(operation, project, dataset, table),
				Underlying: err,
				Retryable:  false,
				Context:    context,
			}
		}
		
		return &BQSError{
			Type:       ErrorTypeAPI,
			Message:    fmt.Sprintf("BigQuery command failed: %s", cleanErrorOutput(stderr)),
			Underlying: err,
			Retryable:  true,
			Context:    context,
		}

	default:
		return &BQSError{
			Type:       ErrorTypeUnknown,
			Message:    fmt.Sprintf("BigQuery operation failed: %s", cleanErrorOutput(errorText)),
			Underlying: err,
			Retryable:  true,
			Context:    context,
		}
	}
}

// WrapCacheError wraps cache-related errors
func WrapCacheError(err error, operation string) *BQSError {
	if err == nil {
		return nil
	}

	return &BQSError{
		Type:       ErrorTypeCache,
		Message:    fmt.Sprintf("Cache %s failed: %s", operation, err.Error()),
		Underlying: err,
		Retryable:  false,
		Context:    map[string]string{"operation": operation},
	}
}

// WrapValidationError wraps validation errors
func WrapValidationError(err error, input string) *BQSError {
	if err == nil {
		return nil
	}

	return &BQSError{
		Type:       ErrorTypeValidation,
		Message:    fmt.Sprintf("Invalid input '%s': %s", input, err.Error()),
		Underlying: err,
		Retryable:  false,
		Context:    map[string]string{"input": input},
	}
}

// determineNotFoundMessage creates specific not found messages
func determineNotFoundMessage(operation, project, dataset, table string) string {
	switch operation {
	case "list_tables":
		return fmt.Sprintf("Dataset %s.%s not found or empty", project, dataset)
	case "get_metadata", "get_schema":
		if table != "" {
			return fmt.Sprintf("Table %s.%s.%s not found", project, dataset, table)
		}
		return fmt.Sprintf("Dataset %s.%s not found", project, dataset)
	default:
		if table != "" {
			return fmt.Sprintf("Table %s.%s.%s not found", project, dataset, table)
		}
		return fmt.Sprintf("Dataset %s.%s not found", project, dataset)
	}
}

// cleanErrorOutput cleans up error messages for better user experience
func cleanErrorOutput(errorText string) string {
	// Remove common bq command noise
	cleaned := strings.TrimSpace(errorText)
	
	// Remove "ERROR: " prefix if present
	if strings.HasPrefix(cleaned, "ERROR: ") {
		cleaned = cleaned[7:]
	}
	
	// Remove terminal escape codes if any
	// This is a simple cleanup - could be enhanced with regex
	lines := strings.Split(cleaned, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "WARNING") {
			cleanLines = append(cleanLines, line)
		}
	}
	
	if len(cleanLines) > 0 {
		return cleanLines[0] // Return first meaningful line
	}
	
	return cleaned
}

// isExitError checks if an error is an exec.ExitError
func isExitError(err error) bool {
	_, ok := err.(*exec.ExitError)
	return ok
}

// UserFriendlyMessage returns a user-friendly error message
func (e *BQSError) UserFriendlyMessage() string {
	switch e.Type {
	case ErrorTypeNotFound:
		return e.Message + " - verify the project, dataset, and table names"
	case ErrorTypeAuth:
		return e.Message
	case ErrorTypePermission:
		return e.Message + " - contact your BigQuery administrator"
	case ErrorTypeQuota:
		return e.Message + " - try again in a few moments"
	case ErrorTypeNetwork:
		return e.Message + " - check your internet connection"
	case ErrorTypeValidation:
		return e.Message + " - use format: project.dataset[.table]"
	default:
		return e.Message
	}
}