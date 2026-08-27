package revisionfinding

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
)

func TestRevisionFindingIdentityRefresh(t *testing.T) {
	base := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	clock := base
	evaluator := policy.NewEvaluator(func() time.Time {
		current := clock
		clock = clock.Add(time.Minute)
		return current
	})
	plan := domain.Plan{
		TreeCode: "GS-REVISION-IDENTITY", Species: "香樟", AgeYears: 180, HealthGrade: "良好",
		SourceLocation: "甲", DestinationLocation: "乙", MigrationReason: "避让",
		ConstructionWindow: "2026-11-01/2026-11-03", ProtectionMeasures: "根系、树冠、伤口保护",
		TrunkDiameterCM: 80, RootBallDiameterCM: 800, TransportDurationHour: 8, DestinationReady: true,
	}
	previous := evaluator.Evaluate("case-identity", 1, plan)
	if len(previous) != 1 || previous[0].RuleCode != "TRANSPORT-001" {
		t.Fatalf("测试前置条件不成立：%#v", previous)
	}

	updated := plan
	updated.MigrationReason = "道路避让"
	evaluation := evaluator.EvaluateRevision(
		"case-identity", 2, updated,
		[]domain.FieldChange{{Field: "migration_reason", Before: plan.MigrationReason, After: updated.MigrationReason}},
		previous,
	)
	if len(evaluation.Findings) != 1 {
		t.Fatalf("整改后 finding 数量异常：%#v", evaluation.Findings)
	}
	finding := evaluation.Findings[0]
	if finding.CaseRevision != 2 {
		t.Fatalf("整改后 finding revision 错误：%d", finding.CaseRevision)
	}
	hash := sha256.Sum256([]byte("case-identity|TRANSPORT-001|8|2"))
	wantID := "finding-" + hex.EncodeToString(hash[:8])
	if finding.ID != wantID {
		t.Fatalf("整改后 finding 身份仍复用旧 revision：got=%s want=%s", finding.ID, wantID)
	}
	if !finding.EvaluatedAt.After(previous[0].EvaluatedAt) {
		t.Fatalf("整改后 finding 仍携带旧评估时间：got=%s previous=%s", finding.EvaluatedAt, previous[0].EvaluatedAt)
	}
	if finding.ID == previous[0].ID || finding.ID == "" {
		t.Fatalf("整改后 finding ID 未形成新版本身份：%s", finding.ID)
	}
}
