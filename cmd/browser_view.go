package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"bqs/internal/bigquery"
)

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

		content.WriteString(m.renderSchemaTree())
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