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

	// Step 1 should NOT be recorded in _schema_migrations.
	var recorded int
	_ = db.QueryRow(`SELECT COUNT(*) FROM _schema_migrations WHERE component = 'bad_comp' AND step = 1`).Scan(&recorded)
	if recorded != 0 {
		t.Fatal("failed step 1 was recorded in _schema_migrations")
	}
}

func TestMigrateTransactionalRollbackAndRetry(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	initial := []string{
		`CREATE TABLE accounts (id INTEGER PRIMARY KEY, balance INTEGER NOT NULL)`,
	}

	// 1. First migration succeeds.
	if err := Migrate(db, "bank", initial); err != nil {
		t.Fatalf("initial migration failed: %v", err)
	}

	// 2. Step 1 attempts a multi-statement change that fails on an invalid table.
	// The transaction must roll back the insert into accounts.
	badStep := `INSERT INTO accounts (id, balance) VALUES (100, 500); INSERT INTO missing_table VALUES (1)`
	err = Migrate(db, "bank", []string{initial[0], badStep})
	if err == nil {
		t.Fatal("expected migration with badStep to fail, got nil")
	}

	// 3. Verify failed migration step is not recorded.
	var recorded int
	if err := db.QueryRow(`SELECT COUNT(*) FROM _schema_migrations WHERE component = 'bank' AND step = 1`).Scan(&recorded); err != nil {
		t.Fatalf("query _schema_migrations: %v", err)
	}
	if recorded != 0 {
		t.Fatalf("failed step 1 was recorded in _schema_migrations (%d entries)", recorded)
	}

	// 4. Verify failed schema effects were rolled back.
	var accountsCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE id = 100`).Scan(&accountsCount); err != nil {
		t.Fatalf("query accounts: %v", err)
	}
	if accountsCount != 0 {
		t.Fatalf("failed step schema effects were NOT rolled back; accounts count = %d", accountsCount)
	}

	// 5. Corrected migration retried successfully.
	goodStep := `INSERT INTO accounts (id, balance) VALUES (100, 500)`
	corrected := []string{initial[0], goodStep}
	if err := Migrate(db, "bank", corrected); err != nil {
		t.Fatalf("corrected migration failed: %v", err)
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE id = 100`).Scan(&accountsCount); err != nil {
		t.Fatalf("query accounts after retry: %v", err)
	}
	if accountsCount != 1 {
		t.Fatalf("expected accounts count 1 after retry, got %d", accountsCount)
	}

	// 6. Verify step 1 is now recorded.
	if err := db.QueryRow(`SELECT COUNT(*) FROM _schema_migrations WHERE component = 'bank' AND step = 1`).Scan(&recorded); err != nil {
		t.Fatalf("query _schema_migrations after retry: %v", err)
	}
	if recorded != 1 {
		t.Fatalf("expected step 1 to be recorded, got %d", recorded)
	}

	// 7. Repeated migration succeeds and remains idempotent.
	if err := Migrate(db, "bank", corrected); err != nil {
		t.Fatalf("repeated migration failed: %v", err)
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE id = 100`).Scan(&accountsCount)
	if accountsCount != 1 {
		t.Fatalf("repeated migration altered state; accounts count = %d", accountsCount)
	}
}
