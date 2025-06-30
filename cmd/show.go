package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	schemaOnly       bool
	viewDetails      bool
	materializedView bool
	formatFlag       string
	projectOverride  string
	quietMode        bool
)

var showCmd = &cobra.Command{
	Use:   "show <project.dataset.table>",
	Short: "Show BigQuery table or view metadata",
	Long: `Display metadata for BigQuery tables, views, and materialized views.

Supports all major bq show functionality with enhanced usability.
Use --schema for schema-only output, --view for view details,
and --format to control output formatting.`,
	Args: cobra.ExactArgs(1),
	RunE: runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
	
	// Resource type flags
	showCmd.Flags().BoolVar(&schemaOnly, "schema", false, "Show only the schema")
	showCmd.Flags().BoolVar(&viewDetails, "view", false, "Show view-specific details")
	showCmd.Flags().BoolVar(&materializedView, "materialized-view", false, "Show materialized view details")
	
	// Output format flags
	showCmd.Flags().StringVar(&formatFlag, "format", "prettyjson", "Output format: json, prettyjson, pretty, sparse, csv")
	
	// Override flags
	showCmd.Flags().StringVar(&projectOverride, "project", "", "Override project ID")
	showCmd.Flags().BoolVarP(&quietMode, "quiet", "q", false, "Suppress status updates")
}

func runShow(cmd *cobra.Command, args []string) error {
	fullTableID := args[0]
	
	parts := strings.Split(fullTableID, ".")
	if len(parts) < 3 {
		return fmt.Errorf("invalid table format: expected project.dataset.table, got %s", fullTableID)
	}
	
	projectID := parts[0]
	if projectOverride != "" {
		projectID = projectOverride
	}
	
	datasetTableID := strings.Join(parts[1:], ".")
	
	return showBQTable(projectID, datasetTableID)
}

func showBQTable(projectID, datasetTableID string) error {
	args := []string{"show"}
	
	// Add project ID
	args = append(args, "--project_id="+projectID)
	
	// Add resource type flags
	if schemaOnly {
		args = append(args, "--schema")
	}
	if viewDetails {
		args = append(args, "--view")
	}
	if materializedView {
		args = append(args, "--materialized_view")
	}
	
	// Add format flag
	args = append(args, "--format="+formatFlag)
	
	// Add quiet flag
	if quietMode {
		args = append(args, "--quiet")
	}
	
	// Add the table identifier
	args = append(args, datasetTableID)
	
	cmd := exec.Command("bq", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}