package application

import (
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

type CommandMeta struct {
	RequestID string `json:"request_id"`
	Revision  int64  `json:"revision"`
}

type CreateCaseCommand struct {
	RequestID string      `json:"request_id"`
	Plan      domain.Plan `json:"plan"`
}

type UpdateDraftCommand struct {
	CommandMeta
	Plan domain.Plan `json:"plan"`
}

type ValidateCommand struct{ CommandMeta }
type StartReviewCommand struct{ CommandMeta }

type SubmitOpinionCommand struct {
	CommandMeta
	Discipline   domain.Discipline    `json:"discipline"`
	ReviewerName string               `json:"reviewer_name"`
	Conclusion   string               `json:"conclusion"`
	Issues       []domain.ReviewIssue `json:"issues"`
	Basis        string               `json:"basis"`
}

type SubmitRevisionCommand struct {
	CommandMeta
	Plan               domain.Plan                 `json:"plan"`
	IssueResponses     []domain.IssueResponse      `json:"issue_responses"`
	AttachmentMetadata []domain.AttachmentMetadata `json:"attachment_metadata"`
	SubmittedBy        string                      `json:"submitted_by"`
}

type ApproveCommand struct {
	CommandMeta
	ApproverName string `json:"approver_name"`
	DecisionNote string `json:"decision_note"`
}

type CaseDetail struct {
	Case                 *domain.MigrationCase       `json:"case"`
	AuditEvents          []store.AuditEvent          `json:"audit_events"`
	Summary              ReviewSummary               `json:"summary"`
	ApprovalVerification domain.ApprovalVerification `json:"approval_verification"`
}

type ReviewSummary struct {
	CompletedSeats   int  `json:"completed_seats"`
	TotalSeats       int  `json:"total_seats"`
	BlockingFindings int  `json:"blocking_findings"`
	WarningFindings  int  `json:"warning_findings"`
	ResolvedIssues   int  `json:"resolved_issues"`
	UnresolvedIssues int  `json:"unresolved_issues"`
	RevisionVersions int  `json:"revision_versions"`
	Approvable       bool `json:"approvable"`
}

type ListCasesQuery struct {
	Statuses []domain.Status
	TreeCode string
	Species  string
	Location string
	Sort     string
	Page     int
	PageSize int
}

type QueueAttention struct {
	BlockingFindings int `json:"blocking_findings"`
	PendingSeats     int `json:"pending_seats"`
	PendingResponses int `json:"pending_responses"`
}

type CaseListResult struct {
	Cases        []*domain.MigrationCase `json:"cases"`
	StatusCounts map[domain.Status]int   `json:"status_counts"`
	Attention    QueueAttention          `json:"attention"`
	Page         int                     `json:"page"`
	PageSize     int                     `json:"page_size"`
	Total        int                     `json:"total"`
	TotalPages   int                     `json:"total_pages"`
}
