package audit

import (
	"fmt"
)

// RuleResultStatus defines the exact six approved evaluation statuses for an atomic rule result.
// INFO is explicitly prohibited as a RuleResult status.
type RuleResultStatus string

const (
	StatusPass          RuleResultStatus = "PASS"
	StatusWarning       RuleResultStatus = "WARNING"
	StatusFail          RuleResultStatus = "FAIL"
	StatusManualReview  RuleResultStatus = "MANUAL_REVIEW"
	StatusNotApplicable RuleResultStatus = "NOT_APPLICABLE"
	StatusUnknown       RuleResultStatus = "UNKNOWN"
)

// ValidRuleResultStatuses returns the slice of all approved rule statuses.
func ValidRuleResultStatuses() []RuleResultStatus {
	return []RuleResultStatus{
		StatusPass,
		StatusWarning,
		StatusFail,
		StatusManualReview,
		StatusNotApplicable,
		StatusUnknown,
	}
}

// ValidateRuleResultStatus validates that a status matches one of the six frozen statuses.
func ValidateRuleResultStatus(s RuleResultStatus) error {
	switch s {
	case StatusPass, StatusWarning, StatusFail, StatusManualReview, StatusNotApplicable, StatusUnknown:
		return nil
	default:
		return fmt.Errorf("audit: invalid rule result status %q (allowed: PASS, WARNING, FAIL, MANUAL_REVIEW, NOT_APPLICABLE, UNKNOWN; INFO is prohibited)", s)
	}
}

// PresentationClassification defines the report/presentation level classification.
// It is intentionally separated from atomic RuleResultStatus and must not be hard-mapped 1:1.
type PresentationClassification string

const (
	PresentationIssue        PresentationClassification = "ISSUE"
	PresentationWarning      PresentationClassification = "WARNING"
	PresentationManualReview PresentationClassification = "MANUAL_REVIEW"
	PresentationInformation  PresentationClassification = "INFORMATION"
)

// ValidatePresentationClassification validates the report presentation classification.
func ValidatePresentationClassification(p PresentationClassification) error {
	switch p {
	case PresentationIssue, PresentationWarning, PresentationManualReview, PresentationInformation:
		return nil
	default:
		return fmt.Errorf("audit: invalid presentation classification %q (allowed: ISSUE, WARNING, MANUAL_REVIEW, INFORMATION)", p)
	}
}

// Severity defines the technical impact severity level.
// Severity is NOT project priority and does not contain numeric weights, scores, or priority calculations.
type Severity string

const (
	SeverityP0 Severity = "P0" // Critical: materially blocks site availability, crawling, or indexing at large/critical scope
	SeverityP1 Severity = "P1" // High: strong technical impact or substantial consolidation/discovery risk
	SeverityP2 Severity = "P2" // Medium: optimization issue, partial impact, or problem requiring context
	SeverityP3 Severity = "P3" // Low / Optional: best practice, informational check, or experimental item
)

// ValidateSeverity validates that a severity is one of P0, P1, P2, P3.
func ValidateSeverity(s Severity) error {
	switch s {
	case SeverityP0, SeverityP1, SeverityP2, SeverityP3:
		return nil
	default:
		return fmt.Errorf("audit: invalid severity %q (allowed: P0, P1, P2, P3)", s)
	}
}

// AutomationClass defines the automation classification of an executable atomic rule.
type AutomationClass string

const (
	AutomationDeterministic AutomationClass = "deterministic"
	AutomationAssisted      AutomationClass = "assisted"
	AutomationManual        AutomationClass = "manual"
)

// ValidateAutomationClass validates that the automation class is one of deterministic, assisted, manual.
func ValidateAutomationClass(a AutomationClass) error {
	switch a {
	case AutomationDeterministic, AutomationAssisted, AutomationManual:
		return nil
	default:
		return fmt.Errorf("audit: invalid automation class %q (allowed: deterministic, assisted, manual)", a)
	}
}

// LifecycleStage defines the canonical SEO lifecycle stages.
// Note: "Access" is a Crawl precondition / cross-cutting concern, NOT a separate lifecycle stage.
type LifecycleStage string

