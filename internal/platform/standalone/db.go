package standalone

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// OpenDB opens a standalone SQLite database handle at the given path (or in-memory ":memory:").
// It configures connection pragmas suitable for single-writer concurrency and tests connectivity.
func OpenDB(path string) (*sql.DB, error) {
	dsn := path
	if path != ":memory:" && !strings.Contains(path, "?") {
		dsn = path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}
	return db, nil
}
