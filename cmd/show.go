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
	noCache          bool
)

var showCmd = &cobra.Command{
	Use:   "show [flags] <project.dataset.table>",
	Short: "Show BigQuery table or view metadata",
	Long: `Display metadata for BigQuery tables, views, and materialized views.

Supports all major bq show functionality with enhanced usability.

Common usage:
  bqs show project.dataset.table              # Complete metadata (prettyjson)
  bqs show -s project.dataset.table           # Schema only
  bqs show -v project.dataset.view            # View with SQL definition
  bqs show -f json project.dataset.table      # Compact JSON format
  bqs show -s -f pretty project.dataset.table # Schema in table format
  bqs show -p other-project dataset.table     # Cross-project access`,
	Args: cobra.ExactArgs(1),
	RunE: runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
	
	// Resource type flags with short versions
	showCmd.Flags().BoolVarP(&schemaOnly, "schema", "s", false, "Show only the schema")
	showCmd.Flags().BoolVarP(&viewDetails, "view", "v", false, "Show view-specific details including SQL definition")
	showCmd.Flags().BoolVar(&materializedView, "materialized-view", false, "Show materialized view details including refresh policies")
	
	// Output format flags with short version
	showCmd.Flags().StringVarP(&formatFlag, "format", "f", "prettyjson", "Output format: json, prettyjson, pretty, sparse, csv")
	
	// Override flags
	showCmd.Flags().StringVarP(&projectOverride, "project", "p", "", "Override project ID for cross-project access")
	showCmd.Flags().BoolVarP(&quietMode, "quiet", "q", false, "Suppress status updates")
	showCmd.Flags().BoolVar(&noCache, "no-cache", false, "Bypass cache and fetch fresh data")
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