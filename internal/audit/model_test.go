package audit_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

func TestModelOptionality(t *testing.T) {
	// Test UrlResource with unavailable/nil optional fields
	u := audit.UrlResource{
		URLID:         "url:run1:1",
		AuditRunID:    "run1",
		URL:           "https://example.com",
		NormalizedURL: "https://example.com",
		Scheme:        "https",
		Host:          "example.com",
		Origin:        "https://example.com",
		IsInternal:    nil,
		CreatedAt:     nil,
	}

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal UrlResource failed: %v", err)
	}
	var uDec audit.UrlResource
	if err := json.Unmarshal(data, &uDec); err != nil {
		t.Fatalf("unmarshal UrlResource failed: %v", err)
	}
	if uDec.IsInternal != nil {
		t.Errorf("expected IsInternal to be nil, got %v", *uDec.IsInternal)
	}
	if uDec.CreatedAt != nil {
		t.Errorf("expected CreatedAt to be nil, got %v", *uDec.CreatedAt)
	}

	// Test with values populated
	isInternal := true
	now := time.Now().UTC().Truncate(time.Second)
	u.IsInternal = &isInternal
	u.CreatedAt = &now

	data, err = json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal populated UrlResource failed: %v", err)
	}
	if err := json.Unmarshal(data, &uDec); err != nil {
		t.Fatalf("unmarshal populated UrlResource failed: %v", err)
	}
	if uDec.IsInternal == nil || !*uDec.IsInternal {
		t.Errorf("expected IsInternal to be true, got %v", uDec.IsInternal)
	}
	if uDec.CreatedAt == nil || !uDec.CreatedAt.Equal(now) {
		t.Errorf("expected CreatedAt to be %v, got %v", now, uDec.CreatedAt)
	}

	// Test FetchObservation optional fields
	f := audit.FetchObservation{
		FetchID:            "fetch:run1:1",
		AuditRunID:         "run1",
		URLID:              "url:run1:1",
		AcquisitionPurpose: audit.PurposeCrawl,
		RequestProfile:     audit.ProfileDefault,
		RequestedAt:        nil,
		CompletedAt:        nil,
		TLSValid:           nil,
		ChallengeDetected:  nil,
	}
	data, err = json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal FetchObservation failed: %v", err)
	}
	var fDec audit.FetchObservation
	if err := json.Unmarshal(data, &fDec); err != nil {
		t.Fatalf("unmarshal FetchObservation failed: %v", err)
	}
	if fDec.RequestedAt != nil || fDec.CompletedAt != nil || fDec.TLSValid != nil || fDec.ChallengeDetected != nil {
		t.Errorf("expected nil optional fields in FetchObservation")
	}

	// Test HtmlObservation optional fields
	h := audit.HtmlObservation{
		HTMLObservationID: "html:run1:1",
		AuditRunID:        "run1",
		URLID:             "url:run1:1",
		FetchID:           "fetch:run1:1",
		MainTextPresent:   nil,
	}
	data, err = json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal HtmlObservation failed: %v", err)
	}
	var hDec audit.HtmlObservation
	if err := json.Unmarshal(data, &hDec); err != nil {
		t.Fatalf("unmarshal HtmlObservation failed: %v", err)
	}
	if hDec.MainTextPresent != nil {
		t.Errorf("expected MainTextPresent to be nil, got %v", *hDec.MainTextPresent)
	}
}
