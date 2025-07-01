package cmd

import (
	"strings"
	"testing"
	tea "github.com/charmbracelet/bubbletea"
	"bqs/internal/bigquery"
)

func TestSearchStateBasics(t *testing.T) {
	search := SearchState{}
	
	// Test initial state
	if !search.IsEmpty() {
		t.Error("New SearchState should be empty")
	}
	
	if search.ResultCount() != 0 {
		t.Error("New SearchState should have 0 results")
	}
	
	if search.HasResults() {
		t.Error("New SearchState should have no results")
	}
	
	// Test active state
	search.Active = true
	search.Query = "test"
	search.Context = SearchTables
	
	if search.IsEmpty() {
		t.Error("Active SearchState with query should not be empty")
	}
	
	// Test clear
	search.Clear()
	if !search.IsEmpty() {
		t.Error("Cleared SearchState should be empty")
	}
	
	if search.Active {
		t.Error("Cleared SearchState should not be active")
	}
}

func TestSearchStateResultCounting(t *testing.T) {
	search := SearchState{Context: SearchTables}
	
	// Test table results
	search.FilteredTables = []bigquery.TableInfo{{}, {}}
	if search.ResultCount() != 2 {
		t.Errorf("Expected 2 table results, got %d", search.ResultCount())
	}
	
	if !search.HasResults() {
		t.Error("Should have results with filtered tables")
	}
	
	// Test schema results
	search.Context = SearchSchema
	search.FilteredNodes = []schemaNode{{}, {}, {}}
	if search.ResultCount() != 3 {
		t.Errorf("Expected 3 node results, got %d", search.ResultCount())
	}
}

func TestNavigationHandler(t *testing.T) {
	// Create a test model with schema nodes
	model := &browserModel{
		state: stateTableDetail,
		schemaNodes: []schemaNode{
			{Field: bigquery.SchemaField{Name: "field1"}},
			{Field: bigquery.SchemaField{Name: "field2"}},
			{Field: bigquery.SchemaField{Name: "field3"}},
		},
		selectedSchema: 1,
	}
	
	// Test up navigation
	model.handleNavigation("up")
	if model.selectedSchema != 0 {
		t.Errorf("Up navigation failed: expected 0, got %d", model.selectedSchema)
	}
	
	// Test down navigation
	model.handleNavigation("down")
	if model.selectedSchema != 1 {
		t.Errorf("Down navigation failed: expected 1, got %d", model.selectedSchema)
	}
	
	// Test bottom navigation
	model.handleNavigation("bottom")
	if model.selectedSchema != 2 {
		t.Errorf("Bottom navigation failed: expected 2, got %d", model.selectedSchema)
	}
	
	// Test top navigation
	model.handleNavigation("top")
	if model.selectedSchema != 0 {
		t.Errorf("Top navigation failed: expected 0, got %d", model.selectedSchema)
	}
}

func TestNavigationWithFilteredResults(t *testing.T) {
	// Test navigation with filtered search results
	model := &browserModel{
		state: stateTableDetail,
		schemaNodes: []schemaNode{
			{Field: bigquery.SchemaField{Name: "field1"}},
			{Field: bigquery.SchemaField{Name: "field2"}},
			{Field: bigquery.SchemaField{Name: "field3"}},
			{Field: bigquery.SchemaField{Name: "field4"}},
		},
		selectedSchema: 0,
	}
	
	// Add filtered results (subset of schema nodes)
	model.search.FilteredNodes = []schemaNode{
		{Field: bigquery.SchemaField{Name: "field1"}},
		{Field: bigquery.SchemaField{Name: "field3"}},
	}
	
	// Test bottom navigation with filtered results
	model.handleNavigation("bottom")
	if model.selectedSchema != 1 { // Should be index 1 in filtered results (2 items)
		t.Errorf("Bottom navigation with filtered results failed: expected 1, got %d", model.selectedSchema)
	}
}

func TestSearchModeActivation(t *testing.T) {
	model := &browserModel{
		state: stateTableList,
		search: SearchState{},
	}
	
	// Test entering search mode
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}
	result, _ := model.handleKeyPress(msg)
	updatedModel := result.(*browserModel)
	
	if !updatedModel.search.Active {
		t.Error("Search mode should be active after pressing '/'")
	}
	
	if updatedModel.search.Context != SearchTables {
		t.Error("Search context should be SearchTables in table list state")
	}
}

func TestEscapeFromSearch(t *testing.T) {
	model := &browserModel{
		state: stateTableList,
		search: SearchState{
			Active: true,
			Query: "test",
			Context: SearchTables,
		},
	}
	
	// Test escape from search
	msg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := model.handleSearchInput(msg)
	updatedModel := result.(*browserModel)
	
	if updatedModel.search.Active {
		t.Error("Search mode should be inactive after escape")
	}
	
	if updatedModel.search.Query != "" {
		t.Error("Search query should be cleared after escape")
	}
}