const (
	LifecycleDiscover    LifecycleStage = "Discover"
	LifecycleCrawl       LifecycleStage = "Crawl"
	LifecycleRender      LifecycleStage = "Render"
	LifecycleIndex       LifecycleStage = "Index"
	LifecycleConsolidate LifecycleStage = "Consolidate"
	LifecycleRetrieve    LifecycleStage = "Retrieve"
	LifecycleCite        LifecycleStage = "Cite"
	LifecycleMeasure     LifecycleStage = "Measure"
)

// ValidateLifecycleStage validates that a lifecycle stage is one of the eight canonical stages.
func ValidateLifecycleStage(l LifecycleStage) error {
	switch l {
	case LifecycleDiscover, LifecycleCrawl, LifecycleRender, LifecycleIndex,
		LifecycleConsolidate, LifecycleRetrieve, LifecycleCite, LifecycleMeasure:
		return nil
	default:
		return fmt.Errorf("audit: invalid lifecycle stage %q (Access must NOT be a lifecycle stage; allowed: Discover, Crawl, Render, Index, Consolidate, Retrieve, Cite, Measure)", l)
	}
}

// AuditRunStatus represents the workflow status of an AuditRun.
// These are separate from SiteCrawl run states and RuleResult statuses.
type AuditRunStatus string

const (
	RunStatusCreated        AuditRunStatus = "CREATED"
	RunStatusAcquiring      AuditRunStatus = "ACQUIRING"
	RunStatusNormalizing    AuditRunStatus = "NORMALIZING"
	RunStatusSnapshotFrozen AuditRunStatus = "SNAPSHOT_FROZEN"
	RunStatusEvaluating     AuditRunStatus = "EVALUATING"
	RunStatusAggregating    AuditRunStatus = "AGGREGATING"
	RunStatusCompleted      AuditRunStatus = "COMPLETED"
	RunStatusFailed         AuditRunStatus = "FAILED"
	RunStatusCancelled      AuditRunStatus = "CANCELLED"
)

// ValidateAuditRunStatus validates that an AuditRun workflow status is recognized.
func ValidateAuditRunStatus(s AuditRunStatus) error {
	switch s {
	case RunStatusCreated, RunStatusAcquiring, RunStatusNormalizing, RunStatusSnapshotFrozen,
		RunStatusEvaluating, RunStatusAggregating, RunStatusCompleted, RunStatusFailed, RunStatusCancelled:
		return nil
	default:
		return fmt.Errorf("audit: invalid audit run status %q", s)
	}
}

// DiscoveryType defines the legitimate provenance for site discovery membership.
// Probe requests or synthetic fetches do NOT create discovery records.
type DiscoveryType string

const (
	DiscoveryStartURL        DiscoveryType = "START_URL"
	DiscoveryInternalLink    DiscoveryType = "INTERNAL_LINK"
	DiscoverySitemap         DiscoveryType = "SITEMAP"
	DiscoveryPagination      DiscoveryType = "PAGINATION"
	DiscoverySuppliedURLList DiscoveryType = "SUPPLIED_URL_LIST"
)

// ValidateDiscoveryType validates that a discovery provenance type is recognized.
func ValidateDiscoveryType(d DiscoveryType) error {
	switch d {
	case DiscoveryStartURL, DiscoveryInternalLink, DiscoverySitemap, DiscoveryPagination, DiscoverySuppliedURLList:
		return nil
	default:
		return fmt.Errorf("audit: invalid discovery type %q", d)
	}
}

// AcquisitionPurpose defines the purpose of an HTTP acquisition attempt.
// Acquisition purpose does NOT imply an audit conclusion.
type AcquisitionPurpose string

