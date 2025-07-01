package utils

import (
	"bqs/internal/cache"
	"bqs/internal/config"
)

// NewCache creates a new cache with default configuration
func NewCache() (cache.Service, error) {
	return cache.New(config.DefaultCacheTTL)
}