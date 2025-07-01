package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"bqs/internal/bigquery"
)

// Color palette for consistent theming
var (
	// Primary colors
	primaryBlue    = lipgloss.Color("39")   // Bright blue for headers
	primaryGreen   = lipgloss.Color("82")   // Success/cached indicators
	primaryYellow  = lipgloss.Color("220")  // Status messages
	primaryRed     = lipgloss.Color("196")  // Errors/required fields

	// Secondary colors
	secondaryGray  = lipgloss.Color("244")  // Metadata text
	lightGray      = lipgloss.Color("248")  // Table types
	darkGray       = lipgloss.Color("240")  // Borders
	footerGray     = lipgloss.Color("241")  // Footer text

	// Accent colors
	accentCyan     = lipgloss.Color("86")   // Project/dataset names
	accentPurple   = lipgloss.Color("135")  // Schema field types
	accentOrange   = lipgloss.Color("208")  // Repeated fields

	// Background colors
	selectedBg     = lipgloss.Color("62")   // Selected item background
	selectedFg     = lipgloss.Color("230")  // Selected item foreground

	// Cache status colors
	cachedColor    = primaryGreen
	loadingColor   = primaryYellow
)

func (m *browserModel) renderLoading() string {
	loadingStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(primaryBlue).
		Bold(true)

	spinner := "🔄"
	content := fmt.Sprintf("%s Loading BigQuery metadata...", spinner)
	return loadingStyle.Render(content)
}