const (
	PurposeCrawl             AcquisitionPurpose = "CRAWL"
	PurposeRobotsFetch       AcquisitionPurpose = "ROBOTS_FETCH"
	PurposeSitemapFetch      AcquisitionPurpose = "SITEMAP_FETCH"
	PurposeCanonicalTarget   AcquisitionPurpose = "CANONICAL_TARGET"
	PurposeRedirectTarget    AcquisitionPurpose = "REDIRECT_TARGET"
	PurposeHostVariantProbe  AcquisitionPurpose = "HOST_VARIANT_PROBE"
	PurposeSlashVariantProbe AcquisitionPurpose = "SLASH_VARIANT_PROBE"
	PurposeKnownMissingProbe AcquisitionPurpose = "KNOWN_MISSING_PROBE"
	PurposeBotProfileCompare AcquisitionPurpose = "BOT_PROFILE_COMPARE"
	PurposeRenderSource      AcquisitionPurpose = "RENDER_SOURCE"
	PurposeRenderResource    AcquisitionPurpose = "RENDER_RESOURCE"
)

// ValidateAcquisitionPurpose validates the acquisition purpose.
func ValidateAcquisitionPurpose(p AcquisitionPurpose) error {
	switch p {
	case PurposeCrawl, PurposeRobotsFetch, PurposeSitemapFetch, PurposeCanonicalTarget,
		PurposeRedirectTarget, PurposeHostVariantProbe, PurposeSlashVariantProbe,
		PurposeKnownMissingProbe, PurposeBotProfileCompare, PurposeRenderSource, PurposeRenderResource:
		return nil
	default:
		return fmt.Errorf("audit: invalid acquisition purpose %q", p)
	}
}

// RequestProfile defines user-agent / client request profiles tested.
// GUARDRAIL: A request profile is an observed request profile and is NOT verified crawler identity.
type RequestProfile string

const (
	ProfileDefault      RequestProfile = "DEFAULT"
	ProfileGooglebot    RequestProfile = "GOOGLEBOT_PROFILE"
	ProfileOAISearchbot RequestProfile = "OAI_SEARCHBOT_PROFILE"
	ProfileGPTBot       RequestProfile = "GPTBOT_PROFILE"
	ProfileCustomBot    RequestProfile = "CUSTOM_BOT_PROFILE"
)

// ValidateRequestProfile validates the request profile.
func ValidateRequestProfile(r RequestProfile) error {
	switch r {
	case ProfileDefault, ProfileGooglebot, ProfileOAISearchbot, ProfileGPTBot, ProfileCustomBot:
		return nil
	default:
		return fmt.Errorf("audit: invalid request profile %q", r)
	}
}

// SnapshotStatus defines the lifecycle status of an EvidenceSnapshot.
type SnapshotStatus string

const (
	SnapshotBuilding SnapshotStatus = "BUILDING"
	SnapshotFrozen   SnapshotStatus = "FROZEN"
)

// ValidateSnapshotStatus validates the snapshot status.
func ValidateSnapshotStatus(s SnapshotStatus) error {
	switch s {
	case SnapshotBuilding, SnapshotFrozen:
		return nil
	default:
		return fmt.Errorf("audit: invalid snapshot status %q", s)
	}
}

// ManualReviewTaskStatus defines the status of a human review task.
type ManualReviewTaskStatus string

const (
	ReviewTaskOpen     ManualReviewTaskStatus = "OPEN"
	ReviewTaskResolved ManualReviewTaskStatus = "RESOLVED"
)

// ValidateManualReviewTaskStatus validates the review task status.
func ValidateManualReviewTaskStatus(s ManualReviewTaskStatus) error {
	switch s {
	case ReviewTaskOpen, ReviewTaskResolved:
		return nil
	default:
		return fmt.Errorf("audit: invalid manual review task status %q", s)
	}
}

// EvidenceRole defines the role of an evidence reference in a RuleResult.
type EvidenceRole string

const (
	EvidenceRolePrimary    EvidenceRole = "PRIMARY"
	EvidenceRoleSupporting EvidenceRole = "SUPPORTING"
	EvidenceRoleContext    EvidenceRole = "CONTEXT"
)

// ValidateEvidenceRole validates the evidence role.
func ValidateEvidenceRole(r EvidenceRole) error {
	switch r {
	case EvidenceRolePrimary, EvidenceRoleSupporting, EvidenceRoleContext:
		return nil
	default:
		return fmt.Errorf("audit: invalid evidence role %q", r)
	}
}

