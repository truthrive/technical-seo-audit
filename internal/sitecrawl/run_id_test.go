package sitecrawl

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestGenerateRunID(t *testing.T) {
	for i := 0; i < 50; i++ {
		id, err := generateRunID()
		if err != nil {
			t.Fatalf("generateRunID failed: %v", err)
		}
		if id == "" {
			t.Fatal("generateRunID returned an empty string")
		}

		parts := strings.Split(id, "_")
		if len(parts) != 3 {
			t.Fatalf("expected 3 parts separated by underscore, got %d in %q", len(parts), id)
		}

		// Check prefix
		if parts[0] != "crawl" {
			t.Errorf("expected prefix 'crawl', got %q", parts[0])
		}

		// Check timestamp format: 20060102150405
		parsedTime, err := time.Parse("20060102150405", parts[1])
		if err != nil {
			t.Errorf("invalid timestamp %q: %v", parts[1], err)
		}
		// Confirm timestamp is recent (within 1 minute)
		if time.Since(parsedTime) > time.Minute || time.Until(parsedTime) > time.Minute {
			t.Errorf("timestamp %v out of reasonable range compared to current UTC time", parsedTime)
		}

		// Check random suffix length (128 bits = 16 bytes = 32 hex chars)
		suffix := parts[2]
		if len(suffix) != 32 {
			t.Errorf("expected 32 hex chars for 128-bit entropy, got %d in %q", len(suffix), suffix)
		}

		// Check valid hexadecimal
		decoded, err := hex.DecodeString(suffix)
		if err != nil {
			t.Errorf("suffix %q is not valid hex: %v", suffix, err)
		}
		if len(decoded) != 16 {
			t.Errorf("expected 16 decoded bytes, got %d", len(decoded))
		}
	}
}
