package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docsCmd = &cobra.Command{
	Use:    "docs",
	Short:  "Generate documentation",
	Long:   `Generate man pages and other documentation for bqs.`,
	Hidden: true,
	RunE:   generateDocs,
}

var (
	docsOutputDir string
	docsFormat    string
)

func init() {
	rootCmd.AddCommand(docsCmd)
	
	docsCmd.Flags().StringVar(&docsOutputDir, "output", "./docs", "Output directory for documentation")
	docsCmd.Flags().StringVar(&docsFormat, "format", "man", "Documentation format: man, md, yaml")
}

func generateDocs(cmd *cobra.Command, args []string) error {
	// Ensure output directory exists
	if err := os.MkdirAll(docsOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	switch docsFormat {
	case "man":
		header := &doc.GenManHeader{
			Title:   "BQS",
			Section: "1",
			Source:  "bqs",
			Manual:  "BigQuery Schema Tool Manual",
		}
		return doc.GenManTree(rootCmd, header, docsOutputDir)
	case "md":
		return doc.GenMarkdownTree(rootCmd, docsOutputDir)
	case "yaml":
		return doc.GenYamlTree(rootCmd, docsOutputDir)
	default:
		return fmt.Errorf("unsupported format: %s (supported: man, md, yaml)", docsFormat)
	}
}