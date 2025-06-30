package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	prettytable "github.com/jedib0t/go-pretty/v6/table"
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
  bqs browse my-project.analytics          # Browse analytics dataset
  bqs browse my-project.analytics.table    # Deep dive into specific table`,
	Args: cobra.ExactArgs(1),
	RunE: runBrowse,
}

func init() {
	rootCmd.AddCommand(browseCmd)
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
		return runStaticBrowse(project, dataset, table, bqClient)
	}

	return nil
}

func runStaticBrowse(project, dataset, tableName string, client *bigquery.Client) error {
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
	t := prettytable.NewWriter()
	t.SetStyle(prettytable.StyleRounded)

	t.AppendHeader(prettytable.Row{"", "Table", "Type", "Created"})

	for _, tbl := range tables {
		tableName := tbl.TableID
		if tableName == "" {
			tableName = tbl.TableReference.TableID
		}

		icon := bigquery.GetTableTypeIcon(tbl.Type)
		created := bigquery.FormatTime(tbl.CreationTime)

		t.AppendRow(prettytable.Row{
			icon,
			tableName,
			tbl.Type,
			created,
		})
	}

	fmt.Println(t.Render())
	fmt.Printf("\nUse 'bqs browse %s.%s.TABLE_NAME' to explore specific tables\n", project, dataset)

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

func newBrowserModel(project, dataset, tableName string, client *bigquery.Client) *browserModel {
	// Initialize the table component with simple, fast columns
	columns := []table.Column{
		{Title: "Cache", Width: 5},     // Prefetch status
		{Title: "Table", Width: 35},    // Table name
		{Title: "Type", Width: 8},      // Table type
		{Title: "Created", Width: 20},  // Creation time (always available)
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	// Apply styling
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	model := &browserModel{
		project:        project,
		dataset:        dataset,
		table:          tableName,
		client:         client,
		loading:        true,
		tableModel:     t,
		expandedNodes:  make(map[string]bool),
		cachedMetadata: make(map[string]*bigquery.TableMetadata),
	}

	// Always start in loading state when data needs to be fetched
	model.state = stateLoading

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
	var cmd tea.Cmd

	// Update the table model for table list state
	if m.state == stateTableList {
		m.tableModel, cmd = m.tableModel.Update(msg)
	}

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update table model height based on available space
		if m.state == stateTableList {
			tableHeight := m.height - 8 // Account for header, footer, padding
			if tableHeight < 5 {
				tableHeight = 5
			}
			m.tableModel.SetHeight(tableHeight)
		}
		return m, cmd

	case tea.KeyMsg:
		newModel, keyCmd := m.handleKeyPress(msg)
		// Combine commands
		return newModel, tea.Batch(cmd, keyCmd)

	case tableListLoadedMsg:
		m.loading = false
		m.tables = msg.tables
		m.state = stateTableList
		m.checkCacheStatus() // Check for existing cached metadata
		m.updateTableRows()  // Update Bubbletea table component
		return m, nil

	case tableMetadataLoadedMsg:
		m.loading = false
		m.metadata = msg.metadata
		m.state = stateTableDetail
		m.buildSchemaTree()
		// Cache the metadata for future use
		if m.table != "" {
			m.cachedMetadata[m.table] = msg.metadata
			// Update table rows to show the new cache status
			if len(m.tables) > 0 {
				m.updateTableRows()
			}
		}
		return m, nil

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
		// For table list, navigation is handled by the table model
		// For table detail, handle schema navigation
		if m.state == stateTableDetail && m.selectedSchema > 0 {
			m.selectedSchema--
		}

	case "down", "j":
		// For table list, navigation is handled by the table model
		// For table detail, handle schema navigation
		if m.state == stateTableDetail && m.selectedSchema < len(m.schemaNodes)-1 {
			m.selectedSchema++
		}

	case "enter":
		if m.state == stateTableList && len(m.tables) > 0 {
			// Get selected table from the table model cursor
			selectedIdx := m.tableModel.Cursor()
			if selectedIdx >= 0 && selectedIdx < len(m.tables) {
				table := m.tables[selectedIdx]
				tableID := table.TableID
				if tableID == "" {
					tableID = table.TableReference.TableID
				}

				m.table = tableID

				// Check if we have real cached metadata (not just a placeholder)
				if cached, exists := m.cachedMetadata[tableID]; exists && cached != nil && cached.Schema != nil {
					// Use cached data immediately (real metadata, not placeholder)
					m.metadata = cached
					m.state = stateTableDetail
					m.buildSchemaTree()
					return m, nil
				} else {
					// Load metadata and cache it (this will be fast if persistently cached)
					m.loading = true
					m.state = stateLoading
					return m, loadTableMetadata(m.client, m.project, m.dataset, tableID)
				}
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
			// Check if we have table list data
			if len(m.tables) == 0 {
				// Need to load table list first
				m.loading = true
				m.state = stateLoading
				m.table = ""
				m.metadata = nil
				m.schemaNodes = nil
				m.selectedSchema = 0
				return m, loadTableList(m.client, m.project, m.dataset)
			} else {
				// Table list already loaded, just switch state
				m.state = stateTableList
				m.table = ""
				m.metadata = nil
				m.schemaNodes = nil
				m.selectedSchema = 0
				// Update table rows to reflect current cache status
				m.updateTableRows()
			}
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

	// Table list using Bubbletea Table component
	if m.loading {
		// Show loading while data is being fetched
		content.WriteString("🔄 Loading tables...")
	} else if len(m.tables) == 0 {
		content.WriteString("No tables found in this dataset")
	} else {
		// Render the table component
		content.WriteString(m.tableModel.View())
	}

	// Footer
	content.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(1, 1)

	footer := "⌨️  [↑↓] Navigate • [Enter] Explore • [q] Quit • ✓ = Cached"
	content.WriteString(footerStyle.Render(footer))

	return content.String()
}

func (m *browserModel) renderTableDetail() string {
	if m.metadata == nil {
		if m.loading {
			return lipgloss.NewStyle().
				Width(m.width).
				Height(m.height).
				Align(lipgloss.Center, lipgloss.Center).
				Render("🔄 Loading table metadata...")
		}
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

// updateTableRows populates the Bubbletea table component with current table data
func (m *browserModel) updateTableRows() {
	if len(m.tables) == 0 {
		m.tableModel.SetRows([]table.Row{})
		return
	}

	rows := make([]table.Row, len(m.tables))
	for i, tbl := range m.tables {
		tableID := tbl.TableID
		if tableID == "" {
			tableID = tbl.TableReference.TableID
		}

		// Add cache status indicator
		cacheStatus := ""
		if _, isCached := m.cachedMetadata[tableID]; isCached {
			cacheStatus = "✓" // Cached
		}

		// Always show basic, fast info - creation time is always available
		created := bigquery.FormatTime(tbl.CreationTime)
		rows[i] = table.Row{cacheStatus, tableID, tbl.Type, created}
	}

	m.tableModel.SetRows(rows)
}

// checkCacheStatus scans the underlying cache to see which tables are already cached
func (m *browserModel) checkCacheStatus() {
	if len(m.tables) == 0 {
		return
	}

	// Check each table to see if it's in the underlying cache
	cacheUpdated := false
	for _, tbl := range m.tables {
		tableID := tbl.TableID
		if tableID == "" {
			tableID = tbl.TableReference.TableID
		}

		// Check if metadata is cached for this table
		if m.client.IsTableMetadataCached(m.project, m.dataset, tableID) {
			// Mark this table as having cached metadata
			// We don't load the actual metadata yet (lazy loading)
			// but we mark it as cached for UI purposes
			if m.cachedMetadata == nil {
				m.cachedMetadata = make(map[string]*bigquery.TableMetadata)
			}
			// Use a placeholder to indicate it's cached
			m.cachedMetadata[tableID] = &bigquery.TableMetadata{}
			cacheUpdated = true
		}
	}

	// If we found cached items, update the table display
	if cacheUpdated {
		m.updateTableRows()
	}
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
