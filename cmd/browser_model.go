package cmd

import (
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

	// Cache state (lazy loading)
	cachedMetadata map[string]*bigquery.TableMetadata

	// UI state
	loading bool
	err     error
	width   int
	height  int
}

// schemaNode represents a node in the schema tree
type schemaNode struct {
	Field       bigquery.SchemaField
	Path        string // Unique path for tracking expansion state
	Level       int    // Nesting level for indentation
	HasChildren bool
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