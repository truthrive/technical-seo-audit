package sitecrawl

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
)

// SchemaDigest computes a deterministic SHA-256 digest of migration statements.
// Statements are normalized to LF line endings and formatted with step indices
// so the hash is stable across platforms and whitespace-sensitive.
func SchemaDigest(stmts []string) string {
	h := sha256.New()
	for i, s := range stmts {
		clean := strings.ReplaceAll(s, "\r\n", "\n")
		fmt.Fprintf(h, "step[%d]:\n%s\n---\n", i, clean)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TestSchemaGolden locks the standalone SiteCrawl schema migration steps.
//
// Existing migration steps must remain immutable so previously created databases
// remain fully compatible across versions. Future schema updates must be strictly
// append-only.
func TestSchemaGolden(t *testing.T) {
	const (
		wantStepCount = 38
		// Baseline SHA-256 digest computed across all 38 verified migration statements.
		wantDigest = "209ab3ee14d732cc16a77c09bc75829f4445d09f680ee708268d69b30717dab0"
	)

	if len(schemaStmts) != wantStepCount {
		t.Fatalf("schema step count changed: got %d statements, want %d", len(schemaStmts), wantStepCount)
	}

	gotDigest := SchemaDigest(schemaStmts)
	if gotDigest != wantDigest {
		t.Fatalf("schema golden digest mismatch: got %s, want %s (schema statements must be append-only and immutable)",
			gotDigest, wantDigest)
	}

	// Verify byte-level parity against the legacy reference file if present.
	legacyBytes, err := os.ReadFile("../../go/storage/runs.go")
	if err == nil {
		legacySrc := string(legacyBytes)
		for i, stmt := range schemaStmts {
			clean := strings.ReplaceAll(stmt, "\r\n", "\n")
			// Remove backticks to find statement text in legacy source
			raw := strings.TrimSpace(clean)
			if !strings.Contains(strings.ReplaceAll(legacySrc, "\r\n", "\n"), raw) {
				t.Errorf("step %d was not found identically in legacy go/storage/runs.go", i)
			}
		}
	}
}
