package cmd

import (
	"time"
	
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/table"

	"bqs/internal/bigquery"
)

// browserState represents the current view state
type browserState int

const (
	stateLoading browserState = iota
	stateTableList
	stateTableDetail
	stateError
	stateHelp
	stateSearch
	stateCommand
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

	// Search state
	search SearchState

	// Cache state (lazy loading)
	cachedMetadata map[string]*bigquery.TableMetadata

	// UI state
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
	
	// Command mode state
	commandMode  bool
	commandQuery string
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