package persistence

import (
	"path/filepath"
	"testing"
)

func TestBiographyRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "bio.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Empty to start.
	rows, err := db.LoadBiographies()
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("want 0 rows, got %d", len(rows))
	}

	// Insert two.
	if err := db.SaveBiography(42, "the chronicle of forty-two", "Winter Day 1, Year 25"); err != nil {
		t.Fatalf("save 42: %v", err)
	}
	if err := db.SaveBiography(7, "the tale of seven", "Winter Day 1, Year 25"); err != nil {
		t.Fatalf("save 7: %v", err)
	}

	// Upsert (same id, new text) must replace, not duplicate.
	if err := db.SaveBiography(42, "the revised chronicle", "Winter Day 2, Year 25"); err != nil {
		t.Fatalf("upsert 42: %v", err)
	}

	rows, err = db.LoadBiographies()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows after upsert, got %d", len(rows))
	}
	got := map[uint64]string{}
	for _, r := range rows {
		got[r.AgentID] = r.Biography
	}
	if got[42] != "the revised chronicle" {
		t.Errorf("agent 42 = %q, want revised", got[42])
	}
	if got[7] != "the tale of seven" {
		t.Errorf("agent 7 = %q", got[7])
	}
}
