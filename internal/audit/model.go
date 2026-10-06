package audit

import (
	"time"
)

// Identifier type aliases representing logical domain IDs.
// Underlying storage/UUID/ULID implementation is deliberately deferred.
type (
	AuditRunID                  string
	URLID                       string
	PolicyAssignmentID          string
	DiscoveryID                 string
	FetchID                     string
	RedirectHopID               string
	RawArtifactID               string
	RobotsDocumentID            string
	RobotsDecisionID            string
	HtmlObservationID           string
	RobotsDirectiveID           string
	CanonicalObservationID      string
	LinkID                      string
	SitemapID                   string
	SitemapEntryID              string
	StructuredBlockID           string
	RenderID                    string
	RenderComparisonID          string
	RenderResourceObservationID string
	ProbeID                     string
	ProfileComparisonID         string
	SnapshotID                  string
	ObservationID               string
	RuleResultID                string
	RuleEvidenceRefID           string
	ManualReviewTaskID          string
	FindingID                   string
	FindingMemberID             string
)

// AuditRun represents one complete audit execution lifecycle.
type AuditRun struct {
	AuditRunID           AuditRunID     `json:"audit_run_id"`
	StartURL             string         `json:"start_url"`
	NormalizedStartURL   string         `json:"normalized_start_url"`
	StartedAt            time.Time      `json:"started_at"`
	CompletedAt          *time.Time     `json:"completed_at,omitempty"`
	RunStatus            AuditRunStatus `json:"run_status"`
	KnowledgeSnapshot    string         `json:"knowledge_snapshot"`
	RuleManifestVersion  string         `json:"rule_manifest_version"`
	ExecutionSettingsRef string         `json:"execution_settings_ref,omitempty"`
}

