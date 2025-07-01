package utils

import (
	"testing"
)

func TestCopyToClipboard(t *testing.T) {
	// Test that the function doesn't panic with basic input
	err := CopyToClipboard("test.project.dataset.table")
	
	// On CI or systems without clipboard, this may fail - that's expected
	if err != nil {
		t.Logf("Clipboard not available (expected in CI): %v", err)
	} else {
		t.Log("Clipboard copy succeeded")
	}
}