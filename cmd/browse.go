package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"bqs/internal/bigquery"
	"bqs/internal/cache"
)

var browseCmd = &cobra.Command{
	Use:   "browse <project.dataset>",
	Short: "Interactive BigQuery dataset browser",
	Long: `Browse BigQuery datasets interactively with a terminal UI.

Navigate tables and views with keyboard controls, view schemas, and explore
your BigQuery resources without writing queries.

Examples:
  bqs browse my-project.analytics          # Browse analytics dataset (fast)
  bqs browse -d my-project.analytics       # Browse with detailed metadata (slower)
  bqs browse my-project.analytics.table    # Deep dive into specific table`,
	Args: cobra.ExactArgs(1),
	RunE: runBrowse,
}

var detailedMode bool

func init() {
	rootCmd.AddCommand(browseCmd)
	browseCmd.Flags().BoolVarP(&detailedMode, "detailed", "d", false, "Fetch detailed metadata (size, rows) for each table - slower but complete")
}

func runBrowse(cmd *cobra.Command, args []string) error {
	input := args[0]
	
	// Parse input - could be project.dataset or project.dataset.table
	parts := strings.Split(input, ".")
	if len(parts) < 2 {
		return fmt.Errorf("invalid format: expected project.dataset or project.dataset.table, got %s", input)
	}
	
	project := parts[0]
	dataset := parts[1]
	var table string
	if len(parts) > 2 {
		table = strings.Join(parts[2:], ".")
	}
	
	// Initialize cache and BigQuery client
	c, err := cache.New(15 * time.Minute)
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}
	defer c.Close()
	
	bqClient := bigquery.NewClient(c)
	
	// Try interactive mode first, fallback to static mode
	model := newBrowserModel(project, dataset, table, bqClient)
	p := tea.NewProgram(model, tea.WithAltScreen())
	
	if _, err := p.Run(); err != nil {
		// Fallback to static listing if interactive mode fails
		return runStaticBrowse(project, dataset, table, bqClient, detailedMode)
	}
	
	return nil
}