// UrlResource represents URL identity, separate from fetch identity or discovery membership.
type UrlResource struct {
	URLID           URLID      `json:"url_id"`
	AuditRunID     AuditRunID `json:"audit_run_id"`
	URL             string     `json:"url"`
	NormalizedURL   string     `json:"normalized_url"`
	Scheme          string     `json:"scheme"`
	Host            string     `json:"host"`
	Port            int        `json:"port"`
	Path            string     `json:"path"`
	Query           string     `json:"query"`
	FragmentRemoved bool       `json:"fragment_removed"`
	IsInternal      *bool      `json:"is_internal,omitempty"`
	Origin          string     `json:"origin"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
}

// DiscoveryRecord represents legitimate provenance granting site discovery universe membership.
// Synthesized probe requests, redirect continuations, and canonical targets do NOT create discovery records.
type DiscoveryRecord struct {
	DiscoveryID   DiscoveryID   `json:"discovery_id"`
	AuditRunID    AuditRunID    `json:"audit_run_id"`
	URLID         URLID         `json:"url_id"`
	DiscoveryType DiscoveryType `json:"discovery_type"`
	SourceURLID   *URLID        `json:"source_url_id,omitempty"`
	SourceRef     string        `json:"source_ref,omitempty"`
	DiscoveredAt  time.Time     `json:"discovered_at"`
}

// FetchObservation represents one HTTP acquisition attempt.
type FetchObservation struct {
	FetchID             FetchID            `json:"fetch_id"`
	AuditRunID          AuditRunID         `json:"audit_run_id"`
	URLID               URLID              `json:"url_id"`
	AcquisitionPurpose  AcquisitionPurpose `json:"acquisition_purpose"`
	RequestProfile      RequestProfile     `json:"request_profile"`
	RequestedAt         *time.Time         `json:"requested_at,omitempty"`
	CompletedAt         *time.Time         `json:"completed_at,omitempty"`
	FetchAttempted      bool               `json:"fetch_attempted"`
	Status              int                `json:"status"`
	FinalURLID          *URLID             `json:"final_url_id,omitempty"`
	ContentType         string             `json:"content_type"`
	ResponseTimeMs      int64              `json:"response_time_ms"`
	FetchErrorType      string             `json:"fetch_error_type,omitempty"`
	TLSValid            *bool              `json:"tls_valid,omitempty"`
	ChallengeDetected   *bool              `json:"challenge_detected,omitempty"`
	ResponseHeadersRef  string             `json:"response_headers_ref,omitempty"`
	BodyArtifactRef     string             `json:"body_artifact_ref,omitempty"`
	ObservedAt          time.Time          `json:"observed_at"`
}

// RedirectHop stores ordered hops for a redirected fetch path.
type RedirectHop struct {
	RedirectHopID      RedirectHopID `json:"redirect_hop_id"`
	FetchID            FetchID       `json:"fetch_id"`
	HopIndex           int           `json:"hop_index"`
	SourceURL          string        `json:"source_url"`
	Status             int           `json:"status"`
	LocationRaw        string        `json:"location_raw"`
	ResolvedTargetURL  string        `json:"resolved_target_url"`
	ObservedAt         time.Time     `json:"observed_at"`
}

// RawArtifact represents stored raw documents/payloads (HTML, robots.txt, sitemap XML, rendered DOM).
type RawArtifact struct {
	ArtifactID   RawArtifactID `json:"artifact_id"`
	AuditRunID   AuditRunID    `json:"audit_run_id"`
	ArtifactType string        `json:"artifact_type"`
	ContentHash  string        `json:"content_hash"`
	MimeType     string        `json:"mime_type"`
	ByteSize     int64         `json:"byte_size"`
	StorageRef   string        `json:"storage_ref"`
	CapturedAt   time.Time     `json:"captured_at"`
}

// RobotsDocumentObservation represents a fetched and parsed robots.txt file.
type RobotsDocumentObservation struct {
	RobotsDocumentID RobotsDocumentID `json:"robots_document_id"`
	AuditRunID       AuditRunID       `json:"audit_run_id"`
	RobotsURLID      URLID            `json:"robots_url_id"`
	FetchID          FetchID          `json:"fetch_id"`
	ParseStatus      string           `json:"parse_status"`
	ParseError       string           `json:"parse_error,omitempty"`
	RawArtifactRef   string           `json:"raw_artifact_ref,omitempty"`
	ObservedAt       time.Time        `json:"observed_at"`
}

// RobotsDecision represents the effective robots.txt decision for a specific user-agent and URL.
type RobotsDecision struct {
	RobotsDecisionID    RobotsDecisionID `json:"robots_decision_id"`
	AuditRunID          AuditRunID       `json:"audit_run_id"`
	URLID               URLID            `json:"url_id"`
	RobotsDocumentID    RobotsDocumentID `json:"robots_document_id"`
	Agent               string           `json:"agent"`
	Allowed             bool             `json:"allowed"`
	MatchedRuleType     string           `json:"matched_rule_type,omitempty"`
	MatchedRulePattern  string           `json:"matched_rule_pattern,omitempty"`
	ObservedAt          time.Time        `json:"observed_at"`
}

// HtmlObservation represents extracted technical HTML metadata and signals.
type HtmlObservation struct {
	HTMLObservationID    HtmlObservationID `json:"html_observation_id"`
	AuditRunID           AuditRunID        `json:"audit_run_id"`
	URLID                URLID             `json:"url_id"`
	FetchID              FetchID           `json:"fetch_id"`
	Title                string            `json:"title"`
	MetaDescription      string            `json:"meta_description"`
	H1Values             []string          `json:"h1_values"`
	MetaRobotsRaw        []string          `json:"meta_robots_raw"`
	XRobotsRaw           []string          `json:"x_robots_raw"`
	CanonicalRawValues   []string          `json:"canonical_raw_values"`
	MainTextPresent      *bool             `json:"main_text_present,omitempty"`
	MainTextFingerprint  string            `json:"main_text_fingerprint,omitempty"`
	ContentFingerprint   string            `json:"content_fingerprint,omitempty"`
	ObservedAt           time.Time         `json:"observed_at"`
}

// RobotsDirectiveObservation stores individual and combined robots directives from meta tags or headers.
type RobotsDirectiveObservation struct {
	RobotsDirectiveObservationID RobotsDirectiveID `json:"robots_directive_observation_id"`
	AuditRunID                   AuditRunID        `json:"audit_run_id"`
	URLID                        URLID             `json:"url_id"`
	Source                       DirectiveSource   `json:"source"`
	RawValue                     string            `json:"raw_value"`
	ParsedTokens                 []string          `json:"parsed_tokens"`
	UnsupportedTokens            []string          `json:"unsupported_tokens"`
	ParseErrors                  []string          `json:"parse_errors"`
	EffectiveNoindex             bool              `json:"effective_noindex"`
	ObservedAt                   time.Time         `json:"observed_at"`
}

// CanonicalObservation stores raw and normalized canonical declarations for a URL.
type CanonicalObservation struct {
	CanonicalObservationID CanonicalObservationID `json:"canonical_observation_id"`
	AuditRunID             AuditRunID             `json:"audit_run_id"`
	URLID                  URLID                  `json:"url_id"`
	CanonicalCount         int                    `json:"canonical_count"`
	RawValues              []string               `json:"raw_values"`
	NormalizedValues       []string               `json:"normalized_values"`
	ResolvedTargetURLIDs   []URLID                `json:"resolved_target_url_ids"`
	ParseErrors            []string               `json:"parse_errors"`
	ObservedAt             time.Time              `json:"observed_at"`
}

// LinkObservation represents an extracted hyperlink from an HTML document.
type LinkObservation struct {
	LinkID                     LinkID     `json:"link_id"`
	AuditRunID                 AuditRunID `json:"audit_run_id"`
	SourceURLID                URLID      `json:"source_url_id"`
	TargetURLID                *URLID     `json:"target_url_id,omitempty"`
	TargetURLRaw               string     `json:"target_url_raw"`
	TargetURLResolved          string     `json:"target_url_resolved"`
	ElementTag                 string     `json:"element_tag"`
	HREFRaw                    string     `json:"href_raw"`
	AnchorText                 string     `json:"anchor_text"`
	LinkLocation               string     `json:"link_location"`
	NavigationCandidate        bool       `json:"navigation_candidate"`
	NavigationCandidateReason  string     `json:"navigation_candidate_reason,omitempty"`
	ObservedAt                 time.Time  `json:"observed_at"`
}

// SitemapObservation represents a fetched and parsed sitemap document.
type SitemapObservation struct {
	SitemapID      SitemapID           `json:"sitemap_id"`
	AuditRunID     AuditRunID          `json:"audit_run_id"`
	SitemapURLID   URLID               `json:"sitemap_url_id"`
	FetchID        FetchID             `json:"fetch_id"`
	DocumentType   SitemapDocumentType `json:"document_type"`
	ParseStatus    string              `json:"parse_status"`
	ParseError     string              `json:"parse_error,omitempty"`
	RawArtifactRef string              `json:"raw_artifact_ref,omitempty"`
	ObservedAt     time.Time           `json:"observed_at"`
}

// SitemapEntry represents a single URL listed in a sitemap.
type SitemapEntry struct {
	SitemapEntryID   SitemapEntryID `json:"sitemap_entry_id"`
	AuditRunID       AuditRunID     `json:"audit_run_id"`
	SitemapID        SitemapID      `json:"sitemap_id"`
	ListedURLID      URLID          `json:"listed_url_id"`
	ListedURLRaw     string         `json:"listed_url_raw"`
	LastmodRaw       string         `json:"lastmod_raw,omitempty"`
	LastmodNormalized *time.Time    `json:"lastmod_normalized,omitempty"`
	ObservedAt       time.Time      `json:"observed_at"`
}

// StructuredDataBlock represents an extracted block of structured data.
type StructuredDataBlock struct {
	StructuredBlockID      StructuredBlockID    `json:"structured_block_id"`
	AuditRunID             AuditRunID           `json:"audit_run_id"`
	URLID                  URLID                `json:"url_id"`
	Format                 StructuredDataFormat `json:"format"`
	BlockIndex             int                  `json:"block_index"`
	RawArtifactOrValueRef  string               `json:"raw_artifact_or_value_ref"`
	ParseStatus            string               `json:"parse_status"`
	ParseError             string               `json:"parse_error,omitempty"`
	ObservedAt             time.Time            `json:"observed_at"`
}

// RenderObservation represents the execution and output of JavaScript rendering on a URL.
type RenderObservation struct {
	RenderID               RenderID     `json:"render_id"`
	AuditRunID             AuditRunID   `json:"audit_run_id"`
	URLID                  URLID        `json:"url_id"`
	RenderRequested        bool         `json:"render_requested"`
	RenderStatus           RenderStatus `json:"render_status"`
	RenderError            string       `json:"render_error,omitempty"`
	RenderedDOMArtifactRef string       `json:"rendered_dom_artifact_ref,omitempty"`
	RenderedAt             *time.Time   `json:"rendered_at,omitempty"`
}

// RenderFieldComparison captures the difference between raw HTML and rendered DOM for an audited field.
type RenderFieldComparison struct {
	RenderComparisonID RenderComparisonID    `json:"render_comparison_id"`
	AuditRunID         AuditRunID            `json:"audit_run_id"`
	URLID              URLID                 `json:"url_id"`
	RenderID           RenderID              `json:"render_id"`
	Field              RenderComparisonField `json:"field"`
	RawValue           string                `json:"raw_value"`
	RenderedValue      string                `json:"rendered_value"`
	DifferenceType     DifferenceType        `json:"difference_type"`
	ObservedAt         time.Time             `json:"observed_at"`
}

// RenderResourceObservation captures dependency and accessibility of sub-resources required for rendering.
type RenderResourceObservation struct {
	RenderResourceObservationID RenderResourceObservationID `json:"render_resource_observation_id"`
	AuditRunID                  AuditRunID                  `json:"audit_run_id"`
	PageURLID                   URLID                       `json:"page_url_id"`
	ResourceURLID               URLID                       `json:"resource_url_id"`
	ResourceType                string                      `json:"resource_type"`
	RobotsAllowed               bool                        `json:"robots_allowed"`
	LoadStatus                  string                      `json:"load_status"`
	RenderDependencyObserved    bool                        `json:"render_dependency_observed"`
	RenderDiffEvidenceRef       string                      `json:"render_diff_evidence_ref,omitempty"`
	ObservedAt                  time.Time                   `json:"observed_at"`
}

// ProbeObservation captures diagnostic probe executions (known-missing, host/slash variants, bot profiles).
// Probes do NOT create discovery records and cannot affect site discovery graph metrics.
type ProbeObservation struct {
	ProbeID            ProbeID    `json:"probe_id"`
	AuditRunID         AuditRunID `json:"audit_run_id"`
	ProbeType          ProbeType  `json:"probe_type"`
	TargetURLID        URLID      `json:"target_url_id"`
	RelatedURLID       *URLID     `json:"related_url_id,omitempty"`
	ExpectedProbeState string     `json:"expected_probe_state"`
	FetchID            FetchID    `json:"fetch_id"`
	CreatedAt          time.Time  `json:"created_at"`
}

// ProfileComparisonObservation captures observed response comparison between default and bot request profiles.
// GUARDRAIL: Response comparison is technical observation only and is NOT verified crawler identity.
type ProfileComparisonObservation struct {
	ProfileComparisonID           ProfileComparisonID `json:"profile_comparison_id"`
	AuditRunID                    AuditRunID          `json:"audit_run_id"`
	URLID                         URLID               `json:"url_id"`
	BaselineFetchID               FetchID             `json:"baseline_fetch_id"`
	ComparisonFetchID             FetchID             `json:"comparison_fetch_id"`
	Profile                       RequestProfile      `json:"profile"`
	BaselineStatus                int                 `json:"baseline_status"`
	ComparisonStatus              int                 `json:"comparison_status"`
	BaselineChallengeDetected     bool                `json:"baseline_challenge_detected"`
	ComparisonChallengeDetected   bool                `json:"comparison_challenge_detected"`
	BaselineResponseFingerprint   string              `json:"baseline_response_fingerprint,omitempty"`
	ComparisonResponseFingerprint string              `json:"comparison_response_fingerprint,omitempty"`
	ObservedAt                    time.Time           `json:"observed_at"`
}

// EvidenceSnapshot represents the frozen evaluation boundary.
// Rule evaluation is permitted only against a FROZEN EvidenceSnapshot.
type EvidenceSnapshot struct {
	SnapshotID               SnapshotID               `json:"snapshot_id"`
	AuditRunID               AuditRunID               `json:"audit_run_id"`
	CreatedAt                time.Time                `json:"created_at"`
	FrozenAt                 *time.Time               `json:"frozen_at,omitempty"`
	SnapshotStatus           SnapshotStatus           `json:"snapshot_status"`
	NormalizationVersion     string                   `json:"normalization_version"`
	CrawlComplete            bool                     `json:"crawl_complete"`
	SitemapDiscoveryComplete bool                     `json:"sitemap_discovery_complete"`
	RenderSelectionComplete  bool                     `json:"render_selection_complete"`
	ProbeCollectionComplete  bool                     `json:"probe_collection_complete"`
	NormalizedObservations   []NormalizedObservation  `json:"normalized_observations,omitempty"`
}

// NormalizedObservation represents a generic normalized observation for Rule Engine consumption.
type NormalizedObservation struct {
	ObservationID      ObservationID         `json:"observation_id"`
	AuditRunID         AuditRunID            `json:"audit_run_id"`
	SnapshotID         SnapshotID            `json:"snapshot_id"`
	SubjectType        EvaluationSubjectType `json:"subject_type"`
	SubjectRef         string                `json:"subject_ref"`
	Field              string                `json:"field"`
	Value              string                `json:"value"`
	DerivationType     DerivationType        `json:"derivation_type"`
	SourceEvidenceRefs []string              `json:"source_evidence_refs"`
	ObservedAt         time.Time             `json:"observed_at"`
}

// RuleResult represents an immutable atomic conclusion of evaluating one rule against a frozen snapshot.
// No numeric score, health score, or business priority is calculated here.
type RuleResult struct {
	RuleResultID    RuleResultID          `json:"rule_result_id"`
	AuditRunID      AuditRunID            `json:"audit_run_id"`
	SnapshotID      SnapshotID            `json:"snapshot_id"`
	RuleID          string                `json:"rule_id"`
	ParentCheck     string                `json:"parent_check"`
	RuleVersion     int                   `json:"rule_version"`
	SubjectType     EvaluationSubjectType `json:"subject_type"`
	SubjectRef      string                `json:"subject_ref"`
	Status          RuleResultStatus      `json:"status"`
	Severity        Severity              `json:"severity"`
	Scope           AuditReportScope      `json:"scope"`
	ObservedSummary string                `json:"observed_summary"`
	ExpectedSummary string                `json:"expected_summary"`
	EvaluatedAt     time.Time             `json:"evaluated_at"`
	EvidenceRefs    []RuleEvidenceRef     `json:"evidence_refs,omitempty"`
}

// RuleEvidenceRef provides exact traceability from a RuleResult to observed evidence.
type RuleEvidenceRef struct {
	RuleEvidenceRefID RuleEvidenceRefID `json:"rule_evidence_ref_id"`
	RuleResultID      RuleResultID      `json:"rule_result_id"`
	EvidenceType      string            `json:"evidence_type"`
	EvidenceRef       string            `json:"evidence_ref"`
	Field             string            `json:"field"`
	ObservedValue     string            `json:"observed_value"`
	Role              EvidenceRole      `json:"role"`
}

// ManualReviewTask represents an open or resolved human review task generated for guided review rules.
// Human resolution does NOT overwrite the machine RuleResult history.
type ManualReviewTask struct {
	TaskID         ManualReviewTaskID      `json:"task_id"`
	AuditRunID     AuditRunID              `json:"audit_run_id"`
	RuleResultID   RuleResultID            `json:"rule_result_id"`
	ReviewQuestion string                  `json:"review_question"`
	EvidenceRefs   []string                `json:"evidence_refs"`
	ReviewStatus   ManualReviewTaskStatus  `json:"review_status"`
	CreatedAt      time.Time               `json:"created_at"`
	Resolution     *ManualReviewResolution `json:"resolution,omitempty"`
}

// ManualReviewResolution stores optional human decision without overwriting original machine evidence.
type ManualReviewResolution struct {
	ResolvedBy string    `json:"resolved_by"`
	ResolvedAt time.Time `json:"resolved_at"`
	Verdict    string    `json:"verdict"`
	Notes      string    `json:"notes"`
}

// Finding represents report-level aggregation over atomic RuleResults.
// Safe V1 invariant: A Finding centers on ONE atomic rule_id and aggregates by status and severity.
type Finding struct {
	FindingID                  FindingID                  `json:"finding_id"`
	AuditRunID                 AuditRunID                 `json:"audit_run_id"`
	RuleID                     string                     `json:"rule_id"`
	ParentCheck                string                     `json:"parent_check"`
	Status                     RuleResultStatus           `json:"status"`
	Severity                   Severity                   `json:"severity"`
	PresentationClassification PresentationClassification `json:"presentation_classification"`
	AffectedCount              int                        `json:"affected_count"`
	Summary                    string                     `json:"summary"`
	ExpectedState              string                     `json:"expected_state"`
	RecommendedAction          string                     `json:"recommended_action"`
	CreatedAt                  time.Time                  `json:"created_at"`
	Members                    []FindingMember            `json:"members,omitempty"`
}

// FindingMember links an individual RuleResult to its parent Finding aggregation.
type FindingMember struct {
	FindingMemberID FindingMemberID `json:"finding_member_id"`
	FindingID       FindingID       `json:"finding_id"`
	RuleResultID    RuleResultID    `json:"rule_result_id"`
}
