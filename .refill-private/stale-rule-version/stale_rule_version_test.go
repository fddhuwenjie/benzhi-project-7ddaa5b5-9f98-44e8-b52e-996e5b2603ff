package staleruleversion_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/application"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

func TestRevisionReevaluatesAllRulesAfterVersionChange(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	plan := domain.Plan{TreeCode: "GS-LEGACY-001", Species: "香樟", AgeYears: 180, HealthGrade: "良好", SourceLocation: "甲地", DestinationLocation: "乙地", MigrationReason: "建设避让", ConstructionWindow: "2026-11-01/2026-11-03", ProtectionMeasures: "仅固定树冠", TrunkDiameterCM: 80, RootBallDiameterCM: 800, TransportDurationHour: 4, DestinationReady: true}
	c, err := domain.NewCase("case-legacy", plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyValidation(nil, "2025.1", now); err != nil {
		t.Fatal(err)
	}
	if err := c.StartReview(now); err != nil {
		t.Fatal(err)
	}
	for i, discipline := range []domain.Discipline{domain.DisciplineArboriculture, domain.DisciplineConstruction} {
		opinion := domain.ReviewOpinion{ID: "opinion-" + string(rune('1'+i)), CaseID: c.ID, ReviewRound: c.ReviewRound, Discipline: discipline, ReviewerName: "审查人", Conclusion: "pass", Basis: "旧版规则审查", SubmittedAt: now}
		if err := c.AddOpinion(opinion, now); err != nil {
			t.Fatal(err)
		}
	}

	path := filepath.Join(t.TempDir(), "snapshot.json")
	repo, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	response, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(c, "legacy-import", "create_case", response, now); err != nil {
		t.Fatal(err)
	}
	repo, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, policy.NewEvaluator(func() time.Time { return now }), func() time.Time { return now }, func(prefix string) string { return prefix + "-new" })
	changed := plan
	changed.MigrationReason = "建设避让并补充协调责任"
	result, err := service.SubmitRevision(c.ID, application.SubmitRevisionCommand{CommandMeta: application.CommandMeta{RequestID: "revision-current", Revision: c.Revision}, Plan: changed, SubmittedBy: "编制组"})
	if !errors.Is(err, domain.ErrBlockingFindings) {
		t.Fatalf("规则版本变化后应全量复算并发现保护措施阻断项，实际 err=%v status=%v findings=%v", err, resultStatus(result), resultFindings(result))
	}
}

func resultStatus(c *domain.MigrationCase) domain.Status {
	if c == nil {
		return ""
	}
	return c.Status
}

func resultFindings(c *domain.MigrationCase) []domain.PolicyFinding {
	if c == nil {
		return nil
	}
	return c.Findings
}