func runStaticBrowse(project, dataset, tableName string, client *bigquery.Client, detailed bool) error {
	if tableName != "" {
		// Show specific table metadata
		metadata, err := client.GetTableMetadata(project, dataset, tableName)
		if err != nil {
			return fmt.Errorf("failed to get table metadata: %w", err)
		}
		
		fmt.Printf("📊 %s.%s.%s (%s)\n", project, dataset, tableName, metadata.Type)
		fmt.Printf("📈 %d rows • 💾 %s • 🕒 Modified %s\n\n", 
			metadata.NumRows, 
			bigquery.FormatSize(metadata.NumBytes),
			bigquery.FormatTime(metadata.LastModifiedTime))
		
		if metadata.Schema != nil {
			fmt.Println("🌲 Schema:")
			for _, field := range metadata.Schema.Fields {
				mode := ""
				if field.Mode == "REQUIRED" {
					mode = " (REQUIRED)"
				} else if field.Mode == "REPEATED" {
					mode = " (REPEATED)"
				}
				fmt.Printf("  ├─ %s %s%s\n", field.Name, field.Type, mode)
			}
		}
		
		return nil
	}
	
	// Show table list
	tables, err := client.ListTables(project, dataset)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}
	
	fmt.Printf("📊 %s.%s\n\n", project, dataset)
	
	if len(tables) == 0 {
		fmt.Println("No tables found in this dataset")
		return nil
	}
	
	// Create a nicely formatted table
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	
	if detailed {
		fmt.Println("🔄 Fetching detailed metadata for each table...")
		t.AppendHeader(table.Row{"", "Table", "Type", "Rows", "Size", "Modified"})
		
		for _, tbl := range tables {
			tableName := tbl.TableID
			if tableName == "" {
				tableName = tbl.TableReference.TableID
			}
			
			icon := bigquery.GetTableTypeIcon(tbl.Type)
			
			// Fetch detailed metadata
			metadata, err := client.GetTableMetadata(project, dataset, tableName)
			if err != nil {
				// Fallback to basic info if detailed fetch fails
				t.AppendRow(table.Row{
					icon,
					tableName,
					tbl.Type,
					"Error",
					"Error",
					bigquery.FormatTime(tbl.CreationTime),
				})
				continue
			}
			
			rows := "0"
			if metadata.NumRows > 0 {
				rows = fmt.Sprintf("%d", metadata.NumRows)
			}
			
			size := bigquery.FormatSize(metadata.NumBytes)
			modified := bigquery.FormatTime(metadata.LastModifiedTime)
			
			t.AppendRow(table.Row{
				icon,
				tableName,
				tbl.Type,
				rows,
				size,
				modified,
			})
		}
	} else {
		t.AppendHeader(table.Row{"", "Table", "Type", "Created"})
		
		for _, tbl := range tables {
			tableName := tbl.TableID
			if tableName == "" {
				tableName = tbl.TableReference.TableID
			}
			
			icon := bigquery.GetTableTypeIcon(tbl.Type)
			created := bigquery.FormatTime(tbl.CreationTime)
			
			t.AppendRow(table.Row{
				icon,
				tableName,
				tbl.Type,
				created,
			})
		}
	}
	
	fmt.Println(t.Render())
	
	if detailed {
		fmt.Printf("\n💡 Detailed metadata fetched for %d tables\n", len(tables))
	} else {
		fmt.Printf("\n💡 Use --detailed flag for size and row count information\n")
	}
	
	fmt.Printf("Use 'bqs browse %s.%s.TABLE_NAME' to explore specific tables\n", project, dataset)
	
	return nil
}

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
	state     browserState
	project   string
	dataset   string
	table     string
	client    *bigquery.Client
	
	// Table list state
	tables      []bigquery.TableInfo
	selectedIdx int
	
	// Table detail state
	metadata *bigquery.TableMetadata
	
	// Schema tree state
	schemaNodes    []schemaNode
	selectedSchema int
	expandedNodes  map[string]bool
	
	// Prefetch state
	prefetchedTables map[string]*bigquery.TableMetadata
	prefetchQueue    []string
	prefetchInProgress map[string]bool
	
	// UI state
	loading    bool
	err        error
	width      int
	height     int
}

// schemaNode represents a node in the schema tree
type schemaNode struct {
	Field    bigquery.SchemaField
	Path     string // Unique path for tracking expansion state
	Level    int    // Nesting level for indentation
	HasChildren bool
}

func newBrowserModel(project, dataset, table string, client *bigquery.Client) *browserModel {
	model := &browserModel{
		project:            project,
		dataset:            dataset,
		table:              table,
		client:             client,
		loading:            true,
		expandedNodes:      make(map[string]bool),
		prefetchedTables:   make(map[string]*bigquery.TableMetadata),
		prefetchInProgress: make(map[string]bool),
	}
	
	if table != "" {
		model.state = stateTableDetail
	} else {
		model.state = stateTableList
	}
	
	return model
}

// Init implements tea.Model
func (m *browserModel) Init() tea.Cmd {
	if m.table != "" {
		return loadTableMetadata(m.client, m.project, m.dataset, m.table)
	}
	return loadTableList(m.client, m.project, m.dataset)
}

// Update implements tea.Model
func (m *browserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
		
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
		
	case tableListLoadedMsg:
		m.loading = false
		m.tables = msg.tables
		m.state = stateTableList
		m.buildPrefetchQueue()
		return m, startPrefetch
		
	case tableMetadataLoadedMsg:
		m.loading = false
		m.metadata = msg.metadata
		m.state = stateTableDetail
		m.buildSchemaTree()
		return m, nil
		
	case tablePrefetchedMsg:
		// Store prefetched metadata
		m.prefetchedTables[msg.tableID] = msg.metadata
		delete(m.prefetchInProgress, msg.tableID)
		
		// Continue prefetching next table
		return m, m.continueNextPrefetch()
		
	case prefetchErrorMsg:
		// Remove from progress tracking (silently fail prefetch)
		delete(m.prefetchInProgress, msg.tableID)
		
		// Continue prefetching next table
		return m, m.continueNextPrefetch()
		
	case startPrefetchMsg:
		return m, m.startNextPrefetch()
		
	case errorMsg:
		m.loading = false
		m.err = msg.err
		m.state = stateError
		return m, nil
	}
	
	return m, nil
}

