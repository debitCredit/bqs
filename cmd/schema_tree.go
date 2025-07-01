package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"bqs/internal/bigquery"
)

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

// renderSchemaTree renders the schema tree with proper styling
func (m *browserModel) renderSchemaTree() string {
	var content strings.Builder

	for i, node := range m.schemaNodes {
		selected := i == m.selectedSchema

		var style lipgloss.Style
		if selected {
			style = lipgloss.NewStyle().
				Background(selectedBg).
				Foreground(selectedFg).
				Padding(0, 1).
				Bold(true)
		} else {
			style = lipgloss.NewStyle().Padding(0, 1)
		}

		// Build indentation
		indent := strings.Repeat("  ", node.Level)

		// Build tree connector
		connector := "├─"

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

		// Build mode indicator with enhanced colors
		mode := ""
		if node.Field.Mode == "REQUIRED" {
			mode = lipgloss.NewStyle().Foreground(primaryRed).Bold(true).Render(" REQUIRED")
		} else if node.Field.Mode == "REPEATED" {
			mode = lipgloss.NewStyle().Foreground(accentOrange).Bold(true).Render(" REPEATED")
		}

		// Build type with enhanced color
		var typeColor lipgloss.Color
		switch node.Field.Type {
		case "STRING", "BYTES":
			typeColor = primaryGreen
		case "INTEGER", "INT64", "FLOAT", "FLOAT64", "NUMERIC", "BIGNUMERIC":
			typeColor = primaryBlue
		case "BOOLEAN":
			typeColor = primaryYellow
		case "TIMESTAMP", "DATE", "TIME", "DATETIME":
			typeColor = accentCyan
		case "RECORD", "STRUCT":
			typeColor = accentPurple
		default:
			typeColor = accentPurple
		}
		typeStyle := lipgloss.NewStyle().Foreground(typeColor).Bold(true).Render(node.Field.Type)

		line := fmt.Sprintf("%s%s%s%s %s%s", indent, connector, expandIcon, node.Field.Name, typeStyle, mode)
		content.WriteString(style.Render(line))
		content.WriteString("\n")
	}

	return content.String()
}