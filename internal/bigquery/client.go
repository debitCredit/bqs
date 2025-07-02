package bigquery

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"bqs/internal/cache"
	"bqs/internal/config"
	"bqs/internal/errors"
	"bqs/internal/retry"
	"bqs/internal/utils"
)

// Client wraps BigQuery operations with caching
type Client struct {
	cache cache.Service
}

// NewClient creates a new BigQuery client with caching
func NewClient(c cache.Service) *Client {
	return &Client{
		cache: c,
	}
}

// IsTableMetadataCached checks if table metadata is available in cache
func (c *Client) IsTableMetadataCached(project, dataset, table string) bool {
	key := cache.MetadataKey(project, dataset, table)
	exists, err := c.cache.Exists(key)
	if err != nil {
		return false
	}
	return exists
}

// TableInfo represents BigQuery table metadata
type TableInfo struct {
	TableID          string         `json:"tableId"`
	TableReference   TableReference `json:"tableReference"`
	Type             string         `json:"type"` // TABLE, VIEW, MATERIALIZED_VIEW
	CreationTime     int64          `json:"creationTime,string"`
	LastModifiedTime int64          `json:"lastModifiedTime,string"`
	NumRows          int64          `json:"numRows,string,omitempty"`
	NumBytes         int64          `json:"numBytes,string,omitempty"`
	Location         string         `json:"location,omitempty"`
	FriendlyName     string         `json:"friendlyName,omitempty"`
	Description      string         `json:"description,omitempty"`
}

// TableReference represents BigQuery table reference
type TableReference struct {
	ProjectID string `json:"projectId"`
	DatasetID string `json:"datasetId"`
	TableID   string `json:"tableId"`
}

// Schema represents BigQuery table schema
type Schema struct {
	Fields []SchemaField `json:"fields"`
}

// SchemaField represents a BigQuery schema field
type SchemaField struct {
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	Mode        string        `json:"mode,omitempty"` // REQUIRED, NULLABLE, REPEATED
	Description string        `json:"description,omitempty"`
	Fields      []SchemaField `json:"fields,omitempty"` // For nested/repeated fields
}

// TableMetadata represents complete table metadata
type TableMetadata struct {
	TableInfo
	Schema *Schema `json:"schema,omitempty"`
}

// ListTables retrieves tables in a dataset with caching and retry logic
func (c *Client) ListTables(project, dataset string) ([]TableInfo, error) {
	cacheKey := cache.TableListKey(project, dataset)

	// Try cache first
	if entry, err := c.cache.Get(cacheKey); err == nil {
		var tables []TableInfo
		if err := json.Unmarshal([]byte(entry.Data), &tables); err == nil {
			return tables, nil
		}
	}

	// Cache miss or invalid data, fetch from BigQuery with retry
	var tables []TableInfo
	ctx := context.Background()
	err := retry.WithQuickRetry(ctx, "list tables", func() error {
		var fetchErr error
		tables, fetchErr = c.fetchTableList(project, dataset)
		if fetchErr != nil {
			return errors.WrapBigQueryError(fetchErr, "list_tables", project, dataset, "")
		}
		return nil
	})
	
	if err != nil {
		return nil, err
	}

	// Cache the result
	data, err := json.Marshal(tables)
	if err != nil {
		// Log cache error but don't fail - continue without caching
		if cacheErr := errors.WrapCacheError(err, "marshal table list"); cacheErr != nil {
			fmt.Printf("Warning: %s\n", cacheErr.UserFriendlyMessage())
		}
		return tables, nil
	}
	ttl := config.TableListTTL
	if err := c.cache.Set(cacheKey, string(data), &ttl); err != nil {
		// Log cache error but don't fail - continue without caching
		if cacheErr := errors.WrapCacheError(err, "set table list cache"); cacheErr != nil {
			fmt.Printf("Warning: %s\n", cacheErr.UserFriendlyMessage())
		}
	}

	return tables, nil
}

// GetSchema retrieves table schema with caching and retry logic
func (c *Client) GetSchema(project, dataset, table string) (*Schema, error) {
	cacheKey := cache.SchemaKey(project, dataset, table)

	// Try cache first
	if entry, err := c.cache.Get(cacheKey); err == nil {
		var schema Schema
		if err := json.Unmarshal([]byte(entry.Data), &schema); err == nil {
			return &schema, nil
		}
	}

	// Cache miss, fetch from BigQuery with retry
	var schema *Schema
	ctx := context.Background()
	err := retry.WithDefaultRetry(ctx, "get schema", func() error {
		var fetchErr error
		schema, fetchErr = c.fetchSchema(project, dataset, table)
		if fetchErr != nil {
			return errors.WrapBigQueryError(fetchErr, "get_schema", project, dataset, table)
		}
		return nil
	})
	
	if err != nil {
		return nil, err
	}

	// Cache the result
	data, err := json.Marshal(schema)
	if err != nil {
		// Log cache error but don't fail - continue without caching
		if cacheErr := errors.WrapCacheError(err, "marshal schema"); cacheErr != nil {
			fmt.Printf("Warning: %s\n", cacheErr.UserFriendlyMessage())
		}
		return schema, nil
	}
	ttl := config.SchemaTTL
	if err := c.cache.Set(cacheKey, string(data), &ttl); err != nil {
		// Log cache error but don't fail - continue without caching
		if cacheErr := errors.WrapCacheError(err, "set schema cache"); cacheErr != nil {
			fmt.Printf("Warning: %s\n", cacheErr.UserFriendlyMessage())
		}
	}

	return schema, nil
}

