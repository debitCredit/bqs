package config

import (
	"testing"
	"time"
)

func TestTTLConstants(t *testing.T) {
	// Verify TTL values are reasonable
	if TableListTTL != 5*time.Minute {
		t.Errorf("Expected TableListTTL to be 5 minutes, got %v", TableListTTL)
	}
	
	if MetadataTTL != 15*time.Minute {
		t.Errorf("Expected MetadataTTL to be 15 minutes, got %v", MetadataTTL)
	}
	
	if SchemaTTL != 30*time.Minute {
		t.Errorf("Expected SchemaTTL to be 30 minutes, got %v", SchemaTTL)
	}
	
	if DefaultCacheTTL != MetadataTTL {
		t.Errorf("Expected DefaultCacheTTL to equal MetadataTTL, got %v", DefaultCacheTTL)
	}
}

func TestUIConstants(t *testing.T) {
	// Verify UI constants are sensible
	if DefaultTableHeight < MinTableHeight {
		t.Errorf("DefaultTableHeight (%d) should be >= MinTableHeight (%d)", DefaultTableHeight, MinTableHeight)
	}
	
	// Verify column widths sum to reasonable total
	totalWidth := CacheColumnWidth + TableColumnWidth + TypeColumnWidth + CreatedColumnWidth
	if totalWidth < 60 || totalWidth > 100 {
		t.Errorf("Total column width (%d) seems unreasonable", totalWidth)
	}
}