package validation

import "testing"

func TestValidateProjectDatasetTable(t *testing.T) {
	validCases := []string{
		"my-project.dataset",
		"my-project.dataset.table",
		"my-project123.test_dataset.test_table",
	}
	
	invalidCases := []string{
		"",
		"single",
		"too.many.parts.here",
		"invalid-PROJECT.dataset",
		"project.123invalid",
		"project.-invalid",
	}
	
	for _, valid := range validCases {
		if err := ValidateProjectDatasetTable(valid); err != nil {
			t.Errorf("Expected %s to be valid, got error: %v", valid, err)
		}
	}
	
	for _, invalid := range invalidCases {
		if err := ValidateProjectDatasetTable(invalid); err == nil {
			t.Errorf("Expected %s to be invalid, but validation passed", invalid)
		}
	}
}

func TestValidateProject(t *testing.T) {
	validCases := []string{
		"my-project",
		"test123",
		"project-with-hyphens",
		"a12345678901234567890123456789", // 30 chars
	}
	
	invalidCases := []string{
		"",
		"short",                             // too short
		"a1234567890123456789012345678901", // too long (31 chars)
		"123project",                       // starts with number
		"Project",                          // uppercase
		"project-",                         // ends with hyphen
		"-project",                         // starts with hyphen
		"project_underscore",               // underscore not allowed
	}
	
	for _, valid := range validCases {
		if err := ValidateProject(valid); err != nil {
			t.Errorf("Expected %s to be valid project, got error: %v", valid, err)
		}
	}
	
	for _, invalid := range invalidCases {
		if err := ValidateProject(invalid); err == nil {
			t.Errorf("Expected %s to be invalid project, but validation passed", invalid)
		}
	}
}

func TestValidateDataset(t *testing.T) {
	validCases := []string{
		"dataset",
		"_dataset",
		"dataset123",
		"data_set_name",
		"Dataset_With_Mixed_Case",
	}
	
	invalidCases := []string{
		"",
		"123dataset", // starts with number
		"dataset-name", // hyphen not allowed
		"dataset.name", // dot not allowed
	}
	
	for _, valid := range validCases {
		if err := ValidateDataset(valid); err != nil {
			t.Errorf("Expected %s to be valid dataset, got error: %v", valid, err)
		}
	}
	
	for _, invalid := range invalidCases {
		if err := ValidateDataset(invalid); err == nil {
			t.Errorf("Expected %s to be invalid dataset, but validation passed", invalid)
		}
	}
}

func TestValidateTable(t *testing.T) {
	validCases := []string{
		"table",
		"_table",
		"table123",
		"table_name",
		"Table_With_Mixed_Case",
	}
	
	invalidCases := []string{
		"",
		"123table", // starts with number
		"table-name", // hyphen not allowed
		"table.name", // dot not allowed
	}
	
	for _, valid := range validCases {
		if err := ValidateTable(valid); err != nil {
			t.Errorf("Expected %s to be valid table, got error: %v", valid, err)
		}
	}
	
	for _, invalid := range invalidCases {
		if err := ValidateTable(invalid); err == nil {
			t.Errorf("Expected %s to be invalid table, but validation passed", invalid)
		}
	}
}