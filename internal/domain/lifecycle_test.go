package domain

import (
	"errors"
	"testing"
	"time"
)

func validPlan() Plan {
	return Plan{TreeCode: "GS-1", Species: "香樟", AgeYears: 180, HealthGrade: "良好", SourceLocation: "甲地", DestinationLocation: "乙地", MigrationReason: "建设避让", ConstructionWindow: "2026-11-01/2026-11-10", ProtectionMeasures: "根系保护、树冠固定、伤口处理", TrunkDiameterCM: 80, RootBallDiameterCM: 800, TransportDurationHour: 4, DestinationReady: true}
}

func TestLifecycleRequiresBothReviewsAndResolvedRevision(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	c, err := NewCase("case-1", validPlan(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyValidation(nil, "test", now); err != nil {
		t.Fatal(err)
	}
	if err := c.StartReview(now); err != nil {
		t.Fatal(err)
	}
	first := ReviewOpinion{ID: "o1", CaseID: c.ID, ReviewRound: 1, Discipline: DisciplineArboriculture, ReviewerName: "林工", Conclusion: "revise", Basis: "规范", Issues: []ReviewIssue{{Code: "A-1", Title: "补充保护", FieldPath: "protection_measures", Suggestion: "明确复核"}}, SubmittedAt: now}
	if err := c.AddOpinion(first, now); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusUnderReview {
		t.Fatalf("单席位后状态为 %s", c.Status)
	}
	second := ReviewOpinion{ID: "o2", CaseID: c.ID, ReviewRound: 1, Discipline: DisciplineConstruction, ReviewerName: "周工", Conclusion: "pass", Basis: "要点", SubmittedAt: now}
	if err := c.AddOpinion(second, now); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusRevisionRequired || len(c.ModificationItems) != 1 {
		t.Fatalf("汇总结果异常：%s %#v", c.Status, c.ModificationItems)
	}
	changed := validPlan()
	changed.ProtectionMeasures += "；每两小时复核"
	sub := RevisionSubmission{ID: "r1", CaseID: c.ID, FromRevision: c.Revision, IssueResponses: []IssueResponse{{IssueCode: "A-1", Response: "已补充", Resolved: true}}, SubmittedBy: "编制组", SubmittedAt: now}
	if err := c.SubmitRevision(sub, changed, nil, now); err != nil {
		t.Fatal(err)
	}
	if len(c.RevisionSubmissions[0].FieldChanges) != 1 {
		t.Fatalf("未记录版本差异：%#v", c.RevisionSubmissions[0])
	}
	digest, err := c.ApprovalDigest()
	if err != nil || len(digest) != 64 {
		t.Fatalf("摘要无效：%q %v", digest, err)
	}
	record := ApprovalRecord{ID: "a1", CaseID: c.ID, ApproverName: "负责人", Decision: "approved", ContentDigest: digest, ApprovedAt: now}
	if err := c.Approve(record, now); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusApproved {
		t.Fatalf("批准状态为 %s", c.Status)
	}
	if verification := c.ApprovalReadiness("test"); verification.ArchiveStatus != "verified" {
		t.Fatalf("批准摘要未通过验真：%#v", verification)
	}
	c.Plan.MigrationReason = "摘要形成后被改写"
	if verification := c.ApprovalReadiness("test"); verification.ArchiveStatus != "mismatch" || c.Approval.ApproverName != "负责人" {
		t.Fatalf("档案异常未被识别或批准记录被覆盖：%#v", verification)
	}
	if err := c.StartReview(now); !errors.Is(err, ErrAlreadyApproved) {
		t.Fatalf("批准后仍可修改：%v", err)
	}
}

func TestValidateDraftRejectsInvalidWindow(t *testing.T) {
	plan := validPlan()
	plan.ConstructionWindow = "2026-12-10/2026-11-01"
	var fields *ValidationErrors
	if err := ValidateDraft(plan); !errors.As(err, &fields) || fields.Fields["construction_window"] == "" {
		t.Fatalf("未返回窗口字段错误：%v", err)
	}
}
