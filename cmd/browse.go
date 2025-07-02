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
	"bqs/internal/errors"
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
		if bqsErr := errors.WrapValidationError(err, input); bqsErr != nil {
			return fmt.Errorf("%s", bqsErr.UserFriendlyMessage())
		}
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
		if cacheErr := errors.WrapCacheError(err, "initialize"); cacheErr != nil {
			return fmt.Errorf("%s", cacheErr.UserFriendlyMessage())
		}
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
		keyDispatcher:  NewKeyDispatcher(),
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

	case exportCompletedMsg:
		if msg.success {
			m.setStatusMessage(fmt.Sprintf("✓ Copied %s metadata to clipboard", msg.tableID))
			
			// Cache the metadata if it was fetched (only happens from dataset level export)
			if msg.metadata != nil && m.state == stateTableList {
				m.cachedMetadata[msg.tableID] = msg.metadata
				// Update table rows to show the new cache status
				if len(m.tables) > 0 {
					m.updateTableRows()
				}
			}
		} else {
			// Display user-friendly error message with retry hint
			errorMessage := msg.error
			if msg.retryable {
				errorMessage += " - try again in a moment"
			}
			m.setStatusMessage(fmt.Sprintf("✗ %s", errorMessage))
		}
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
	// Delegate all key handling to the key dispatcher
	return m.keyDispatcher.Dispatch(m, msg)
}







// updateTableRows populates the Bubbletea table component with current table data
func (m *browserModel) updateTableRows() {
	// Use filtered tables if searching, otherwise use all tables
	tablesToShow := m.tables
	if m.ui.Search.FilteredTables != nil {
		tablesToShow = m.ui.Search.FilteredTables
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

// exportTable initiates async export of the selected table's metadata
func (m *browserModel) exportTable() (tea.Model, tea.Cmd) {
	var tableID string
	var tableMetadata *bigquery.TableMetadata
	
	// Determine which table to export based on current state
	if m.state == stateTableList && len(m.tables) > 0 {
		// Dataset level: export selected table
		selectedIdx := m.tableModel.Cursor()
		if selectedIdx >= 0 && selectedIdx < len(m.tables) {
			table := m.tables[selectedIdx]
			if table.TableID != "" {
				tableID = table.TableID
			} else {
				tableID = table.TableReference.TableID
			}
		} else {
			m.setStatusMessage("No table selected")
			return m, nil
		}
	} else if m.state == stateTableDetail && m.table != "" {
		// Table detail level: export current table
		tableID = m.table
		tableMetadata = m.metadata
	} else {
		m.setStatusMessage("Export only available when viewing tables")
		return m, nil
	}

	if tableID == "" {
		m.setStatusMessage("No table available to export")
		return m, nil
	}

	// Show immediate feedback
	if tableMetadata != nil {
		// We have metadata already (table detail view) - export will be fast
		m.setStatusMessage(fmt.Sprintf("Copying %s metadata to clipboard...", tableID))
	} else {
		// Need to fetch metadata (dataset level) - might take a moment
		m.setStatusMessage(fmt.Sprintf("Fetching and copying %s metadata...", tableID))
	}

	// Start async export
	return m, exportTableMetadata(m.client, m.project, m.dataset, tableID, tableMetadata)
}

// clearSearchState resets all search-related state
func (m *browserModel) clearSearchState() {
	m.ui.ExitSpecialMode()
}

// selectCurrentSearchResult selects the currently highlighted item from search results
// and maps it back to the full list for proper highlighting
func (m *browserModel) selectCurrentSearchResult() {
	if m.state == stateTableList && m.ui.Search.FilteredTables != nil && len(m.ui.Search.FilteredTables) > 0 {
		// Get the currently selected item from filtered results
		selectedIdx := m.tableModel.Cursor()
		if selectedIdx >= 0 && selectedIdx < len(m.ui.Search.FilteredTables) {
			selectedTable := m.ui.Search.FilteredTables[selectedIdx]
			
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
	} else if m.state == stateTableDetail && m.ui.Search.FilteredNodes != nil && len(m.ui.Search.FilteredNodes) > 0 {
		// Get the currently selected field from filtered results
		if m.selectedSchema >= 0 && m.selectedSchema < len(m.ui.Search.FilteredNodes) {
			selectedNode := m.ui.Search.FilteredNodes[m.selectedSchema]
			
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
		if len(m.ui.Search.Query) > 0 {
			m.ui.Search.Query = m.ui.Search.Query[:len(m.ui.Search.Query)-1]
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
			m.ui.Search.Query += key
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
			if m.ui.Search.FilteredNodes != nil {
				maxNodes = len(m.ui.Search.FilteredNodes)
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
			if m.ui.Search.FilteredNodes != nil {
				maxNodes = len(m.ui.Search.FilteredNodes)
			}
			if maxNodes > 0 {
				m.selectedSchema = maxNodes - 1
			}
		}
	}
}

// filterTables filters the table list based on the search query
func (m *browserModel) filterTables() {
	if m.ui.Search.Query == "" {
		m.ui.Search.FilteredTables = nil
		m.ui.Search.FilteredNodes = nil
		return
	}
	
	query := strings.ToLower(m.ui.Search.Query)
	
	// Filter tables if in table list view
	if m.ui.Search.Context == SearchTables && len(m.tables) > 0 {
		m.ui.Search.FilteredTables = make([]bigquery.TableInfo, 0)
		for _, table := range m.tables {
			tableID := table.TableID
			if tableID == "" {
				tableID = table.TableReference.TableID
			}
			
			// Simple substring search (case-insensitive)
			if strings.Contains(strings.ToLower(tableID), query) {
				m.ui.Search.FilteredTables = append(m.ui.Search.FilteredTables, table)
			}
		}
	}
	
	// Filter schema nodes if in table detail view
	if m.ui.Search.Context == SearchSchema && len(m.schemaNodes) > 0 {
		m.ui.Search.FilteredNodes = make([]schemaNode, 0)
		for _, node := range m.schemaNodes {
			// Search in field name and field type
			fieldName := strings.ToLower(node.Field.Name)
			fieldType := strings.ToLower(node.Field.Type)
			
			if strings.Contains(fieldName, query) || strings.Contains(fieldType, query) {
				m.ui.Search.FilteredNodes = append(m.ui.Search.FilteredNodes, node)
			}
		}
	}
}


