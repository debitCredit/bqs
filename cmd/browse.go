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
	"bqs/internal/config"
	"bqs/internal/utils"
	"bqs/internal/validation"
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

	// Validate input format
	if err := validation.ValidateProjectDatasetTable(input); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// Parse input - could be project.dataset or project.dataset.table
	parts := strings.Split(input, ".")
	project := parts[0]
	dataset := parts[1]
	var table string
	if len(parts) > 2 {
		table = strings.Join(parts[2:], ".")
	}

	// Initialize cache and BigQuery client
	c, err := utils.NewCache()
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

	t.AppendHeader(prettytable.Row{"Table", "Type", "Created", "Cache"})

	for _, tbl := range tables {
		tableName := tbl.TableID
		if tableName == "" {
			tableName = tbl.TableReference.TableID
		}

		icon := bigquery.GetTableTypeIcon(tbl.Type)
		created := bigquery.FormatTime(tbl.CreationTime)

		t.AppendRow(prettytable.Row{
			tableName,
			tbl.Type,
			created,
			icon, // Use icon as cache indicator for static view
		})
	}

	fmt.Println(t.Render())
	fmt.Printf("\nUse 'bqs browse %s.%s.TABLE_NAME' to explore specific tables\n", project, dataset)

	return nil
}


func newBrowserModel(project, dataset, tableName string, client *bigquery.Client) *browserModel {
	// Initialize the table component with better column order
	columns := []table.Column{
		{Title: "Table", Width: config.TableColumnWidth},
		{Title: "Type", Width: config.TypeColumnWidth},
		{Title: "Created", Width: config.CreatedColumnWidth},
		{Title: "Cache", Width: config.CacheColumnWidth},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(config.DefaultTableHeight),
	)

	// Apply enhanced styling
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(darkGray).
		BorderBottom(true).
		Bold(true).
		Foreground(primaryBlue)
	s.Selected = s.Selected.
		Foreground(selectedFg).
		Background(selectedBg).
		Bold(true)
	s.Cell = s.Cell.
		Foreground(lightGray)
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

	// Clear expired status messages
	if !m.statusTimeout.IsZero() && time.Now().After(m.statusTimeout) {
		m.statusMessage = ""
		m.statusTimeout = time.Time{}
	}

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
			tableHeight := m.height - config.HeaderFooterPadding
			if tableHeight < config.MinTableHeight {
				tableHeight = config.MinTableHeight
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
	case stateHelp:
		return m.renderHelp()
	default:
		return "Unknown state"
	}
}

