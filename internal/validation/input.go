package validation

import (
	"fmt"
	"regexp"
	"strings"
)

// BigQuery identifier patterns
var (
	projectPattern = regexp.MustCompile(`^[a-z][a-z0-9\-]*[a-z0-9]$`)
	datasetPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	tablePattern   = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// ValidateProjectDatasetTable validates a project.dataset.table identifier
func ValidateProjectDatasetTable(input string) error {
	parts := strings.Split(input, ".")
	if len(parts) < 2 {
		return fmt.Errorf("invalid format: expected project.dataset or project.dataset.table, got %s", input)
	}
	
	if len(parts) > 3 {
		return fmt.Errorf("invalid format: too many parts in %s", input)
	}
	
	// Validate project
	if err := ValidateProject(parts[0]); err != nil {
		return fmt.Errorf("invalid project: %w", err)
	}
	
	// Validate dataset
	if err := ValidateDataset(parts[1]); err != nil {
		return fmt.Errorf("invalid dataset: %w", err)
	}
	
	// Validate table if present
	if len(parts) > 2 {
		if err := ValidateTable(parts[2]); err != nil {
			return fmt.Errorf("invalid table: %w", err)
		}
	}
	
	return nil
}

// ValidateProject validates a BigQuery project ID
func ValidateProject(project string) error {
	if project == "" {
		return fmt.Errorf("project cannot be empty")
	}
	
	if len(project) < 6 || len(project) > 30 {
		return fmt.Errorf("project length must be 6-30 characters, got %d", len(project))
	}
	
	if !projectPattern.MatchString(project) {
		return fmt.Errorf("project must start with lowercase letter, contain only lowercase letters, numbers, and hyphens, and end with letter or number")
	}
	
	return nil
}

// ValidateDataset validates a BigQuery dataset ID
func ValidateDataset(dataset string) error {
	if dataset == "" {
		return fmt.Errorf("dataset cannot be empty")
	}
	
	if len(dataset) > 1024 {
		return fmt.Errorf("dataset length cannot exceed 1024 characters, got %d", len(dataset))
	}
	
	if !datasetPattern.MatchString(dataset) {
		return fmt.Errorf("dataset must start with letter or underscore, contain only letters, numbers, and underscores")
	}
	
	return nil
}

// ValidateTable validates a BigQuery table ID
func ValidateTable(table string) error {
	if table == "" {
		return fmt.Errorf("table cannot be empty")
	}
	
	if len(table) > 1024 {
		return fmt.Errorf("table length cannot exceed 1024 characters, got %d", len(table))
	}
	
	if !tablePattern.MatchString(table) {
		return fmt.Errorf("table must start with letter or underscore, contain only letters, numbers, and underscores")
	}
	
	return nil
}