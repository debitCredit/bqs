package bigquery

import (
	"testing"

	"bqs/internal/cache"
	"bqs/internal/utils"
)

func TestClient(t *testing.T) {
	// Create a cache for testing
	c, err := utils.NewCache()
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Create BigQuery client
	_ = NewClient(c)

	// Test cache key functions
	tableListKey := cache.TableListKey("project", "dataset")
	expected := "tables:project.dataset"
	if tableListKey != expected {
		t.Errorf("Expected table list key %s, got %s", expected, tableListKey)
	}

	schemaKey := cache.SchemaKey("project", "dataset", "table")
	expected = "schema:project.dataset.table"
	if schemaKey != expected {
		t.Errorf("Expected schema key %s, got %s", expected, schemaKey)
	}

	// Test format functions
	sizeStr := FormatSize(1024)
	if sizeStr != "1.0 KB" {
		t.Errorf("Expected '1.0 KB', got '%s'", sizeStr)
	}

	sizeStr = FormatSize(1048576)
	if sizeStr != "1.0 MB" {
		t.Errorf("Expected '1.0 MB', got '%s'", sizeStr)
	}

	// Test table type icons
	icon := GetTableTypeIcon("TABLE")
	if icon != "📋" {
		t.Errorf("Expected table icon, got %s", icon)
	}

	icon = GetTableTypeIcon("VIEW")
	if icon != "👁️" {
		t.Errorf("Expected view icon, got %s", icon)
	}

	t.Logf("Client tests passed successfully")
}