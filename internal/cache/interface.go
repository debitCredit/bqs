package cache

import "time"

// Service defines the interface for cache operations
type Service interface {
	Get(key string) (*CacheEntry, error)
	Set(key, data string, ttl *time.Duration, etag ...string) error
	Exists(key string) (bool, error)
	Delete(key string) error
	Clear() error
	Cleanup() error
	Stats() (*CacheStats, error)
	Close() error
}

// Ensure Cache implements Service
var _ Service = (*Cache)(nil)