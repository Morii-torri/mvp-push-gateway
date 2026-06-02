package db

import (
	"os"
	"strings"
	"testing"
)

func TestOverviewQueriesExcludeNonDeliveryTerminalStatuses(t *testing.T) {
	source, err := os.ReadFile("monitoring.go")
	if err != nil {
		t.Fatalf("read monitoring.go: %v", err)
	}

	legacyFilter := "status IN ('sent', 'failed', 'deduped', 'skipped')"
	if strings.Contains(string(source), legacyFilter) {
		t.Fatalf("overview delivery metrics must exclude deduped/skipped from send totals")
	}
}
