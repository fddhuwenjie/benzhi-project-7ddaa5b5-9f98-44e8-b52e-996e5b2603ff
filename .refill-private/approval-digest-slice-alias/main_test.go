package approvaldigestalias

import (
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

func TestApprovalDigestDoesNotReorderAggregate(t *testing.T) {
	first := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	caseData := &domain.MigrationCase{
		ID:   "case-digest-alias",
		Plan: domain.Plan{TreeCode: "GS-DIGEST", Species: "香樟"},
		Findings: []domain.PolicyFinding{
			{RuleCode: "rule-z"},
			{RuleCode: "rule-a"},
		},
		Opinions: []domain.ReviewOpinion{
			{ReviewRound: 2, Discipline: domain.DisciplineConstruction, Issues: []domain.ReviewIssue{{Code: "Z-ISSUE"}, {Code: "A-ISSUE"}}},
			{ReviewRound: 1, Discipline: domain.DisciplineArboriculture},
		},
		ValidationBatches: []domain.ValidationBatch{
			{ID: "batch-later", EvaluatedAt: second},
			{ID: "batch-earlier", EvaluatedAt: first},
		},
		RevisionSubmissions: []domain.RevisionSubmission{
			{ID: "revision-2", ToRevision: 2, FieldChanges: []domain.FieldChange{{Field: "z_field"}, {Field: "a_field"}}, IssueResponses: []domain.IssueResponse{{IssueCode: "Z-ISSUE"}, {IssueCode: "A-ISSUE"}}, AttachmentMetadata: []domain.AttachmentMetadata{{Name: "z.pdf"}, {Name: "a.pdf"}}},
			{ID: "revision-1", ToRevision: 1},
		},
	}

	if _, err := caseData.ApprovalDigest(); err != nil {
		t.Fatal(err)
	}
	if got := caseData.Findings[0].RuleCode; got != "rule-z" {
		t.Fatalf("摘要计算改写了 findings 顺序：首项为 %q", got)
	}
	if got := caseData.Opinions[0].ReviewRound; got != 2 {
		t.Fatalf("摘要计算改写了 opinions 顺序：首轮为 %d", got)
	}
	if got := caseData.Opinions[0].Issues[0].Code; got != "Z-ISSUE" {
		t.Fatalf("摘要计算改写了 opinion issues 顺序：首项为 %q", got)
	}
	if got := caseData.ValidationBatches[0].ID; got != "batch-later" {
		t.Fatalf("摘要计算改写了 validation_batches 顺序：首项为 %q", got)
	}
	if got := caseData.RevisionSubmissions[0].ID; got != "revision-2" {
		t.Fatalf("摘要计算改写了 revision_submissions 顺序：首项为 %q", got)
	}
	if got := caseData.RevisionSubmissions[0].FieldChanges[0].Field; got != "z_field" {
		t.Fatalf("摘要计算改写了 field_changes 顺序：首项为 %q", got)
	}
	if got := caseData.RevisionSubmissions[0].IssueResponses[0].IssueCode; got != "Z-ISSUE" {
		t.Fatalf("摘要计算改写了 issue_responses 顺序：首项为 %q", got)
	}
	if got := caseData.RevisionSubmissions[0].AttachmentMetadata[0].Name; got != "z.pdf" {
		t.Fatalf("摘要计算改写了 attachment_metadata 顺序：首项为 %q", got)
	}
}
