package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLogs(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "toad.log")

	lines := []string{
		`time=2026-03-09T10:00:00Z level=INFO msg="server started" port=8099`,
		`time=2026-03-09T10:00:01Z level=DEBUG msg="processing message" channel=general`,
		`time=2026-03-09T10:00:02Z level=ERROR msg="triage failed" error="timeout"`,
		`time=2026-03-09T10:00:03Z level=INFO msg="ribbit complete" duration=2.5s`,
		`time=2026-03-09T10:00:04Z level=WARN msg="rate limited" user=U123`,
	}
	os.WriteFile(logFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	// Read last 3 lines
	result, err := readLogs(logFile, 3, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(result), "\n")
	if len(got) != 3 {
		t.Errorf("expected 3 lines, got %d: %q", len(got), result)
	}

	// Filter by level
	result, _ = readLogs(logFile, 100, "ERROR", "", "")
	if !strings.Contains(result, "triage failed") {
		t.Errorf("expected error line, got: %q", result)
	}
	if strings.Contains(result, "server started") {
		t.Error("should not contain INFO when filtering ERROR")
	}

	// Search filter
	result, _ = readLogs(logFile, 100, "", "ribbit", "")
	if !strings.Contains(result, "ribbit complete") {
		t.Errorf("expected ribbit line, got: %q", result)
	}

	// No matches
	result, _ = readLogs(logFile, 100, "", "nonexistent", "")
	if !strings.Contains(result, "No matching") {
		t.Errorf("expected no matches message, got: %q", result)
	}
}