// DerivationType defines how a normalized observation was derived.
type DerivationType string

const (
	DerivationDirect     DerivationType = "DIRECT"
	DerivationNormalized DerivationType = "NORMALIZED"
	DerivationDerived    DerivationType = "DERIVED"
)

// ValidateDerivationType validates the derivation type.
func ValidateDerivationType(d DerivationType) error {
	switch d {
	case DerivationDirect, DerivationNormalized, DerivationDerived:
		return nil
	default:
		return fmt.Errorf("audit: invalid derivation type %q", d)
	}
}

// EvaluationSubjectType defines the logical subject of rule evaluation.
type EvaluationSubjectType string

const (
	SubjectSite                EvaluationSubjectType = "SITE"
	SubjectURL                 EvaluationSubjectType = "URL"
	SubjectLink                EvaluationSubjectType = "LINK"
	SubjectSitemap             EvaluationSubjectType = "SITEMAP"
	SubjectSitemapEntry        EvaluationSubjectType = "SITEMAP_ENTRY"
	SubjectStructuredDataBlock EvaluationSubjectType = "STRUCTURED_DATA_BLOCK"
	SubjectResource            EvaluationSubjectType = "RESOURCE"
	SubjectRenderComparison    EvaluationSubjectType = "RENDER_COMPARISON"
	SubjectProbe               EvaluationSubjectType = "PROBE"
	SubjectSiteConfiguration   EvaluationSubjectType = "SITE_CONFIGURATION"
)

// ValidateEvaluationSubjectType validates the evaluation subject type.
func ValidateEvaluationSubjectType(s EvaluationSubjectType) error {
	switch s {
	case SubjectSite, SubjectURL, SubjectLink, SubjectSitemap, SubjectSitemapEntry,
		SubjectStructuredDataBlock, SubjectResource, SubjectRenderComparison, SubjectProbe, SubjectSiteConfiguration:
		return nil
	default:
		return fmt.Errorf("audit: invalid evaluation subject type %q", s)
	}
}

// AuditReportScope defines the report-level aggregation or finding scope.
type AuditReportScope string

const (
	ScopeURL               AuditReportScope = "URL"
	ScopeTemplate          AuditReportScope = "TEMPLATE"
	ScopeDirectory         AuditReportScope = "DIRECTORY"
	ScopeSitewide          AuditReportScope = "SITEWIDE"
	ScopeSiteConfiguration AuditReportScope = "SITE_CONFIGURATION"
)

// ValidateAuditReportScope validates the report scope.
func ValidateAuditReportScope(s AuditReportScope) error {
	switch s {
	case ScopeURL, ScopeTemplate, ScopeDirectory, ScopeSitewide, ScopeSiteConfiguration:
		return nil
	default:
		return fmt.Errorf("audit: invalid audit report scope %q", s)
	}
}

// DirectiveSource defines the source of a robots directive.
type DirectiveSource string

const (
	DirectiveSourceMeta         DirectiveSource = "META"
	DirectiveSourceHTTPHeader   DirectiveSource = "HTTP_HEADER"
	DirectiveSourceRenderedMeta DirectiveSource = "RENDERED_META"
)

// ValidateDirectiveSource validates the directive source.
func ValidateDirectiveSource(d DirectiveSource) error {
	switch d {
	case DirectiveSourceMeta, DirectiveSourceHTTPHeader, DirectiveSourceRenderedMeta:
		return nil
	default:
		return fmt.Errorf("audit: invalid directive source %q", d)
	}
}

// SitemapDocumentType defines the parsed type of a sitemap document.
type SitemapDocumentType string

const (
	SitemapDocumentURLSet       SitemapDocumentType = "URLSET"
	SitemapDocumentSitemapIndex SitemapDocumentType = "SITEMAP_INDEX"
	SitemapDocumentUnknown      SitemapDocumentType = "UNKNOWN"
)

