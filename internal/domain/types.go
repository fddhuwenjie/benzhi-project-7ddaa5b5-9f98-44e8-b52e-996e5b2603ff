package domain

import "time"

type Status string

const (
	StatusDraft            Status = "draft"
	StatusValidated        Status = "validated"
	StatusUnderReview      Status = "under_review"
	StatusRevisionRequired Status = "revision_required"
	StatusResubmitted      Status = "resubmitted"
	StatusApproved         Status = "approved"
)

type Plan struct {
	TreeCode              string `json:"tree_code"`
	Species               string `json:"species"`
	AgeYears              int    `json:"age_years"`
	HealthGrade           string `json:"health_grade"`
	SourceLocation        string `json:"source_location"`
	DestinationLocation   string `json:"destination_location"`
	MigrationReason       string `json:"migration_reason"`
	ConstructionWindow    string `json:"construction_window"`
	ProtectionMeasures    string `json:"protection_measures"`
	TrunkDiameterCM       int    `json:"trunk_diameter_cm"`
	RootBallDiameterCM    int    `json:"root_ball_diameter_cm"`
	TransportDurationHour int    `json:"transport_duration_hours"`
	DestinationReady      bool   `json:"destination_ready"`
}

type MigrationCase struct {
	ID                  string               `json:"id"`
	Plan                Plan                 `json:"plan"`
	Status              Status               `json:"status"`
	Revision            int64                `json:"revision"`
	RuleVersion         string               `json:"rule_version,omitempty"`
	Findings            []PolicyFinding      `json:"findings,omitempty"`
	ValidationBatches   []ValidationBatch    `json:"validation_batches,omitempty"`
	ReviewRound         int                  `json:"review_round"`
	ReviewSeats         []ReviewSeat         `json:"review_seats,omitempty"`
	Opinions            []ReviewOpinion      `json:"opinions,omitempty"`
	ModificationItems   []ModificationItem   `json:"modification_items,omitempty"`
	RevisionSubmissions []RevisionSubmission `json:"revision_submissions,omitempty"`
	Approval            *ApprovalRecord      `json:"approval,omitempty"`
	Timeline            []TimelineEntry      `json:"timeline"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
}

type FindingSeverity string

const (
	SeverityBlocker FindingSeverity = "blocker"
	SeverityWarning FindingSeverity = "warning"
)

type PolicyFinding struct {
	ID            string          `json:"id"`
	CaseID        string          `json:"case_id"`
	CaseRevision  int64           `json:"case_revision"`
	RuleCode      string          `json:"rule_code"`
	Severity      FindingSeverity `json:"severity"`
	Message       string          `json:"message"`
	FieldPath     string          `json:"field_path"`
	EvidenceValue string          `json:"evidence_value"`
	EvaluatedAt   time.Time       `json:"evaluated_at"`
}

type ConclusionChange string

const (
	ConclusionNew        ConclusionChange = "new"
	ConclusionContinued  ConclusionChange = "continued"
	ConclusionEliminated ConclusionChange = "eliminated"
)

type ValidationConclusion struct {
	RuleCode         string           `json:"rule_code"`
	Change           ConclusionChange `json:"change"`
	FieldPath        string           `json:"field_path"`
	Applicability    string           `json:"applicability"`
	PreviousSeverity FindingSeverity  `json:"previous_severity,omitempty"`
	PreviousEvidence string           `json:"previous_evidence,omitempty"`
	CurrentSeverity  FindingSeverity  `json:"current_severity,omitempty"`
	CurrentMessage   string           `json:"current_message,omitempty"`
	CurrentEvidence  string           `json:"current_evidence,omitempty"`
}

type ValidationBatch struct {
	ID           string                 `json:"id"`
	CaseID       string                 `json:"case_id"`
	CaseRevision int64                  `json:"case_revision"`
	RuleVersion  string                 `json:"rule_version"`
	EvaluatedAt  time.Time              `json:"evaluated_at"`
	Findings     []PolicyFinding        `json:"findings"`
	Conclusions  []ValidationConclusion `json:"conclusions"`
}

type Discipline string

const (
	DisciplineArboriculture Discipline = "arboriculture"
	DisciplineConstruction  Discipline = "construction"
)

type ReviewSeat struct {
	Discipline Discipline `json:"discipline"`
	Completed  bool       `json:"completed"`
}

type ReviewIssue struct {
	Code       string `json:"code"`
	Title      string `json:"title"`
	FieldPath  string `json:"field_path"`
	Suggestion string `json:"suggestion"`
}

type ReviewOpinion struct {
	ID           string        `json:"id"`
	CaseID       string        `json:"case_id"`
	ReviewRound  int           `json:"review_round"`
	Discipline   Discipline    `json:"discipline"`
	ReviewerName string        `json:"reviewer_name"`
	Conclusion   string        `json:"conclusion"`
	Issues       []ReviewIssue `json:"issues"`
	Basis        string        `json:"basis"`
	SubmittedAt  time.Time     `json:"submitted_at"`
}

type ModificationItem struct {
	IssueCode    string     `json:"issue_code"`
	Discipline   Discipline `json:"discipline"`
	Title        string     `json:"title"`
	FieldPath    string     `json:"field_path"`
	Suggestion   string     `json:"suggestion"`
	ReviewerName string     `json:"reviewer_name"`
	Basis        string     `json:"basis"`
	Resolved     bool       `json:"resolved"`
}

type FieldChange struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type IssueResponse struct {
	IssueCode      string `json:"issue_code"`
	Response       string `json:"response"`
	Resolved       bool   `json:"resolved"`
	NoChangeReason string `json:"no_change_reason,omitempty"`
}

type AttachmentMetadata struct {
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Digest      string `json:"digest"`
}

type RevisionSubmission struct {
	ID                 string               `json:"id"`
	CaseID             string               `json:"case_id"`
	FromRevision       int64                `json:"from_revision"`
	ToRevision         int64                `json:"to_revision"`
	FieldChanges       []FieldChange        `json:"field_changes"`
	IssueResponses     []IssueResponse      `json:"issue_responses"`
	AttachmentMetadata []AttachmentMetadata `json:"attachment_metadata,omitempty"`
	RuleVersion        string               `json:"rule_version"`
	FieldRuleImpacts   []FieldRuleImpact    `json:"field_rule_impacts"`
	RuleImpacts        []RuleImpact         `json:"rule_impacts"`
	SubmittedBy        string               `json:"submitted_by"`
	SubmittedAt        time.Time            `json:"submitted_at"`
}

type RuleOutcome string

const (
	RuleOutcomePass    RuleOutcome = "pass"
	RuleOutcomeWarning RuleOutcome = "warning"
	RuleOutcomeBlocker RuleOutcome = "blocker"
)

type RuleImpactDisposition string

const (
	RuleRecalculated RuleImpactDisposition = "recalculated"
	RuleCarried      RuleImpactDisposition = "carried_forward"
)

type FieldRuleImpact struct {
	Field           string   `json:"field"`
	RuleCodes       []string `json:"rule_codes"`
	NoRecalculation bool     `json:"no_recalculation"`
}

type RuleImpact struct {
	RuleCode       string                `json:"rule_code"`
	Disposition    RuleImpactDisposition `json:"disposition"`
	Reason         string                `json:"reason"`
	RuleVersion    string                `json:"rule_version"`
	BeforeOutcome  RuleOutcome           `json:"before_outcome"`
	BeforeEvidence string                `json:"before_evidence,omitempty"`
	AfterOutcome   RuleOutcome           `json:"after_outcome"`
	AfterEvidence  string                `json:"after_evidence,omitempty"`
}

type RevisionEvaluation struct {
	Findings         []PolicyFinding   `json:"findings"`
	FieldRuleImpacts []FieldRuleImpact `json:"field_rule_impacts"`
	RuleImpacts      []RuleImpact      `json:"rule_impacts"`
}

type ApprovalRecord struct {
	ID               string    `json:"id"`
	CaseID           string    `json:"case_id"`
	ApprovedRevision int64     `json:"approved_revision"`
	ApproverName     string    `json:"approver_name"`
	Decision         string    `json:"decision"`
	DecisionNote     string    `json:"decision_note"`
	ContentDigest    string    `json:"content_digest"`
	ApprovedAt       time.Time `json:"approved_at"`
}

type TimelineEntry struct {
	Status     Status    `json:"status"`
	Revision   int64     `json:"revision"`
	Action     string    `json:"action"`
	OccurredAt time.Time `json:"occurred_at"`
}

type ApprovalCheck struct {
	Code    string `json:"code"`
	Label   string `json:"label"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type ApprovalVerification struct {
	Approvable     bool            `json:"approvable"`
	Checks         []ApprovalCheck `json:"checks"`
	Reasons        []string        `json:"reasons,omitempty"`
	ArchiveStatus  string          `json:"archive_status"`
	ComputedDigest string          `json:"computed_digest,omitempty"`
}
