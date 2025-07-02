package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
)

// KeyHandler interface for handling specific key combinations
type KeyHandler interface {
	HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd)
}

// KeyDispatcher handles key routing based on current state and mode
type KeyDispatcher struct {
	handlers map[string]KeyHandler
}

// NewKeyDispatcher creates a new key dispatcher with all handlers
func NewKeyDispatcher() *KeyDispatcher {
	return &KeyDispatcher{
		handlers: map[string]KeyHandler{
			"q":        &quitHandler{},
			"ctrl+c":   &quitHandler{},
			"?":        &helpHandler{},
			"/":        &searchHandler{},
			"escape":   &escapeHandler{},
			"g":        &navigationHandler{key: "g"},
			"G":        &navigationHandler{key: "G"},
			"y":        &yankHandler{},
			"e":        &exportHandler{},
			"up":       &navigationHandler{key: "up"},
			"k":        &navigationHandler{key: "up"},
			"down":     &navigationHandler{key: "down"},
			"j":        &navigationHandler{key: "down"},
			"enter":    &enterHandler{},
			"space":    &expandHandler{},
			"right":    &expandHandler{},
			"l":        &expandHandler{},
			"left":     &collapseHandler{},
			"h":        &collapseHandler{},
			"b":        &backHandler{},
		},
	}
}

// Dispatch handles a key press by routing to the appropriate handler
func (kd *KeyDispatcher) Dispatch(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	
	// Handle special UI modes first
	if m.ui.IsSearchMode() {
		return m.handleSearchInput(msg)
	}
	
	
	// Handle help mode - only allow certain keys
	if m.state == stateHelp {
		switch key {
		case "q", "ctrl+c", "?", "escape":
			// These keys work in help mode - continue to handlers
		default:
			// All other keys in help mode hide help
			m.state = m.previousState
			m.lastKey = ""
			return m, nil
		}
	}
	
	// Dispatch to specific handler
	if handler, exists := kd.handlers[key]; exists {
		return handler.HandleKey(m, msg)
	}
	
	// No specific handler - clear lastKey for any unhandled key
	m.lastKey = ""
	return m, nil
}

// quitHandler handles quit operations
type quitHandler struct{}

func (h *quitHandler) HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.lastKey = ""
	return m, tea.Quit
}

// helpHandler toggles help display
type helpHandler struct{}

func (h *helpHandler) HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.state == stateHelp {
		// Hide help overlay
		m.state = m.previousState
	} else {
		// Show help overlay
		m.previousState = m.state
		m.state = stateHelp
	}
	m.lastKey = ""
	return m, nil
}

// searchHandler initiates search mode
type searchHandler struct{}

func (h *searchHandler) HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Enter search mode (for table list and table detail)
	if (m.state == stateTableList || m.state == stateTableDetail) && m.ui.IsNormalMode() {
		if m.state == stateTableList {
			m.ui.EnterSearchMode(SearchTables)
		} else {
			m.ui.EnterSearchMode(SearchSchema)
		}
		m.lastKey = ""
		return m, nil
	}
	m.lastKey = ""
	return m, nil
}


// escapeHandler handles escape key
type escapeHandler struct{}

func (h *escapeHandler) HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.state == stateHelp {
		// Hide help overlay
		m.state = m.previousState
		m.lastKey = ""
		return m, nil
	}
	// Note: search and command mode escapes are handled in their respective input handlers
	m.lastKey = ""
	return m, nil
}

// navigationHandler handles navigation keys including vim-style sequences
type navigationHandler struct {
	key string
}

func (h *navigationHandler) HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch h.key {
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
		return m, nil
	case "up":
		m.handleNavigation("up")
		m.lastKey = ""
		return m, nil
	case "down":
		m.handleNavigation("down")
		m.lastKey = ""
		return m, nil
	}
	
	m.lastKey = ""
	return m, nil
}

// yankHandler handles copy operations (yy sequence)
type yankHandler struct{}

func (h *yankHandler) HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.lastKey == "y" { // yy sequence - copy table identifier
		m.copyCurrentTable()
		m.lastKey = ""
		return m, nil
	}
	m.lastKey = "y"
	return m, nil
}

// exportHandler handles export operations
type exportHandler struct{}

func (h *exportHandler) HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Export table metadata
	m.lastKey = ""
	return m.exportTable()
}

// enterHandler handles enter key
type enterHandler struct{}

func (h *enterHandler) HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.lastKey = ""
	if m.state == stateTableList && len(m.tables) > 0 {
		// Get selected table from the table model cursor
		selectedIdx := m.tableModel.Cursor()
		
		// Use filtered tables if searching, otherwise use all tables
		tablesToShow := m.tables
		if m.ui.Search.FilteredTables != nil {
			tablesToShow = m.ui.Search.FilteredTables
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
	return m, nil
}

// expandHandler handles expand operations (space, right, l)
type expandHandler struct{}

func (h *expandHandler) HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.lastKey = ""
	// Expand/collapse schema nodes
	if m.state == stateTableDetail && len(m.schemaNodes) > 0 {
		node := m.schemaNodes[m.selectedSchema]
		if node.HasChildren {
			m.expandedNodes[node.Path] = !m.expandedNodes[node.Path]
			m.buildSchemaTree() // Rebuild tree with new expansion state
		}
	}
	return m, nil
}

// collapseHandler handles collapse operations (left, h)
type collapseHandler struct{}

func (h *collapseHandler) HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.lastKey = ""
	// Collapse current node or go to parent
	if m.state == stateTableDetail && len(m.schemaNodes) > 0 {
		node := m.schemaNodes[m.selectedSchema]
		if node.HasChildren && m.expandedNodes[node.Path] {
			// If current node is expanded, collapse it
			m.expandedNodes[node.Path] = false
			m.buildSchemaTree()
		}
		// TODO: Could add logic to jump to parent node
	}
	return m, nil
}

// backHandler handles back navigation (b key)
type backHandler struct{}

func (h *backHandler) HandleKey(m *browserModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.lastKey = ""
	if m.state == stateTableDetail {
		// Clear search state when going back to table list
		m.clearSearchState()
		m.state = stateTableList
		m.table = ""
		m.metadata = nil
		m.schemaNodes = nil
		m.selectedSchema = 0
	}
	return m, nil
}