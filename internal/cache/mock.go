package cache

import (
	"sync"
	"time"
)

// MockService is a simple in-memory cache for testing
type MockService struct {
	mu    sync.RWMutex
	data  map[string]*CacheEntry
	stats CacheStats
}

// NewMockService creates a new mock cache service
func NewMockService() *MockService {
	return &MockService{
		data: make(map[string]*CacheEntry),
	}
}

func (m *MockService) Get(key string) (*CacheEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	entry, exists := m.data[key]
	if !exists || time.Now().After(entry.ExpiresAt) {
		return nil, ErrCacheMiss
	}
	return entry, nil
}

func (m *MockService) Set(key, data string, ttl *time.Duration, etag ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	cacheTTL := 15 * time.Minute
	if ttl != nil {
		cacheTTL = *ttl
	}
	
	var etagValue string
	if len(etag) > 0 {
		etagValue = etag[0]
	}
	
	now := time.Now()
	m.data[key] = &CacheEntry{
		Key:       key,
		Data:      data,
		CreatedAt: now,
		ExpiresAt: now.Add(cacheTTL),
		ETag:      etagValue,
	}
	
	m.stats.TotalEntries++
	return nil
}

func (m *MockService) Exists(key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	entry, exists := m.data[key]
	if !exists || time.Now().After(entry.ExpiresAt) {
		return false, nil
	}
	return true, nil
}

func (m *MockService) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MockService) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]*CacheEntry)
	m.stats = CacheStats{}
	return nil
}

func (m *MockService) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	for key, entry := range m.data {
		if now.After(entry.ExpiresAt) {
			delete(m.data, key)
		}
	}
	return nil
}

func (m *MockService) Stats() (*CacheStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	now := time.Now()
	valid := int64(0)
	expired := int64(0)
	
	for _, entry := range m.data {
		if now.After(entry.ExpiresAt) {
			expired++
		} else {
			valid++
		}
	}
	
	return &CacheStats{
		TotalEntries:   int64(len(m.data)),
		ValidEntries:   valid,
		ExpiredEntries: expired,
		SizeBytes:      0, // Not tracked in mock
	}, nil
}

func (m *MockService) Close() error {
	return nil
}