// View implements tea.Model
func (m *browserModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	
	switch m.state {
	case stateLoading:
		return m.renderLoading()
	case stateTableList:
		return m.renderTableList()
	case stateTableDetail:
		return m.renderTableDetail()
	case stateError:
		return m.renderError()
	default:
		return "Unknown state"
	}
}

func (m *browserModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
		
	case "up", "k":
		if m.state == stateTableList && m.selectedIdx > 0 {
			m.selectedIdx--
		} else if m.state == stateTableDetail && m.selectedSchema > 0 {
			m.selectedSchema--
		}
		
	case "down", "j":
		if m.state == stateTableList && m.selectedIdx < len(m.tables)-1 {
			m.selectedIdx++
		} else if m.state == stateTableDetail && m.selectedSchema < len(m.schemaNodes)-1 {
			m.selectedSchema++
		}
		
	case "enter":
		if m.state == stateTableList && len(m.tables) > 0 {
			table := m.tables[m.selectedIdx]
			tableID := table.TableID
			if tableID == "" {
				tableID = table.TableReference.TableID
			}
			
			m.table = tableID
			
			// Check if we have prefetched data
			if prefetched, exists := m.prefetchedTables[tableID]; exists {
				// Use prefetched data immediately
				m.metadata = prefetched
				m.state = stateTableDetail
				m.buildSchemaTree()
				return m, nil
			} else {
				// Fallback to loading
				m.loading = true
				m.state = stateLoading
				return m, loadTableMetadata(m.client, m.project, m.dataset, tableID)
			}
		}
		
	case "space", "right", "l":
		// Expand/collapse schema nodes
		if m.state == stateTableDetail && len(m.schemaNodes) > 0 {
			node := m.schemaNodes[m.selectedSchema]
			if node.HasChildren {
				m.expandedNodes[node.Path] = !m.expandedNodes[node.Path]
				m.buildSchemaTree() // Rebuild tree with new expansion state
			}
		}
		
	case "left", "h":
		// Collapse current node or move to parent
		if m.state == stateTableDetail && len(m.schemaNodes) > 0 {
			node := m.schemaNodes[m.selectedSchema]
			if m.expandedNodes[node.Path] {
				// Collapse current node
				m.expandedNodes[node.Path] = false
				m.buildSchemaTree()
			} else if node.Level > 0 {
				// Move to parent node
				for i := m.selectedSchema - 1; i >= 0; i-- {
					if m.schemaNodes[i].Level < node.Level {
						m.selectedSchema = i
						break
					}
				}
			}
		}
		
	case "b", "backspace":
		if m.state == stateTableDetail {
			m.state = stateTableList
			m.table = ""
			m.metadata = nil
			m.schemaNodes = nil
			m.selectedSchema = 0
		}
	}
	
	return m, nil
}

func (m *browserModel) renderLoading() string {
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render("🔄 Loading BigQuery metadata...")
}

func (m *browserModel) renderTableList() string {
	var content strings.Builder
	
	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Padding(0, 1)
	
	content.WriteString(headerStyle.Render(fmt.Sprintf("📊 %s.%s", m.project, m.dataset)))
	content.WriteString("\n\n")
	
	// Table list
	if len(m.tables) == 0 {
		content.WriteString("No tables found in this dataset")
	} else {
		for i, table := range m.tables {
			selected := i == m.selectedIdx
			
			var style lipgloss.Style
			if selected {
				style = lipgloss.NewStyle().
					Background(lipgloss.Color("62")).
					Foreground(lipgloss.Color("230")).
					Padding(0, 1)
			} else {
				style = lipgloss.NewStyle().
					Padding(0, 1)
			}
			
			tableID := table.TableID
			if tableID == "" {
				tableID = table.TableReference.TableID
			}
			
			icon := bigquery.GetTableTypeIcon(table.Type)
			size := bigquery.FormatSize(table.NumBytes)
			lastMod := bigquery.FormatTime(table.LastModifiedTime)
			rows := ""
			if table.NumRows > 0 {
				rows = fmt.Sprintf("%d rows", table.NumRows)
			}
			
			// Add prefetch status indicator
			prefetchStatus := " "
			if _, isPrefetched := m.prefetchedTables[tableID]; isPrefetched {
				prefetchStatus = "✓" // Cached
			} else if m.prefetchInProgress[tableID] {
				prefetchStatus = "⏳" // Loading
			}
			
			line := fmt.Sprintf("%s%s %-29s %8s %10s %s", 
				icon, prefetchStatus, tableID, size, rows, lastMod)
			
			content.WriteString(style.Render(line))
			content.WriteString("\n")
		}
	}
	
	// Footer
	content.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(1, 1)
	
	footer := "⌨️  [↑↓] Navigate • [Enter] Explore • [q] Quit"
	content.WriteString(footerStyle.Render(footer))
	
	return content.String()
}