func (m *browserModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	
	// Handle search mode FIRST - this takes priority
	if m.search.Active {
		return m.handleSearchInput(msg)
	}
	
	// Handle command mode
	if m.commandMode {
		return m.handleCommandInput(msg)
	}
	
	// Handle help mode separately
	if m.state == stateHelp {
		switch key {
		case "q", "ctrl+c", "?", "escape":
			// These keys work in help mode - handle below
		default:
			// Ignore all other keys in help mode
			return m, nil
		}
	}
	
	switch key {
	case "q", "ctrl+c":
		if m.state == stateHelp {
			// Return to previous state when quitting from help
			m.state = m.previousState
			return m, nil
		}
		return m, tea.Quit

	case "?":
		if m.state != stateHelp {
			// Show help overlay
			m.previousState = m.state
			m.state = stateHelp
		} else {
			// Hide help overlay
			m.state = m.previousState
		}
		m.lastKey = ""
		return m, nil

	case "/":
		// Enter search mode (for table list and table detail)
		if (m.state == stateTableList || m.state == stateTableDetail) && !m.search.Active {
			m.search.Active = true
			m.search.Query = ""
			if m.state == stateTableList {
				m.search.Context = SearchTables
			} else {
				m.search.Context = SearchSchema
			}
			m.lastKey = ""
			return m, nil
		}

	case ":":
		// Enter command mode
		if !m.commandMode && !m.search.Active {
			m.commandMode = true
			m.commandQuery = ""
			m.lastKey = ""
			return m, nil
		}

	case "escape":
		if m.state == stateHelp {
			// Hide help overlay
			m.state = m.previousState
			m.lastKey = ""
			return m, nil
		}
		// Note: search and command mode escapes are handled in their respective input handlers

	case "g":
		if m.lastKey == "g" { // gg sequence - jump to top
			m.handleNavigation("top")
			m.lastKey = ""
			return m, nil
		}
		m.lastKey = "g"
		return m, nil

	case "G":
		m.handleNavigation("bottom")
		m.lastKey = ""

	case "y":
		if m.lastKey == "y" { // yy sequence - copy table identifier
			m.copyCurrentTable()
			m.lastKey = ""
			return m, nil
		}
		m.lastKey = "y"
		return m, nil

	case "up", "k":
		m.handleNavigation("up")
		m.lastKey = ""

	case "down", "j":
		m.handleNavigation("down")
		m.lastKey = ""

	case "enter":
		m.lastKey = ""
		if m.state == stateTableList && len(m.tables) > 0 {
			// Get selected table from the table model cursor
			selectedIdx := m.tableModel.Cursor()
			
			// Use filtered tables if searching, otherwise use all tables
			tablesToShow := m.tables
			if m.search.FilteredTables != nil {
				tablesToShow = m.search.FilteredTables
			}
			
			if selectedIdx >= 0 && selectedIdx < len(tablesToShow) {
				table := tablesToShow[selectedIdx]
				tableID := table.TableID
				if tableID == "" {
					tableID = table.TableReference.TableID
				}

				m.table = tableID
				
				// Clear search state when navigating to table detail
				m.clearSearchState()

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
		m.lastKey = ""
		// Expand/collapse schema nodes
		if m.state == stateTableDetail && len(m.schemaNodes) > 0 {
			node := m.schemaNodes[m.selectedSchema]
			if node.HasChildren {
				m.expandedNodes[node.Path] = !m.expandedNodes[node.Path]
				m.buildSchemaTree() // Rebuild tree with new expansion state
			}
		}

	case "left", "h":
		m.lastKey = ""
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
		m.lastKey = ""
		if m.state == stateTableDetail {
			// Clear search state when navigating back to table list
			m.clearSearchState()
			
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
		
	default:
		// Clear lastKey for any unhandled key
		m.lastKey = ""
	}

	return m, nil
}






// updateTableRows populates the Bubbletea table component with current table data
func (m *browserModel) updateTableRows() {
	// Use filtered tables if searching, otherwise use all tables
	tablesToShow := m.tables
	if m.search.FilteredTables != nil {
		tablesToShow = m.search.FilteredTables
	}
	
	if len(tablesToShow) == 0 {
		m.tableModel.SetRows([]table.Row{})
		return
	}

	rows := make([]table.Row, len(tablesToShow))
	for i, tbl := range tablesToShow {
		tableID := tbl.TableID
		if tableID == "" {
			tableID = tbl.TableReference.TableID
		}

		// Add cache status indicator with color
		cacheStatus := ""
		if _, isCached := m.cachedMetadata[tableID]; isCached {
			cacheStatus = "✓" // Cached - will be colored green in the view
		}

		// Always show basic, fast info - creation time is always available
		created := bigquery.FormatTime(tbl.CreationTime)
		rows[i] = table.Row{tableID, tbl.Type, created, cacheStatus}
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

// copyCurrentTable copies the current table identifier to clipboard
func (m *browserModel) copyCurrentTable() {
	var tableID string
	
	if m.state == stateTableList && len(m.tables) > 0 {
		// Get selected table from table list
		selectedIdx := m.tableModel.Cursor()
		if selectedIdx >= 0 && selectedIdx < len(m.tables) {
			table := m.tables[selectedIdx]
			if table.TableID != "" {
				tableID = table.TableID
			} else {
				tableID = table.TableReference.TableID
			}
			// Build full identifier
			tableID = m.project + "." + m.dataset + "." + tableID
		}
	} else if m.state == stateTableDetail && m.table != "" {
		// Use current table in detail view
		tableID = m.project + "." + m.dataset + "." + m.table
	}
	
	if tableID != "" {
		// Copy to clipboard
		if err := utils.CopyToClipboard(tableID); err != nil {
			m.setStatusMessage("Clipboard not available (install xclip/xsel)")
		} else {
			m.setStatusMessage("Copied: " + tableID)
		}
	}
}

// setStatusMessage sets a temporary status message with timeout
func (m *browserModel) setStatusMessage(message string) {
	m.statusMessage = message
	m.statusTimeout = time.Now().Add(config.StatusMessageTTL)
}

// clearSearchState resets all search-related state
func (m *browserModel) clearSearchState() {
	m.search.Clear()
}

// selectCurrentSearchResult selects the currently highlighted item from search results
// and maps it back to the full list for proper highlighting
func (m *browserModel) selectCurrentSearchResult() {
	if m.state == stateTableList && m.search.FilteredTables != nil && len(m.search.FilteredTables) > 0 {
		// Get the currently selected item from filtered results
		selectedIdx := m.tableModel.Cursor()
		if selectedIdx >= 0 && selectedIdx < len(m.search.FilteredTables) {
			selectedTable := m.search.FilteredTables[selectedIdx]
			
			// Find this table in the full list and set cursor there
			for i, table := range m.tables {
				tableID := table.TableID
				if tableID == "" {
					tableID = table.TableReference.TableID
				}
				selectedTableID := selectedTable.TableID
				if selectedTableID == "" {
					selectedTableID = selectedTable.TableReference.TableID
				}
				
				if tableID == selectedTableID {
					m.tableModel.SetCursor(i)
					break
				}
			}
		}
	} else if m.state == stateTableDetail && m.search.FilteredNodes != nil && len(m.search.FilteredNodes) > 0 {
		// Get the currently selected field from filtered results
		if m.selectedSchema >= 0 && m.selectedSchema < len(m.search.FilteredNodes) {
			selectedNode := m.search.FilteredNodes[m.selectedSchema]
			
			// Find this field in the full schema and set selection there
			for i, node := range m.schemaNodes {
				if node.Path == selectedNode.Path {
					m.selectedSchema = i
					break
				}
			}
		}
	}
}

// handleSearchInput handles keyboard input in search mode
func (m *browserModel) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	
	// Check for escape using multiple methods (robust escape detection)
	if key == "escape" || key == "esc" || msg.Type == tea.KeyEscape {
		// Exit search mode and clear all search state
		m.clearSearchState()
		m.updateTableRows()
		return m, nil
	}
	
	switch key {
	case "enter":
		// fzf-style: select current item and return to full view with selection
		m.selectCurrentSearchResult()
		m.clearSearchState()
		m.updateTableRows()
		return m, nil
		
	case "ctrl+c", "ctrl+g":
		// Exit search mode and clear all search state
		m.clearSearchState()
		m.updateTableRows()
		return m, nil
		
	case "backspace":
		if len(m.search.Query) > 0 {
			m.search.Query = m.search.Query[:len(m.search.Query)-1]
			m.filterTables()
			m.updateTableRows()
		}
		return m, nil
		
	case "up":
		m.handleNavigation("up")
		return m, nil
	case "down":
		m.handleNavigation("down")
		return m, nil
		
	default:
		// Add character to search query
		if len(key) == 1 { // Only single printable characters (including space)
			m.search.Query += key
			m.filterTables()
			m.updateTableRows()
		}
		return m, nil
	}
}

// handleNavigation handles all navigation in a unified way
func (m *browserModel) handleNavigation(direction string) {
	switch direction {
	case "up":
		if m.state == stateTableDetail && m.selectedSchema > 0 {
			m.selectedSchema--
		}
		// For table list, navigation is handled by the table model automatically
		
	case "down":
		if m.state == stateTableDetail {
			// Use filtered nodes count if searching, otherwise use all nodes
			maxNodes := len(m.schemaNodes)
			if m.search.FilteredNodes != nil {
				maxNodes = len(m.search.FilteredNodes)
			}
			if m.selectedSchema < maxNodes-1 {
				m.selectedSchema++
			}
		}
		// For table list, navigation is handled by the table model automatically
		
	case "top":
		if m.state == stateTableList {
			m.tableModel.GotoTop()
		} else if m.state == stateTableDetail {
			m.selectedSchema = 0
		}
		
	case "bottom":
		if m.state == stateTableList {
			m.tableModel.GotoBottom()
		} else if m.state == stateTableDetail {
			maxNodes := len(m.schemaNodes)
			if m.search.FilteredNodes != nil {
				maxNodes = len(m.search.FilteredNodes)
			}
			if maxNodes > 0 {
				m.selectedSchema = maxNodes - 1
			}
		}
	}
}

// filterTables filters the table list based on the search query
func (m *browserModel) filterTables() {
	if m.search.Query == "" {
		m.search.FilteredTables = nil
		m.search.FilteredNodes = nil
		return
	}
	
	query := strings.ToLower(m.search.Query)
	
	// Filter tables if in table list view
	if m.search.Context == SearchTables && len(m.tables) > 0 {
		m.search.FilteredTables = make([]bigquery.TableInfo, 0)
		for _, table := range m.tables {
			tableID := table.TableID
			if tableID == "" {
				tableID = table.TableReference.TableID
			}
			
			// Simple substring search (case-insensitive)
			if strings.Contains(strings.ToLower(tableID), query) {
				m.search.FilteredTables = append(m.search.FilteredTables, table)
			}
		}
	}
	
	// Filter schema nodes if in table detail view
	if m.search.Context == SearchSchema && len(m.schemaNodes) > 0 {
		m.search.FilteredNodes = make([]schemaNode, 0)
		for _, node := range m.schemaNodes {
			// Search in field name and field type
			fieldName := strings.ToLower(node.Field.Name)
			fieldType := strings.ToLower(node.Field.Type)
			
			if strings.Contains(fieldName, query) || strings.Contains(fieldType, query) {
				m.search.FilteredNodes = append(m.search.FilteredNodes, node)
			}
		}
	}
}

// handleCommandInput handles keyboard input in command mode
func (m *browserModel) handleCommandInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	
	// Check for escape using multiple methods (robust escape detection)
	if key == "escape" || key == "esc" || msg.Type == tea.KeyEscape {
		// Cancel command mode and clear command query
		m.commandMode = false
		m.commandQuery = ""
		return m, nil
	}
	
	switch key {
	case "enter":
		// Execute command
		m.commandMode = false
		m.executeCommand(m.commandQuery)
		m.commandQuery = ""
		return m, nil
		
	case "ctrl+c", "ctrl+g":
		// Cancel command
		m.commandMode = false
		m.commandQuery = ""
		return m, nil
		
	case "backspace":
		if len(m.commandQuery) > 0 {
			m.commandQuery = m.commandQuery[:len(m.commandQuery)-1]
		}
		return m, nil
		
	default:
		// Add character to command query
		if len(key) == 1 { // Only single printable characters
			m.commandQuery += key
		}
		return m, nil
	}
}

// executeCommand executes the entered command
func (m *browserModel) executeCommand(command string) {
	switch command {
	case "q", "quit":
		// Note: This won't work in the model, we'd need to return tea.Quit
		m.setStatusMessage("Use 'q' key to quit")
	case "copy":
		m.copyCurrentTable()
	case "help", "h":
		m.setStatusMessage("Available commands: copy, quit, help")
	case "":
		// Empty command, do nothing
		return
	default:
		m.setStatusMessage(fmt.Sprintf("Unknown command: '%s'. Available: copy, quit, help", command))
	}
}

