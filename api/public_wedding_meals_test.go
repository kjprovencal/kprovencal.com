package main

import (
	"strings"
	"testing"
)

func TestNormalizeWeddingMealLines(t *testing.T) {
	t.Parallel()
	got, err := normalizeWeddingMealLines([]string{
		"Guest 1: Chicken Alfredo",
		"Guest 2: Scampi",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len %d", len(got))
	}
}

func TestNormalizeWeddingMealLinesRejectsBadLabel(t *testing.T) {
	t.Parallel()
	_, err := normalizeWeddingMealLines([]string{"Guest 1: Pizza"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeWeddingMealLinesRejectsWrongPrefix(t *testing.T) {
	t.Parallel()
	_, err := normalizeWeddingMealLines([]string{"Guest 2: Chicken Alfredo"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "guest 1") {
		t.Fatalf("unexpected: %v", err)
	}
}
