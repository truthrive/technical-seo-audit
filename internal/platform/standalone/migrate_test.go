package standalone

import (
	"strings"
	"testing"
)

func TestMigrateIdempotentAndOrdered(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO users (id, name) VALUES (1, 'Alice')`,
		`ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`,
	}

	// First run applies all 3 steps.
	if err := Migrate(db, "test_comp", stmts); err != nil {
		t.Fatalf("first Migrate failed: %v", err)
	}

	// Verify table and column exist.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE name = 'Alice' AND email = ''`).Scan(&count); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 user, got %d", count)
	}

	// Second run with the EXACT SAME statements MUST NOT fail on ALTER TABLE.
	if err := Migrate(db, "test_comp", stmts); err != nil {
		t.Fatalf("second Migrate failed (should be idempotent): %v", err)
	}

	// Appending a new statement applies only the new statement.
	stmtsWithExtra := append(stmts, `ALTER TABLE users ADD COLUMN age INTEGER NOT NULL DEFAULT 0`)
	if err := Migrate(db, "test_comp", stmtsWithExtra); err != nil {
		t.Fatalf("Migrate with appended statement failed: %v", err)
	}

	// Verify new column exists.
	var age int
	if err := db.QueryRow(`SELECT age FROM users WHERE id = 1`).Scan(&age); err != nil {
		t.Fatalf("query for age failed: %v", err)
	}
	if age != 0 {
		t.Fatalf("expected age 0, got %d", age)
	}
}

func TestMigrateFailsAndStops(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	badStmts := []string{
		`CREATE TABLE items (id INTEGER PRIMARY KEY)`,
		`SYNTAX ERROR INVALID SQL STATEMENT`,
		`CREATE TABLE items_never_created (id INTEGER PRIMARY KEY)`,
	}

	err = Migrate(db, "bad_comp", badStmts)
	if err == nil {
		t.Fatal("expected migration to fail, got nil")
	}
	if !strings.Contains(err.Error(), "step 1 failed") {
		t.Errorf("expected error to mention step 1 failed, got %v", err)
	}

	// Step 2 should not have been executed.
	var exists int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='items_never_created'`).Scan(&exists)
	if exists != 0 {
		t.Fatal("step 2 was executed despite step 1 failure")
	}
}
