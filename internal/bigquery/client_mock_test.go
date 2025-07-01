package bigquery

import (
	"testing"

	"bqs/internal/cache"
)

func TestClientWithMockCache(t *testing.T) {
	// Create mock cache for testing
	mockCache := cache.NewMockService()
	
	// Create BigQuery client with mock
	client := NewClient(mockCache)
	
	// Test IsTableMetadataCached with empty cache
	cached := client.IsTableMetadataCached("test-project", "test-dataset", "test-table")
	if cached {
		t.Error("Expected empty cache to return false, got true")
	}
	
	// Test cache key generation
	tableListKey := cache.TableListKey("project", "dataset")
	expectedKey := "tables:project.dataset"
	if tableListKey != expectedKey {
		t.Errorf("Expected table list key %s, got %s", expectedKey, tableListKey)
	}
	
	schemaKey := cache.SchemaKey("project", "dataset", "table")
	expectedKey = "schema:project.dataset.table"
	if schemaKey != expectedKey {
		t.Errorf("Expected schema key %s, got %s", expectedKey, schemaKey)
	}
	
	metadataKey := cache.MetadataKey("project", "dataset", "table")
	expectedKey = "metadata:project.dataset.table"
	if metadataKey != expectedKey {
		t.Errorf("Expected metadata key %s, got %s", expectedKey, metadataKey)
	}
}

func TestFormatFunctions(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	
	for _, test := range tests {
		result := FormatSize(test.bytes)
		if result != test.expected {
			t.Errorf("FormatSize(%d) = %s, expected %s", test.bytes, result, test.expected)
		}
	}
}

func TestGetTableTypeIcon(t *testing.T) {
	tests := []struct {
		tableType string
		expected  string
	}{
		{"TABLE", "📋"},
		{"VIEW", "👁️"},
		{"MATERIALIZED_VIEW", "💎"},
		{"UNKNOWN", "❓"},
		{"", "❓"},
	}
	
	for _, test := range tests {
		result := GetTableTypeIcon(test.tableType)
		if result != test.expected {
			t.Errorf("GetTableTypeIcon(%s) = %s, expected %s", test.tableType, result, test.expected)
		}
	}
}