func (m *browserModel) renderTableList() string {
	var content strings.Builder

	// Header with enhanced styling
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryBlue).
		Padding(0, 1).
		MarginBottom(1)

	// Project.dataset with color hierarchy
	projectStyle := lipgloss.NewStyle().Foreground(accentCyan)
	datasetStyle := lipgloss.NewStyle().Foreground(primaryBlue).Bold(true)

	headerText := fmt.Sprintf("📊 %s.%s", 
		projectStyle.Render(m.project),
		datasetStyle.Render(m.dataset))
	content.WriteString(headerStyle.Render(headerText))
	content.WriteString("\n\n")

	// Table list with enhanced styling
	if m.loading {
		// Show loading with style
		loadingStyle := lipgloss.NewStyle().
			Foreground(loadingColor).
			Bold(true).
			Padding(2, 4)
		content.WriteString(loadingStyle.Render("🔄 Loading tables..."))
	} else if len(m.tables) == 0 {
		// Show empty state with style
		emptyStyle := lipgloss.NewStyle().
			Foreground(secondaryGray).
			Italic(true).
			Padding(2, 4)
		content.WriteString(emptyStyle.Render("📋 No tables found in this dataset"))
	} else {
		// Render the table component
		content.WriteString(m.tableModel.View())
	}

	// Status message with enhanced styling
	if m.statusMessage != "" {
		content.WriteString("\n")
		statusStyle := lipgloss.NewStyle().
			Foreground(primaryYellow).
			Bold(true).
			Padding(0, 1).
			MarginTop(1).
			Background(lipgloss.Color("237")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryYellow)
		content.WriteString(statusStyle.Render(fmt.Sprintf("ℹ️  %s", m.statusMessage)))
	}

	// Footer with enhanced styling
	content.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(footerGray).
		Padding(1, 1).
		MarginTop(1).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(darkGray)

	// Color-coded shortcuts
	navKeys := lipgloss.NewStyle().Foreground(primaryBlue).Render("[hjkl/↑↓]")
	actionKeys := lipgloss.NewStyle().Foreground(primaryGreen).Render("[Enter]")
	copyKeys := lipgloss.NewStyle().Foreground(primaryYellow).Render("[yy]")
	quitKeys := lipgloss.NewStyle().Foreground(primaryRed).Render("[q]")
	cachedIcon := lipgloss.NewStyle().Foreground(cachedColor).Render("✓")

	footer := fmt.Sprintf("⌨️  %s Navigate • %s Explore • %s Copy • %s Quit • %s = Cached",
		navKeys, actionKeys, copyKeys, quitKeys, cachedIcon)
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

	// Header with enhanced styling
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryBlue).
		Padding(0, 1).
		MarginBottom(1)

	icon := bigquery.GetTableTypeIcon(m.metadata.Type)
	// Project.dataset.table with color hierarchy
	projectStyle := lipgloss.NewStyle().Foreground(accentCyan)
	datasetStyle := lipgloss.NewStyle().Foreground(primaryBlue)
	tableStyle := lipgloss.NewStyle().Foreground(primaryGreen).Bold(true)

	headerText := fmt.Sprintf("%s %s.%s.%s", icon,
		projectStyle.Render(m.project),
		datasetStyle.Render(m.dataset),
		tableStyle.Render(m.table))
	content.WriteString(headerStyle.Render(headerText))
	content.WriteString("\n\n")

	// Metadata with enhanced styling
	metaStyle := lipgloss.NewStyle().
		Foreground(secondaryGray).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(darkGray).
		Padding(1, 2)

	size := bigquery.FormatSize(m.metadata.NumBytes)
	lastMod := bigquery.FormatTime(m.metadata.LastModifiedTime)

	// Color-coded metadata elements
	rowsStyle := lipgloss.NewStyle().Foreground(primaryBlue).Bold(true)
	sizeStyle := lipgloss.NewStyle().Foreground(primaryGreen)
	timeStyle := lipgloss.NewStyle().Foreground(accentCyan)

	meta := fmt.Sprintf("📊 %s rows • 💾 %s • 🕒 Modified %s",
		rowsStyle.Render(fmt.Sprintf("%d", m.metadata.NumRows)),
		sizeStyle.Render(size),
		timeStyle.Render(lastMod))
	content.WriteString(metaStyle.Render(meta))
	content.WriteString("\n\n")

	// Schema with enhanced styling
	if m.metadata.Schema != nil && len(m.schemaNodes) > 0 {
		schemaStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryBlue).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

		content.WriteString(schemaStyle.Render("🌲 Schema:"))
		content.WriteString("\n\n")

		content.WriteString(m.renderSchemaTree())
	}

	// Status message with enhanced styling
	if m.statusMessage != "" {
		content.WriteString("\n")
		statusStyle := lipgloss.NewStyle().
			Foreground(primaryYellow).
			Bold(true).
			Padding(0, 1).
			MarginTop(1).
			Background(lipgloss.Color("237")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryYellow)
		content.WriteString(statusStyle.Render(fmt.Sprintf("ℹ️  %s", m.statusMessage)))
	}

	// Footer with enhanced styling
	content.WriteString("\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(footerGray).
		Padding(1, 1).
		MarginTop(1).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(darkGray)

	// Color-coded shortcuts for table detail
	navKeys := lipgloss.NewStyle().Foreground(primaryBlue).Render("[hjkl/↑↓]")
	expandKeys := lipgloss.NewStyle().Foreground(primaryGreen).Render("[Space/→]")
	collapseKeys := lipgloss.NewStyle().Foreground(accentOrange).Render("[←]")
	copyKeys := lipgloss.NewStyle().Foreground(primaryYellow).Render("[yy]")
	backKeys := lipgloss.NewStyle().Foreground(accentCyan).Render("[b]")
	quitKeys := lipgloss.NewStyle().Foreground(primaryRed).Render("[q]")

	footer := fmt.Sprintf("⌨️  %s Navigate • %s Expand • %s Collapse • %s Copy • %s Back • %s Quit",
		navKeys, expandKeys, collapseKeys, copyKeys, backKeys, quitKeys)
	content.WriteString(footerStyle.Render(footer))

	return content.String()
}

func (m *browserModel) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(primaryRed).
		Bold(true).
		Padding(2, 4).
		Margin(2, 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryRed).
		Background(lipgloss.Color("237"))

	quitKey := lipgloss.NewStyle().Foreground(primaryYellow).Bold(true).Render("[q]")
	errorText := fmt.Sprintf("❌ Error: %s\n\nPress %s to quit", m.err.Error(), quitKey)
	
	return errorStyle.Render(errorText)
}

