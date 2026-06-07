package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
