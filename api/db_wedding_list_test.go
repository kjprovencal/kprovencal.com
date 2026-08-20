package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestListRSVPsAfterInsert(t *testing.T) {
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
	rows, err := listRSVPs(db, 50)
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

func TestUpdateAndDeleteWeddingRSVP(t *testing.T) {
	dir := t.TempDir()
	db, err := openDB(filepath.Join(dir, "badger"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := insertWeddingRSVP(db, "A", "a@b.co", 1, []string{"Guest 1: Chicken Alfredo"}, "hi"); err != nil {
		t.Fatal(err)
	}
	rows, err := listRSVPs(db, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows", len(rows))
	}
	id := rows[0].ID
	created := rows[0].CreatedAt

	if err := updateWeddingRSVP(db, id, "B", "b@c.co", 2, []string{"Guest 1: Scampi", "Guest 2: Verdura al Napoleon"}, "bye"); err != nil {
		t.Fatal(err)
	}
	rows, err = listRSVPs(db, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows after update", len(rows))
	}
	got := rows[0]
	if got.ID != id || !got.CreatedAt.Equal(created) {
		t.Fatalf("id/created_at changed: %+v", got)
	}
	if got.Name != "B" || got.Email != "b@c.co" || got.GuestCount != 2 || got.Notes != "bye" {
		t.Fatalf("fields not updated: %+v", got)
	}
	if len(got.Meals) != 2 || got.Meals[0] != "Guest 1: Scampi" {
		t.Fatalf("meals %+v", got.Meals)
	}

	if err := deleteWeddingRSVP(db, id); err != nil {
		t.Fatal(err)
	}
	rows, err = listRSVPs(db, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("got %d rows after delete", len(rows))
	}
	if err := deleteWeddingRSVP(db, id); !errors.Is(err, ErrRSVPNotFound) {
		t.Fatalf("want ErrRSVPNotFound, got %v", err)
	}
	if err := updateWeddingRSVP(db, id, "C", "c@d.co", 0, nil, ""); !errors.Is(err, ErrRSVPNotFound) {
		t.Fatalf("want ErrRSVPNotFound on update, got %v", err)
	}
}
