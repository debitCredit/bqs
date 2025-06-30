package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "bqs",
	Short: "BigQuery Schema Tool",
	Long: `BQS is a fast, lightweight CLI tool for BigQuery metadata inspection and schema operations.

It provides a user-friendly wrapper around the 'bq show' command with enhanced
usability features while maintaining full compatibility with BigQuery's native tooling.

Common usage:
  bqs show project.dataset.table           # Show complete table metadata
  bqs show -s project.dataset.table        # Show schema only
  bqs show -v project.dataset.view         # Show view with SQL definition
  bqs show -f json project.dataset.table   # Output in compact JSON format

For more information, visit: https://github.com/debitCredit/bqs`,
	Version: "1.0.0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}