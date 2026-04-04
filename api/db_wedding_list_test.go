package main

import (
	"path/filepath"
	"testing"
)

func TestListWeddingRSVPsAfterInsert(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "badger")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := insertWeddingRSVP(db, "A", "a@b.co", 1, []string{"Guest 1: X"}, ""); err != nil {
		t.Fatal(err)
	}
	rows, err := listWeddingRSVPs(db, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Name != "A" {
		t.Fatalf("name %q", rows[0].Name)
	}
}
