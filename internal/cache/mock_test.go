package cache

import (
	"testing"
	"time"
)

func TestMockService(t *testing.T) {
	mock := NewMockService()
	defer mock.Close()
	
	// Test empty cache
	_, err := mock.Get("nonexistent")
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss for nonexistent key, got %v", err)
	}
	
	exists, err := mock.Exists("nonexistent")
	if err != nil {
		t.Errorf("Exists returned error: %v", err)
	}
	if exists {
		t.Error("Expected false for nonexistent key")
	}
	
	// Test setting and getting data
	testKey := "test:key"
	testData := "test data"
	ttl := 5 * time.Minute
	
	err = mock.Set(testKey, testData, &ttl)
	if err != nil {
		t.Errorf("Set returned error: %v", err)
	}
	
	// Test exists after setting
	exists, err = mock.Exists(testKey)
	if err != nil {
		t.Errorf("Exists returned error: %v", err)
	}
	if !exists {
		t.Error("Expected true for existing key")
	}
	
	// Test getting data
	entry, err := mock.Get(testKey)
	if err != nil {
		t.Errorf("Get returned error: %v", err)
	}
	if entry.Data != testData {
		t.Errorf("Expected data %s, got %s", testData, entry.Data)
	}
	if entry.Key != testKey {
		t.Errorf("Expected key %s, got %s", testKey, entry.Key)
	}
	
	// Test stats
	stats, err := mock.Stats()
	if err != nil {
		t.Errorf("Stats returned error: %v", err)
	}
	if stats.TotalEntries != 1 {
		t.Errorf("Expected 1 total entry, got %d", stats.TotalEntries)
	}
	if stats.ValidEntries != 1 {
		t.Errorf("Expected 1 valid entry, got %d", stats.ValidEntries)
	}
	
	// Test delete
	err = mock.Delete(testKey)
	if err != nil {
		t.Errorf("Delete returned error: %v", err)
	}
	
	exists, err = mock.Exists(testKey)
	if err != nil {
		t.Errorf("Exists returned error: %v", err)
	}
	if exists {
		t.Error("Expected false after delete")
	}
	
	// Test clear
	mock.Set("key1", "data1", nil)
	mock.Set("key2", "data2", nil)
	
	err = mock.Clear()
	if err != nil {
		t.Errorf("Clear returned error: %v", err)
	}
	
	stats, err = mock.Stats()
	if err != nil {
		t.Errorf("Stats returned error: %v", err)
	}
	if stats.TotalEntries != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", stats.TotalEntries)
	}
}

func TestMockServiceExpiration(t *testing.T) {
	mock := NewMockService()
	defer mock.Close()
	
	// Set item with very short TTL
	shortTTL := 1 * time.Millisecond
	err := mock.Set("expire-test", "data", &shortTTL)
	if err != nil {
		t.Errorf("Set returned error: %v", err)
	}
	
	// Wait for expiration
	time.Sleep(2 * time.Millisecond)
	
	// Should not exist anymore
	exists, err := mock.Exists("expire-test")
	if err != nil {
		t.Errorf("Exists returned error: %v", err)
	}
	if exists {
		t.Error("Expected false for expired key")
	}
	
	// Get should return cache miss
	_, err = mock.Get("expire-test")
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss for expired key, got %v", err)
	}
}