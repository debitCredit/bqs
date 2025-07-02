package cmd

import (
	"encoding/json"
	"fmt"
	"time"
	
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/table"

	"bqs/internal/bigquery"
	"bqs/internal/errors"
	"bqs/internal/utils"
)

// browserState represents the current view state
type browserState int

const (
	stateLoading browserState = iota
	stateTableList
	stateTableDetail
	stateError
	stateHelp
)

// UIMode represents the current input/interaction mode
type UIMode int

const (
	modeNormal UIMode = iota
	modeSearch
)

// browserModel is the main Bubble Tea model
type browserModel struct {
	state   browserState
	project string
	dataset string
	table   string
	client  *bigquery.Client

	// Table list state
	tables     []bigquery.TableInfo
	tableModel table.Model // Bubbletea table component

	// Table detail state
	metadata *bigquery.TableMetadata

	// Schema tree state
	schemaNodes    []schemaNode
	selectedSchema int
	expandedNodes  map[string]bool

	// Consolidated UI interaction state
	ui UIState
	
	// Key handling
	keyDispatcher *KeyDispatcher

	// Cache state (lazy loading)
	cachedMetadata map[string]*bigquery.TableMetadata

	// UI rendering state
	loading bool
	err     error
	width   int
	height  int
	
	// Vim-style navigation state
	lastKey string // For tracking key sequences like 'gg'
	
	// Status message state
	statusMessage string
	statusTimeout time.Time
	
	// Help state
	previousState browserState // Store previous state when showing help
}

// schemaNode represents a node in the schema tree
type schemaNode struct {
	Field       bigquery.SchemaField
	Path        string // Unique path for tracking expansion state
	Level       int    // Nesting level for indentation
	HasChildren bool
}

// SearchContext represents what type of content is being searched
type SearchContext int

const (
	SearchTables SearchContext = iota
	SearchSchema
)

// UIState consolidates all user interface interaction state
type UIState struct {
	Mode           UIMode
	Search         SearchState
}

// IsSearchMode returns true if currently in search mode
func (ui *UIState) IsSearchMode() bool {
	return ui.Mode == modeSearch
}


// IsNormalMode returns true if currently in normal interaction mode
func (ui *UIState) IsNormalMode() bool {
	return ui.Mode == modeNormal
}

// EnterSearchMode switches to search mode and activates search state
func (ui *UIState) EnterSearchMode(context SearchContext) {
	ui.Mode = modeSearch
	ui.Search.Active = true
	ui.Search.Context = context
	ui.Search.Query = ""
}


// ExitSpecialMode returns to normal mode and clears all special state
func (ui *UIState) ExitSpecialMode() {
	ui.Mode = modeNormal
	ui.Search.Clear()
}

// SearchState encapsulates all search-related state and behavior
type SearchState struct {
	Active         bool
	Query          string
	Context        SearchContext
	SelectedIndex  int
	FilteredTables []bigquery.TableInfo
	FilteredNodes  []schemaNode
}

// Clear resets the search state
func (s *SearchState) Clear() {
	s.Active = false
	s.Query = ""
	s.SelectedIndex = 0
	s.FilteredTables = nil
	s.FilteredNodes = nil
}

// IsEmpty returns true if no search is active
func (s *SearchState) IsEmpty() bool {
	return !s.Active || s.Query == ""
}

// ResultCount returns the number of filtered results
func (s *SearchState) ResultCount() int {
	if s.Context == SearchTables {
		return len(s.FilteredTables)
	}
	return len(s.FilteredNodes)
}

// HasResults returns true if there are filtered results
func (s *SearchState) HasResults() bool {
	return s.ResultCount() > 0
}

// Messages for async operations
type tableListLoadedMsg struct {
	tables []bigquery.TableInfo
}

type tableMetadataLoadedMsg struct {
	metadata *bigquery.TableMetadata
}

type errorMsg struct {
	err error
}

type exportCompletedMsg struct {
	tableID   string
	success   bool
	error     string
	retryable bool
	metadata  *bigquery.TableMetadata // Include metadata for caching
}

// Commands for async operations
func loadTableList(client *bigquery.Client, project, dataset string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Always start with fast basic table list
		tables, err := client.ListTables(project, dataset)
		if err != nil {
			return errorMsg{err}
		}
		return tableListLoadedMsg{tables}
	})
}

func loadTableMetadata(client *bigquery.Client, project, dataset, table string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		metadata, err := client.GetTableMetadata(project, dataset, table)
		if err != nil {
			return errorMsg{err}
		}
		return tableMetadataLoadedMsg{metadata}
	})
}

func exportTableMetadata(client *bigquery.Client, project, dataset, tableID string, existingMetadata *bigquery.TableMetadata) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		var tableMetadata *bigquery.TableMetadata
		var err error
		
		// Use existing metadata if available, otherwise fetch it
		if existingMetadata != nil {
			tableMetadata = existingMetadata
		} else {
			tableMetadata, err = client.GetTableMetadata(project, dataset, tableID)
			if err != nil {
				// Determine if error is retryable and get user-friendly message
				errorMessage := err.Error()
				retryable := false
				
				if bqsErr, ok := err.(*errors.BQSError); ok {
					errorMessage = bqsErr.UserFriendlyMessage()
					retryable = bqsErr.IsRetryable()
				}
				
				return exportCompletedMsg{
					tableID:   tableID,
					success:   false,
					error:     errorMessage,
					retryable: retryable,
					metadata:  nil,
				}
			}
		}

		// Create comprehensive export data structure
		exportData := struct {
			Project     string                    `json:"project"`
			Dataset     string                    `json:"dataset"`
			TableID     string                    `json:"table_id"`
			FullTableID string                    `json:"full_table_id"`
			Type        string                    `json:"type"`
			Metadata    *bigquery.TableMetadata   `json:"metadata"`
			ExportedAt  string                    `json:"exported_at"`
		}{
			Project:     project,
			Dataset:     dataset,
			TableID:     tableID,
			FullTableID: fmt.Sprintf("%s.%s.%s", project, dataset, tableID),
			Type:        tableMetadata.Type,
			Metadata:    tableMetadata,
			ExportedAt:  time.Now().Format(time.RFC3339),
		}

		// Marshal to JSON with pretty formatting  
		jsonData, err := json.MarshalIndent(exportData, "", "  ")
		if err != nil {
			return exportCompletedMsg{
				tableID:   tableID,
				success:   false,
				error:     "Failed to generate JSON export",
				retryable: false,
				metadata:  nil,
			}
		}

		// Copy to clipboard
		if err := utils.CopyToClipboard(string(jsonData)); err != nil {
			return exportCompletedMsg{
				tableID:   tableID,
				success:   false,
				error:     "Clipboard not available (install xclip/xsel/pbcopy)",
				retryable: false,
				metadata:  nil,
			}
		}

		return exportCompletedMsg{
			tableID:  tableID,
			success:  true,
			metadata: tableMetadata,
		}
	})
}