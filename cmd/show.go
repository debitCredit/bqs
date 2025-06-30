package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <project.dataset.table>",
	Short: "Show BigQuery table metadata",
	Long:  `Display table metadata in JSON format for the specified BigQuery table.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	fullTableID := args[0]
	
	parts := strings.Split(fullTableID, ".")
	if len(parts) < 3 {
		return fmt.Errorf("invalid table format: expected project.dataset.table, got %s", fullTableID)
	}
	
	projectID := parts[0]
	datasetTableID := strings.Join(parts[1:], ".")
	
	return showBQTable(projectID, datasetTableID)
}

func showBQTable(projectID, datasetTableID string) error {
	cmd := exec.Command("bq", "show", "--project_id="+projectID, "--format=prettyjson", datasetTableID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}