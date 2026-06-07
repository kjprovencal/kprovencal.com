package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTailLogFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "api.log")
	content := strings.Join([]string{"line1", "line2", "line3", "line4", "line5"}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	lines, err := tailLogFile(path, 3, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: %v", len(lines), lines)
	}
	if lines[0] != "line3" || lines[2] != "line5" {
		t.Fatalf("unexpected tail: %v", lines)
	}
}

func TestQueryLogFile_TimeRange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "api.log")
	lines := []string{
		`time=2026-06-07T10:00:00Z level=INFO msg=early`,
		`time=2026-06-07T12:00:00Z level=INFO msg=mid`,
		`time=2026-06-07T14:00:00Z level=INFO msg=late`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	since := time.Date(2026, 6, 7, 11, 0, 0, 0, time.UTC)
	until := time.Date(2026, 6, 7, 13, 0, 0, 0, time.UTC)
	res, err := queryLogFile(path, logQueryOpts{
		limit: 50,
		since: &since,
		until: &until,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.matched != 1 || len(res.lines) != 1 || !strings.Contains(res.lines[0], "mid") {
		t.Fatalf("got matched=%d lines=%v", res.matched, res.lines)
	}
}
