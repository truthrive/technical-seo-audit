package adapter

import (
	"errors"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

// Common adapter sentinel errors.
var (
	ErrRunNotCompleted        = errors.New("audit adapter: crawl run is not completed")
	ErrRunNotFound            = errors.New("audit adapter: crawl run not found")
	ErrInvalidPolicy          = errors.New("audit adapter: invalid project policy assignment")
	ErrEmptyAuditRunID        = errors.New("audit adapter: audit run id cannot be empty")
	ErrEmptySnapshotID        = errors.New("audit adapter: snapshot id cannot be empty")
	ErrEmptyCrawlRunID        = errors.New("audit adapter: crawl run id cannot be empty")
	ErrPolicyRunMismatch      = errors.New("audit adapter: policy assignment audit run id does not match request")
	ErrMalformedRunTimestamp          = errors.New("audit adapter: malformed run started_at timestamp")
	ErrMalformedPageTimestamp         = errors.New("audit adapter: malformed page crawled_at timestamp")
	ErrMalformedOptionsJSON           = errors.New("audit adapter: malformed crawl options JSON")
	ErrMalformedPageJSON              = errors.New("audit adapter: malformed page data JSON")
)

// Standard evidence gap codes representing documented frozen data gaps
// where Audit V1 requirements exceed current SiteCrawl storage.
const (
	GapRawCanonicalUnavailable            = "GAP_RAW_CANONICAL_UNAVAILABLE"
	GapRawHrefUnavailable                 = "GAP_RAW_HREF_UNAVAILABLE"
	GapFetchTimingUnavailable             = "GAP_FETCH_TIMING_UNAVAILABLE"
	GapRetryHistoryUnavailable            = "GAP_RETRY_HISTORY_UNAVAILABLE"
	GapTLSValidityUnavailable             = "GAP_TLS_VALIDITY_UNAVAILABLE"
	GapChallengeDetectionUnavailable      = "GAP_CHALLENGE_DETECTION_UNAVAILABLE"
	GapResponseHeadersUnavailable         = "GAP_RESPONSE_HEADERS_UNAVAILABLE"
	GapBodyArtifactUnavailable            = "GAP_BODY_ARTIFACT_UNAVAILABLE"
	GapMainTextUnavailable                = "GAP_MAIN_TEXT_UNAVAILABLE"
	GapRenderedRawSourceUnavailable       = "GAP_RENDERED_RAW_SOURCE_UNAVAILABLE"
	GapRobotsDocumentUnavailable          = "GAP_ROBOTS_DOCUMENT_UNAVAILABLE"
	GapRobotsDecisionUnavailable          = "GAP_ROBOTS_DECISION_UNAVAILABLE"
	GapSitemapDocumentUnavailable         = "GAP_SITEMAP_DOCUMENT_UNAVAILABLE"
	GapStructuredDataStatusUnavailable    = "GAP_STRUCTURED_DATA_STATUS_UNAVAILABLE"
	GapRenderComparisonUnavailable        = "GAP_RENDER_COMPARISON_UNAVAILABLE"
	GapProbeObservationUnavailable        = "GAP_PROBE_OBSERVATION_UNAVAILABLE"
	GapProfileComparisonUnavailable       = "GAP_PROFILE_COMPARISON_UNAVAILABLE"
	GapAcquisitionPurposeAmbiguous        = "GAP_ACQUISITION_PURPOSE_AMBIGUOUS"
	GapPaginationProvenanceUnavailable    = "GAP_PAGINATION_PROVENANCE_UNAVAILABLE"
	GapDiscoveryProvenanceAmbiguous       = "GAP_DISCOVERY_PROVENANCE_AMBIGUOUS"
	GapFetchErrorUnmappable               = "GAP_FETCH_ERROR_UNMAPPABLE"
	GapBotResponseNotPreserved            = "GAP_BOT_RESPONSE_NOT_PRESERVED"
	GapUnresolvedLinkTargetUnavailable    = "GAP_UNRESOLVED_LINK_TARGET_UNAVAILABLE"
)

// EvidenceGap records an implementation-level diagnostic stating that the
// frozen Audit data model requires evidence not currently preserved by SiteCrawl.
// It is NOT a RuleResult or a Finding, and does NOT carry PASS/FAIL severity.
type EvidenceGap struct {
	GapCode         string `json:"gap_code"`
	SubjectRef      string `json:"subject_ref,omitempty"`
	Field           string `json:"field"`
	Reason          string `json:"reason"`
	SourceComponent string `json:"source_component"`
}

// BuildRequest defines the explicit inputs required to build a frozen EvidenceSnapshot.
type BuildRequest struct {
	CrawlRunID               string
	AuditRunID               audit.AuditRunID
	SnapshotID               audit.SnapshotID
	ProjectPolicyAssignments []audit.ProjectPolicyAssignment
}

// BuildResult returns the normalized evidence observations and frozen EvidenceSnapshot.
type BuildResult struct {
	UrlResources                []audit.UrlResource
	DiscoveryRecords            []audit.DiscoveryRecord
	FetchObservations           []audit.FetchObservation
	RedirectHops                []audit.RedirectHop
	HtmlObservations            []audit.HtmlObservation
	RobotsDirectiveObservations []audit.RobotsDirectiveObservation
	CanonicalObservations       []audit.CanonicalObservation
	LinkObservations            []audit.LinkObservation
	EvidenceSnapshot            *audit.EvidenceSnapshot
	EvidenceGaps                []EvidenceGap
}
