package sitecrawl

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
		wantBaselineStepCount = 38
		// Baseline SHA-256 digest computed across all 38 verified migration statements.
		// Verified against the canonical historical SiteCrawl migration sequence.
		wantBaselineDigest = "209ab3ee14d732cc16a77c09bc75829f4445d09f680ee708268d69b30717dab0"

		wantV18bStepCount = 46
	)

	if len(schemaStmts) < wantBaselineStepCount {
		t.Fatalf("schema step count decreased: got %d statements, want at least %d", len(schemaStmts), wantBaselineStepCount)
	}

	gotBaselineDigest := SchemaDigest(schemaStmts[:wantBaselineStepCount])
	if gotBaselineDigest != wantBaselineDigest {
		t.Fatalf("schema golden baseline digest mismatch: got %s, want %s (schema statements must be append-only and immutable)",
			gotBaselineDigest, wantBaselineDigest)
	}

	if len(schemaStmts) != wantV18bStepCount {
		t.Fatalf("schema step count changed: got %d statements, want %d", len(schemaStmts), wantV18bStepCount)
	}

	// Lock the full 46-step V1.8b schema digest as well
	const wantV18bDigest = "7b218e8111edb6d3564035aa2f6e6f9057573d0a13415a89af70a409cad74915"
	gotV18bDigest := SchemaDigest(schemaStmts)
	if gotV18bDigest != wantV18bDigest {
		t.Fatalf("schema golden v1.8b digest mismatch: got %s, want %s (schema statements must be append-only and immutable)",
			gotV18bDigest, wantV18bDigest)
	}
}