func (m *browserModel) renderTableDetail() string {
	if m.metadata == nil {
		return "No metadata available"
	}
	
	var content strings.Builder
	
	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Padding(0, 1)
	
	icon := bigquery.GetTableTypeIcon(m.metadata.Type)
	content.WriteString(headerStyle.Render(fmt.Sprintf("%s %s.%s.%s", icon, m.project, m.dataset, m.table)))
	content.WriteString("\n\n")
	
	// Metadata
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Padding(0, 1)
	
	size := bigquery.FormatSize(m.metadata.NumBytes)
	lastMod := bigquery.FormatTime(m.metadata.LastModifiedTime)
	
	meta := fmt.Sprintf("📊 %d rows • 💾 %s • 🕒 Modified %s", 
		m.metadata.NumRows, size, lastMod)
	content.WriteString(metaStyle.Render(meta))
	content.WriteString("\n\n")
	
	// Schema
	if m.metadata.Schema != nil && len(m.schemaNodes) > 0 {
		schemaStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Padding(0, 1)
		
		content.WriteString(schemaStyle.Render("🌲 Schema:"))
		content.WriteString("\n\n")
		
		for i, node := range m.schemaNodes {
			selected := i == m.selectedSchema
			
			var style lipgloss.Style
			if selected {
				style = lipgloss.NewStyle().
					Background(lipgloss.Color("62")).
					Foreground(lipgloss.Color("230")).
					Padding(0, 1)
			} else {
				style = lipgloss.NewStyle().Padding(0, 1)
			}
			
			// Build indentation
			indent := strings.Repeat("  ", node.Level)
			
			// Build tree connector
			connector := "├─"
			if node.Level == 0 {
				connector = "├─"
			} else {
				connector = "├─"
			}
			
			// Build expansion indicator
			expandIcon := ""
			if node.HasChildren {
				if m.expandedNodes[node.Path] {
					expandIcon = "▼ "
				} else {
					expandIcon = "▶ "
				}
			} else {
				expandIcon = "  "
			}
			
			// Build mode indicator
			mode := ""
			if node.Field.Mode == "REQUIRED" {
				mode = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(" REQUIRED")
			} else if node.Field.Mode == "REPEATED" {
				mode = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render(" REPEATED")
			}
			
			// Build type with color
			typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render(node.Field.Type)
			
			line := fmt.Sprintf("%s%s%s%s %s%s", indent, connector, expandIcon, node.Field.Name, typeStyle, mode)
			content.WriteString(style.Render(line))
			content.WriteString("\n")
		}
	}
	
	// Footer
	content.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(1, 1)
	
	footer := "⌨️  [↑↓] Navigate • [Space/→] Expand • [←] Collapse • [b] Back • [q] Quit"
	content.WriteString(footerStyle.Render(footer))
	
	return content.String()
}

func (m *browserModel) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196"))
	
	return errorStyle.Render(fmt.Sprintf("❌ Error: %s\n\nPress [q] to quit", m.err.Error()))
}

