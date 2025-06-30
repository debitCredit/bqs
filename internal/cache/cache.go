package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Cache handles BigQuery metadata caching with SQLite
type Cache struct {
	db         *sql.DB
	defaultTTL time.Duration
}

// CacheEntry represents a cached metadata entry
type CacheEntry struct {
	Key       string    `json:"key"`
	Data      string    `json:"data"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	ETag      string    `json:"etag,omitempty"`
}

// New creates a new cache instance with SQLite backend
func New(defaultTTL time.Duration) (*Cache, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache directory: %w", err)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	dbPath := filepath.Join(cacheDir, "metadata.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cache database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	cache := &Cache{
		db:         db,
		defaultTTL: defaultTTL,
	}

	if err := cache.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize cache schema: %w", err)
	}

	return cache, nil
}

// Close closes the cache database connection
func (c *Cache) Close() error {
	return c.db.Close()
}

// Get retrieves cached metadata by key
func (c *Cache) Get(key string) (*CacheEntry, error) {
	var entry CacheEntry
	var createdAtUnix, expiresAtUnix int64

	query := `
		SELECT key, data, created_at, expires_at, COALESCE(etag, '') 
		FROM metadata_cache 
		WHERE key = ? AND expires_at > ?
	`

	err := c.db.QueryRow(query, key, time.Now().Unix()).Scan(
		&entry.Key,
		&entry.Data,
		&createdAtUnix,
		&expiresAtUnix,
		&entry.ETag,
	)

	if err == sql.ErrNoRows {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cache entry: %w", err)
	}

	entry.CreatedAt = time.Unix(createdAtUnix, 0)
	entry.ExpiresAt = time.Unix(expiresAtUnix, 0)

	return &entry, nil
}

// Set stores metadata in cache with optional TTL override
func (c *Cache) Set(key, data string, ttl *time.Duration, etag ...string) error {
	cacheTTL := c.defaultTTL
	if ttl != nil {
		cacheTTL = *ttl
	}

	now := time.Now()
	expiresAt := now.Add(cacheTTL)

	var etagValue string
	if len(etag) > 0 {
		etagValue = etag[0]
	}

	query := `
		INSERT OR REPLACE INTO metadata_cache 
		(key, data, created_at, expires_at, etag) 
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := c.db.Exec(query, key, data, now.Unix(), expiresAt.Unix(), etagValue)
	if err != nil {
		return fmt.Errorf("failed to set cache entry: %w", err)
	}

	return nil
}

// Delete removes a cache entry
func (c *Cache) Delete(key string) error {
	_, err := c.db.Exec("DELETE FROM metadata_cache WHERE key = ?", key)
	return err
}

// Clear removes all cache entries
func (c *Cache) Clear() error {
	_, err := c.db.Exec("DELETE FROM metadata_cache")
	return err
}

// Cleanup removes expired entries
func (c *Cache) Cleanup() error {
	query := "DELETE FROM metadata_cache WHERE expires_at <= ?"
	result, err := c.db.Exec(query, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to cleanup cache: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		// Vacuum to reclaim space after cleanup
		_, err = c.db.Exec("VACUUM")
	}

	return err
}

// Stats returns cache statistics
func (c *Cache) Stats() (*CacheStats, error) {
	var stats CacheStats

	// Total entries
	err := c.db.QueryRow("SELECT COUNT(*) FROM metadata_cache").Scan(&stats.TotalEntries)
	if err != nil {
		return nil, err
	}

	// Expired entries
	err = c.db.QueryRow("SELECT COUNT(*) FROM metadata_cache WHERE expires_at <= ?", time.Now().Unix()).Scan(&stats.ExpiredEntries)
	if err != nil {
		return nil, err
	}

	// Database size
	var pageCount, pageSize int64
	err = c.db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	if err != nil {
		return nil, err
	}
	err = c.db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	if err != nil {
		return nil, err
	}

	stats.SizeBytes = pageCount * pageSize
	stats.ValidEntries = stats.TotalEntries - stats.ExpiredEntries

	return &stats, nil
}

// initSchema creates the cache table if it doesn't exist
func (c *Cache) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS metadata_cache (
			key TEXT PRIMARY KEY,
			data TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			etag TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_expires_at ON metadata_cache(expires_at);
		CREATE INDEX IF NOT EXISTS idx_created_at ON metadata_cache(created_at);
	`

	_, err := c.db.Exec(schema)
	return err
}

// getCacheDir returns the cache directory following XDG standards
func getCacheDir() (string, error) {
	// Check environment variable first
	if cacheDir := os.Getenv("BQS_CACHE_DIR"); cacheDir != "" {
		return cacheDir, nil
	}

	// Use XDG_CACHE_HOME if set
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, "bqs"), nil
	}

	// Fallback to ~/.cache/bqs
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".cache", "bqs"), nil
}

// CacheStats represents cache statistics
type CacheStats struct {
	TotalEntries   int64 `json:"total_entries"`
	ValidEntries   int64 `json:"valid_entries"`
	ExpiredEntries int64 `json:"expired_entries"`
	SizeBytes      int64 `json:"size_bytes"`
}

// Common cache errors
var (
	ErrCacheMiss = fmt.Errorf("cache miss")
)

// Helper functions for common cache keys
func TableListKey(project, dataset string) string {
	return fmt.Sprintf("tables:%s.%s", project, dataset)
}

func SchemaKey(project, dataset, table string) string {
	return fmt.Sprintf("schema:%s.%s.%s", project, dataset, table)
}

func MetadataKey(project, dataset, table string) string {
	return fmt.Sprintf("metadata:%s.%s.%s", project, dataset, table)
}

// Exists checks if a key exists in the cache (without retrieving the data)
func (c *Cache) Exists(key string) (bool, error) {
	query := `SELECT 1 FROM metadata_cache WHERE key = ? AND expires_at > ?`
	var exists int
	err := c.db.QueryRow(query, key, time.Now().Unix()).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}