// GetTableMetadata retrieves complete table metadata with caching and retry logic
func (c *Client) GetTableMetadata(project, dataset, table string) (*TableMetadata, error) {
	cacheKey := cache.MetadataKey(project, dataset, table)

	// Try cache first
	if entry, err := c.cache.Get(cacheKey); err == nil {
		var metadata TableMetadata
		if err := json.Unmarshal([]byte(entry.Data), &metadata); err == nil {
			return &metadata, nil
		}
	}

	// Cache miss, fetch from BigQuery with retry
	var metadata *TableMetadata
	ctx := context.Background()
	err := retry.WithDefaultRetry(ctx, "get table metadata", func() error {
		var fetchErr error
		metadata, fetchErr = c.fetchTableMetadata(project, dataset, table)
		if fetchErr != nil {
			return errors.WrapBigQueryError(fetchErr, "get_metadata", project, dataset, table)
		}
		return nil
	})
	
	if err != nil {
		return nil, err
	}

	// Cache the result
	data, err := json.Marshal(metadata)
	if err != nil {
		// Log cache error but don't fail - continue without caching
		if cacheErr := errors.WrapCacheError(err, "marshal metadata"); cacheErr != nil {
			fmt.Printf("Warning: %s\n", cacheErr.UserFriendlyMessage())
		}
		return metadata, nil
	}
	ttl := config.MetadataTTL
	if err := c.cache.Set(cacheKey, string(data), &ttl); err != nil {
		// Log cache error but don't fail - continue without caching
		if cacheErr := errors.WrapCacheError(err, "set metadata cache"); cacheErr != nil {
			fmt.Printf("Warning: %s\n", cacheErr.UserFriendlyMessage())
		}
	}

	return metadata, nil
}

// fetchTableList calls bq ls to get table list
func (c *Client) fetchTableList(project, dataset string) ([]TableInfo, error) {
	cmd := exec.Command("bq", "ls", "--project_id="+project, "--format=json", "--max_results=1000", dataset)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	var tables []TableInfo
	if err := json.Unmarshal(output, &tables); err != nil {
		return nil, fmt.Errorf("failed to parse table list: %w", err)
	}

	// Fix table IDs - use tableReference.tableId if tableId is empty
	for i := range tables {
		if tables[i].TableID == "" && tables[i].TableReference.TableID != "" {
			tables[i].TableID = tables[i].TableReference.TableID
		}
	}

	return tables, nil
}

// fetchSchema calls bq show --schema to get table schema
func (c *Client) fetchSchema(project, dataset, table string) (*Schema, error) {
	tableID := dataset + "." + table
	cmd := exec.Command("bq", "show", "--project_id="+project, "--schema", "--format=json", tableID)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	var fields []SchemaField
	if err := json.Unmarshal(output, &fields); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	return &Schema{Fields: fields}, nil
}

// fetchTableMetadata calls bq show to get complete table metadata
func (c *Client) fetchTableMetadata(project, dataset, table string) (*TableMetadata, error) {
	tableID := dataset + "." + table
	cmd := exec.Command("bq", "show", "--project_id="+project, "--format=json", tableID)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get table metadata: %w", err)
	}

	var metadata TableMetadata
	if err := json.Unmarshal(output, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse table metadata: %w", err)
	}

	return &metadata, nil
}

// InvalidateCache removes cached data for a specific table or dataset
func (c *Client) InvalidateCache(project, dataset, table string) error {
	var keys []string

	if table != "" {
		// Invalidate specific table
		keys = append(keys,
			cache.SchemaKey(project, dataset, table),
			cache.MetadataKey(project, dataset, table),
		)
	}

	if dataset != "" {
		// Invalidate dataset table list
		keys = append(keys, cache.TableListKey(project, dataset))
	}

	for _, key := range keys {
		if err := c.cache.Delete(key); err != nil {
			return fmt.Errorf("failed to invalidate cache key %s: %w", key, err)
		}
	}

	return nil
}

// FormatSize formats bytes in human readable format
func FormatSize(bytes int64) string {
	return utils.FormatBytes(bytes)
}

// FormatTime formats Unix timestamp to readable format
func FormatTime(unixMillis int64) string {
	if unixMillis == 0 {
		return "N/A"
	}
	t := time.Unix(unixMillis/1000, 0)
	return t.Format("Jan 2 15:04")
}

// GetTableTypeIcon returns an icon for the table type
func GetTableTypeIcon(tableType string) string {
	switch strings.ToUpper(tableType) {
	case "TABLE":
		return "📋"
	case "VIEW":
		return "👁️"
	case "MATERIALIZED_VIEW":
		return "💎"
	default:
		return "❓"
	}
}