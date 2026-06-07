package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseLogTimeQuery_RFC3339(t *testing.T) {
	t.Parallel()
	got, err := parseLogTimeQuery("2026-06-07T15:30:00Z", false)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 6, 7, 15, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestParseLogTimeQuery_DateOnlyUntil(t *testing.T) {
	t.Parallel()
	got, err := parseLogTimeQuery("2026-06-07", true)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 6, 7, 23, 59, 59, 999999999, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestFilterLogLinesByRange(t *testing.T) {
	t.Parallel()
	lines := []string{
		`time=2026-06-07T10:00:00Z level=INFO msg=one`,
		`time=2026-06-07T12:00:00Z level=INFO msg=two`,
		`time=2026-06-07T14:00:00Z level=INFO msg=three`,
		`not a slog line`,
	}
	since := time.Date(2026, 6, 7, 11, 0, 0, 0, time.UTC)
	until := time.Date(2026, 6, 7, 13, 0, 0, 0, time.UTC)
	got := filterLogLinesByRange(lines, &since, &until)
	if len(got) != 1 || !strings.Contains(got[0], "msg=two") {
		t.Fatalf("got %v", got)
	}
}