// buildSchemaTree constructs the flattened schema tree for display
func (m *browserModel) buildSchemaTree() {
	if m.metadata == nil || m.metadata.Schema == nil {
		return
	}
	
	m.schemaNodes = []schemaNode{}
	m.buildSchemaNodesRecursive(m.metadata.Schema.Fields, "", 0)
	
	// Reset selection if it's out of bounds
	if m.selectedSchema >= len(m.schemaNodes) {
		m.selectedSchema = 0
	}
}

// buildSchemaNodesRecursive recursively builds schema nodes with proper expansion state
func (m *browserModel) buildSchemaNodesRecursive(fields []bigquery.SchemaField, parentPath string, level int) {
	for _, field := range fields {
		// Build unique path for this field
		var path string
		if parentPath == "" {
			path = field.Name
		} else {
			path = parentPath + "." + field.Name
		}
		
		// Check if this field has nested fields
		hasChildren := len(field.Fields) > 0
		
		// Add this field to the nodes
		node := schemaNode{
			Field:       field,
			Path:        path,
			Level:       level,
			HasChildren: hasChildren,
		}
		m.schemaNodes = append(m.schemaNodes, node)
		
		// If this node is expanded and has children, add them recursively
		if hasChildren && m.expandedNodes[path] {
			m.buildSchemaNodesRecursive(field.Fields, path, level+1)
		}
	}
}

// buildPrefetchQueue creates a prioritized queue of tables to prefetch
func (m *browserModel) buildPrefetchQueue() {
	m.prefetchQueue = []string{}
	
	// Prioritize tables around the current selection
	start := m.selectedIdx
	if start > 0 {
		start--
	}
	end := start + 5 // Prefetch 5 tables around selection
	if end > len(m.tables) {
		end = len(m.tables)
	}
	
	// Add priority tables first
	for i := start; i < end; i++ {
		tableID := m.tables[i].TableID
		if tableID == "" {
			tableID = m.tables[i].TableReference.TableID
		}
		m.prefetchQueue = append(m.prefetchQueue, tableID)
	}
	
	// Add remaining tables
	for _, table := range m.tables {
		tableID := table.TableID
		if tableID == "" {
			tableID = table.TableReference.TableID
		}
		
		// Skip if already in queue
		skip := false
		for _, queued := range m.prefetchQueue {
			if queued == tableID {
				skip = true
				break
			}
		}
		if !skip {
			m.prefetchQueue = append(m.prefetchQueue, tableID)
		}
	}
}

// startNextPrefetch begins prefetching the next table in queue
func (m *browserModel) startNextPrefetch() tea.Cmd {
	// Find next table to prefetch
	for len(m.prefetchQueue) > 0 {
		tableID := m.prefetchQueue[0]
		m.prefetchQueue = m.prefetchQueue[1:]
		
		// Skip if already prefetched or in progress
		if _, exists := m.prefetchedTables[tableID]; exists {
			continue
		}
		if m.prefetchInProgress[tableID] {
			continue
		}
		
		// Start prefetching this table
		m.prefetchInProgress[tableID] = true
		return prefetchTableMetadata(m.client, m.project, m.dataset, tableID)
	}
	
	return nil
}

// continueNextPrefetch continues the prefetch queue
func (m *browserModel) continueNextPrefetch() tea.Cmd {
	return m.startNextPrefetch()
}

// Messages for async operations
type tableListLoadedMsg struct {
	tables []bigquery.TableInfo
}

type tableMetadataLoadedMsg struct {
	metadata *bigquery.TableMetadata
}

type tablePrefetchedMsg struct {
	tableID  string
	metadata *bigquery.TableMetadata
}

type prefetchErrorMsg struct {
	tableID string
	err     error
}

type startPrefetchMsg struct{}

type errorMsg struct {
	err error
}

// Commands for async operations
func loadTableList(client *bigquery.Client, project, dataset string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
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

func prefetchTableMetadata(client *bigquery.Client, project, dataset, table string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		metadata, err := client.GetTableMetadata(project, dataset, table)
		if err != nil {
			return prefetchErrorMsg{tableID: table, err: err}
		}
		return tablePrefetchedMsg{tableID: table, metadata: metadata}
	})
}

func startPrefetch() tea.Msg {
	return startPrefetchMsg{}
}