func TestCommandModeActivation(t *testing.T) {
	model := &browserModel{
		state: stateTableList,
		commandMode: false,
	}
	
	// Test entering command mode
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")}
	result, _ := model.handleKeyPress(msg)
	updatedModel := result.(*browserModel)
	
	if !updatedModel.commandMode {
		t.Error("Command mode should be active after pressing ':'")
	}
	
	if updatedModel.commandQuery != "" {
		t.Error("Command query should be empty when entering command mode")
	}
}

func TestSearchStateIsolation(t *testing.T) {
	model := &browserModel{
		state: stateTableList,
		search: SearchState{
			Active: true,
			Query: "test_query",
			Context: SearchTables,
		},
	}
	
	// Test that clearing search state isolates between views
	model.clearSearchState()
	
	if model.search.Active {
		t.Error("Search should not be active after clear")
	}
	
	if model.search.Query != "" {
		t.Error("Search query should be empty after clear")
	}
	
	if model.search.FilteredTables != nil {
		t.Error("Filtered tables should be nil after clear")
	}
	
	if model.search.FilteredNodes != nil {
		t.Error("Filtered nodes should be nil after clear")
	}
}

func TestEscapeFromCommandMode(t *testing.T) {
	model := &browserModel{
		state: stateTableList,
		commandMode: true,
		commandQuery: "test_command",
	}
	
	// Test escape from command mode using KeyEscape type
	msg := tea.KeyMsg{Type: tea.KeyEscape}
	result, _ := model.handleCommandInput(msg)
	updatedModel := result.(*browserModel)
	
	if updatedModel.commandMode {
		t.Error("Command mode should be inactive after escape")
	}
	
	if updatedModel.commandQuery != "" {
		t.Error("Command query should be cleared after escape")
	}
}

func TestFooterIntegration(t *testing.T) {
	model := &browserModel{
		state: stateTableList,
		width: 80,
		height: 24,
	}
	
	// Test normal footer
	footer := model.renderFooter()
	if !strings.Contains(footer, "Navigate") {
		t.Error("Normal footer should contain navigation shortcuts")
	}
	
	// Test search mode footer
	model.search.Active = true
	model.search.Query = "test"
	footer = model.renderFooter()
	if !strings.Contains(footer, "🔍 Search") {
		t.Error("Search footer should contain search indicator")
	}
	if !strings.Contains(footer, "test") {
		t.Error("Search footer should contain search query")
	}
	
	// Test command mode footer
	model.search.Active = false
	model.commandMode = true
	model.commandQuery = "copy"
	footer = model.renderFooter()
	if !strings.Contains(footer, "⚡ Command") {
		t.Error("Command footer should contain command indicator")
	}
	if !strings.Contains(footer, "copy") {
		t.Error("Command footer should contain command query")
	}
}

func TestSearchInputAllowsNavigationChars(t *testing.T) {
	model := &browserModel{
		state: stateTableList,
		search: SearchState{
			Active: true,
			Context: SearchTables,
		},
	}
	
	// Test that hjkl characters can be typed in search mode
	testChars := []string{"h", "j", "k", "l", "g", "G"}
	
	for _, char := range testChars {
		originalQuery := model.search.Query
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(char)}
		model.handleSearchInput(msg)
		
		expectedQuery := originalQuery + char
		if model.search.Query != expectedQuery {
			t.Errorf("Search should allow typing '%s'. Expected query '%s', got '%s'", 
				char, expectedQuery, model.search.Query)
		}
	}
}

func TestSearchNavigationWithArrowKeys(t *testing.T) {
	model := &browserModel{
		state: stateTableDetail,
		search: SearchState{
			Active: true,
			Context: SearchSchema,
		},
		schemaNodes: []schemaNode{
			{Field: bigquery.SchemaField{Name: "field1"}},
			{Field: bigquery.SchemaField{Name: "field2"}},
			{Field: bigquery.SchemaField{Name: "field3"}},
		},
		selectedSchema: 1,
	}
	
	// Test that arrow keys still work for navigation in search mode
	originalQuery := model.search.Query
	
	// Test up arrow
	msg := tea.KeyMsg{Type: tea.KeyUp}
	model.handleSearchInput(msg)
	
	// Query should be unchanged
	if model.search.Query != originalQuery {
		t.Error("Arrow keys should not modify search query")
	}
	
	// Selection should have moved up
	if model.selectedSchema != 0 {
		t.Errorf("Up arrow should move selection. Expected 0, got %d", model.selectedSchema)
	}
}