// ValidateSitemapDocumentType validates the sitemap document type.
func ValidateSitemapDocumentType(d SitemapDocumentType) error {
	switch d {
	case SitemapDocumentURLSet, SitemapDocumentSitemapIndex, SitemapDocumentUnknown:
		return nil
	default:
		return fmt.Errorf("audit: invalid sitemap document type %q", d)
	}
}

// StructuredDataFormat defines supported structured data syntax formats.
type StructuredDataFormat string

const (
	FormatJSONLD    StructuredDataFormat = "JSON_LD"
	FormatMicrodata StructuredDataFormat = "MICRODATA"
	FormatRDFa      StructuredDataFormat = "RDFA"
)

// ValidateStructuredDataFormat validates the structured data format.
func ValidateStructuredDataFormat(f StructuredDataFormat) error {
	switch f {
	case FormatJSONLD, FormatMicrodata, FormatRDFa:
		return nil
	default:
		return fmt.Errorf("audit: invalid structured data format %q", f)
	}
}

// RenderStatus defines the outcome of rendering a page.
type RenderStatus string

const (
	RenderStatusNotSelected RenderStatus = "NOT_SELECTED"
	RenderStatusSuccess     RenderStatus = "SUCCESS"
	RenderStatusFailed      RenderStatus = "FAILED"
)

// ValidateRenderStatus validates the render status.
func ValidateRenderStatus(r RenderStatus) error {
	switch r {
	case RenderStatusNotSelected, RenderStatusSuccess, RenderStatusFailed:
		return nil
	default:
		return fmt.Errorf("audit: invalid render status %q", r)
	}
}

// RenderComparisonField defines the normalized fields compared between raw and rendered states.
type RenderComparisonField string

const (
	RenderFieldTitle            RenderComparisonField = "TITLE"
	RenderFieldCanonical        RenderComparisonField = "CANONICAL"
	RenderFieldMetaRobots       RenderComparisonField = "META_ROBOTS"
	RenderFieldH1               RenderComparisonField = "H1"
	RenderFieldMainTextPresence RenderComparisonField = "MAIN_TEXT_PRESENCE"
	RenderFieldInternalLinkSet  RenderComparisonField = "INTERNAL_LINK_SET"
)

// ValidateRenderComparisonField validates the render comparison field.
func ValidateRenderComparisonField(f RenderComparisonField) error {
	switch f {
	case RenderFieldTitle, RenderFieldCanonical, RenderFieldMetaRobots, RenderFieldH1,
		RenderFieldMainTextPresence, RenderFieldInternalLinkSet:
		return nil
	default:
		return fmt.Errorf("audit: invalid render comparison field %q", f)
	}
}

// DifferenceType defines how a field differs between raw and rendered DOM.
type DifferenceType string

const (
	DiffUnchanged  DifferenceType = "UNCHANGED"
	DiffAdded      DifferenceType = "ADDED"
	DiffRemoved    DifferenceType = "REMOVED"
	DiffChanged    DifferenceType = "CHANGED"
	DiffSetChanged DifferenceType = "SET_CHANGED"
)

// ValidateDifferenceType validates the difference type.
func ValidateDifferenceType(d DifferenceType) error {
	switch d {
	case DiffUnchanged, DiffAdded, DiffRemoved, DiffChanged, DiffSetChanged:
		return nil
	default:
		return fmt.Errorf("audit: invalid difference type %q", d)
	}
}

// ProbeType defines the type of diagnostic probe executed.
type ProbeType string

const (
	ProbeKnownMissing ProbeType = "KNOWN_MISSING"
	ProbeHostVariant  ProbeType = "HOST_VARIANT"
	ProbeSlashVariant ProbeType = "SLASH_VARIANT"
	ProbeBotProfile   ProbeType = "BOT_PROFILE"
)

// ValidateProbeType validates the probe type.
func ValidateProbeType(p ProbeType) error {
	switch p {
	case ProbeKnownMissing, ProbeHostVariant, ProbeSlashVariant, ProbeBotProfile:
		return nil
	default:
		return fmt.Errorf("audit: invalid probe type %q", p)
	}
}
