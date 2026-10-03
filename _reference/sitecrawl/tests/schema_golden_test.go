package sitecrawl

import (
	"testing"

	"onescout/desktop/internal/testutil"
)

// Locks the migration steps that have shipped. See testutil.SchemaGolden for
// why editing one instead of appending destroys existing users' databases.

func TestSchemaGolden(t *testing.T) {
	// Updated twice: the inlinks index for the link-graph view, then the shared
	// total column core/runs needs. Both times the helper confirmed every
	// previously shipped step was byte-identical.
	testutil.SchemaGolden(t, "sitecrawl", schemaStmts, "5f12609fac59adfa283a3278503e36faec4e15dc2379f0ca815b702e2b8173bc")
}
