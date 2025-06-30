package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"bqs/internal/cache"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage BigQuery metadata cache",
	Long: `Manage the local cache of BigQuery metadata to reduce API calls and improve performance.

The cache stores table lists, schemas, and metadata locally with configurable TTL.
This significantly reduces BigQuery API calls and associated costs.`,
}

var cacheStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache statistics",
	Long:  `Display information about the cache including size, entry count, and expiration details.`,
	RunE:  runCacheStats,
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all cached data",
	Long:  `Remove all cached BigQuery metadata. This will force fresh API calls on next use.`,
	RunE:  runCacheClear,
}

var cacheCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove expired cache entries",
	Long:  `Clean up expired cache entries to reclaim disk space. This is done automatically but can be run manually.`,
	RunE:  runCacheCleanup,
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheStatsCmd)
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheCleanupCmd)
}

func runCacheStats(cmd *cobra.Command, args []string) error {
	c, err := cache.New(15 * time.Minute)
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}
	defer c.Close()

	stats, err := c.Stats()
	if err != nil {
		return fmt.Errorf("failed to get cache stats: %w", err)
	}

	fmt.Printf("Cache Statistics:\n")
	fmt.Printf("  Total entries:   %d\n", stats.TotalEntries)
	fmt.Printf("  Valid entries:   %d\n", stats.ValidEntries)
	fmt.Printf("  Expired entries: %d\n", stats.ExpiredEntries)
	fmt.Printf("  Database size:   %s\n", formatBytes(stats.SizeBytes))

	if stats.TotalEntries > 0 {
		fmt.Printf("  Hit rate:        %.1f%%\n", float64(stats.ValidEntries)/float64(stats.TotalEntries)*100)
	}

	return nil
}

func runCacheClear(cmd *cobra.Command, args []string) error {
	c, err := cache.New(15 * time.Minute)
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}
	defer c.Close()

	stats, err := c.Stats()
	if err != nil {
		return fmt.Errorf("failed to get cache stats: %w", err)
	}

	if stats.TotalEntries == 0 {
		fmt.Println("Cache is already empty")
		return nil
	}

	if err := c.Clear(); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	fmt.Printf("Cleared %d cache entries\n", stats.TotalEntries)
	return nil
}

func runCacheCleanup(cmd *cobra.Command, args []string) error {
	c, err := cache.New(15 * time.Minute)
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}
	defer c.Close()

	statsBefore, err := c.Stats()
	if err != nil {
		return fmt.Errorf("failed to get cache stats: %w", err)
	}

	if err := c.Cleanup(); err != nil {
		return fmt.Errorf("failed to cleanup cache: %w", err)
	}

	statsAfter, err := c.Stats()
	if err != nil {
		return fmt.Errorf("failed to get cache stats after cleanup: %w", err)
	}

	removed := statsBefore.ExpiredEntries
	if removed > 0 {
		fmt.Printf("Removed %d expired cache entries\n", removed)
		fmt.Printf("Cache size reduced by %s\n", formatBytes(statsBefore.SizeBytes-statsAfter.SizeBytes))
	} else {
		fmt.Println("No expired entries to clean up")
	}

	return nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}