package standalone

import (
	"database/sql"
	"fmt"
	"time"
)

// Migrate executes ordered migration steps for a named component, recording each
// applied step in a migration log table. Each step is applied exactly once per
// database. Reopening the same database is idempotent and safe against non-idempotent
// statements (e.g. ALTER TABLE). If any step fails, the error is returned immediately
// and subsequent steps are not executed.
func Migrate(db *sql.DB, component string, stmts []string) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS _schema_migrations (
		component   TEXT NOT NULL,
		step        INTEGER NOT NULL,
		applied_at  TEXT NOT NULL,
		PRIMARY KEY (component, step)
	)`)
	if err != nil {
		return fmt.Errorf("standalone: create migration table: %w", err)
	}

	rows, err := db.Query(`SELECT step FROM _schema_migrations WHERE component = ?`, component)
	if err != nil {
		return fmt.Errorf("standalone: query applied migrations for %s: %w", component, err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var step int
		if err := rows.Scan(&step); err != nil {
			return fmt.Errorf("standalone: scan applied migration step for %s: %w", component, err)
		}
		applied[step] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("standalone: read applied migrations for %s: %w", component, err)
	}

	for step, stmt := range stmts {
		if applied[step] {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("standalone: migration %s step %d failed: %w", component, step, err)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := db.Exec(`INSERT INTO _schema_migrations (component, step, applied_at) VALUES (?, ?, ?)`,
			component, step, now); err != nil {
			return fmt.Errorf("standalone: record migration %s step %d: %w", component, step, err)
		}
	}
	return nil
}