func (m *browserModel) renderHelp() string {
	// Create help content based on the previous state
	var helpContent strings.Builder
	
	// Main help container
	helpStyle := lipgloss.NewStyle().
		Padding(2, 4).
		Margin(2, 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryBlue).
		Background(lipgloss.Color("235")).
		Width(m.width - 16)

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryBlue).
		Align(lipgloss.Center).
		Width(m.width - 24)
	
	helpContent.WriteString(titleStyle.Render("🆘 BQS Help"))
	helpContent.WriteString("\n\n")

	// Context-sensitive shortcuts
	if m.previousState == stateTableList {
		helpContent.WriteString(m.renderTableListHelp())
	} else if m.previousState == stateTableDetail {
		helpContent.WriteString(m.renderTableDetailHelp())
	}

	// Universal shortcuts
	helpContent.WriteString("\n")
	universalStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentCyan).
		MarginTop(1)
	helpContent.WriteString(universalStyle.Render("Universal Commands:"))
	helpContent.WriteString("\n")
	helpContent.WriteString(m.renderUniversalHelp())

	// Footer
	helpContent.WriteString("\n\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(secondaryGray).
		Italic(true).
		Align(lipgloss.Center).
		Width(m.width - 24)
	helpContent.WriteString(footerStyle.Render("Press ? or Esc to close help"))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, 
		helpStyle.Render(helpContent.String()))
}

func (m *browserModel) renderTableListHelp() string {
	var content strings.Builder
	
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryGreen).
		MarginBottom(1)
	content.WriteString(sectionStyle.Render("Table List Navigation:"))
	content.WriteString("\n")

	shortcuts := [][]string{
		{"hjkl, ↑↓", "Navigate table list"},
		{"gg", "Jump to top"},
		{"G", "Jump to bottom"},
		{"Enter", "Explore selected table"},
		{"yy", "Copy table identifier"},
	}

	for _, shortcut := range shortcuts {
		keyStyle := lipgloss.NewStyle().Foreground(primaryYellow).Bold(true)
		descStyle := lipgloss.NewStyle().Foreground(lightGray)
		content.WriteString(fmt.Sprintf("  %s  %s\n", 
			keyStyle.Render(fmt.Sprintf("%-8s", shortcut[0])),
			descStyle.Render(shortcut[1])))
	}

	return content.String()
}

func (m *browserModel) renderTableDetailHelp() string {
	var content strings.Builder
	
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryGreen).
		MarginBottom(1)
	content.WriteString(sectionStyle.Render("Schema Navigation:"))
	content.WriteString("\n")

	shortcuts := [][]string{
		{"hjkl, ↑↓", "Navigate schema fields"},
		{"gg", "Jump to top"},
		{"G", "Jump to bottom"},
		{"Space, →", "Expand field"},
		{"←, h", "Collapse field"},
		{"yy", "Copy table identifier"},
		{"b", "Back to table list"},
	}

	for _, shortcut := range shortcuts {
		keyStyle := lipgloss.NewStyle().Foreground(primaryYellow).Bold(true)
		descStyle := lipgloss.NewStyle().Foreground(lightGray)
		content.WriteString(fmt.Sprintf("  %s  %s\n", 
			keyStyle.Render(fmt.Sprintf("%-8s", shortcut[0])),
			descStyle.Render(shortcut[1])))
	}

	return content.String()
}

func (m *browserModel) renderUniversalHelp() string {
	var content strings.Builder

	shortcuts := [][]string{
		{"?", "Toggle this help"},
		{"q, Ctrl+C", "Quit application"},
		{"Esc", "Close help/go back"},
	}

	for _, shortcut := range shortcuts {
		keyStyle := lipgloss.NewStyle().Foreground(primaryRed).Bold(true)
		descStyle := lipgloss.NewStyle().Foreground(lightGray)
		content.WriteString(fmt.Sprintf("  %s  %s\n", 
			keyStyle.Render(fmt.Sprintf("%-8s", shortcut[0])),
			descStyle.Render(shortcut[1])))
	}

	return content.String()
}