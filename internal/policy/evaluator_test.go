package policy

import (
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

func TestEvaluatorProducesStableSortedFindings(t *testing.T) {
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	e := NewEvaluator(func() time.Time { return now })
	plan := domain.Plan{ConstructionWindow: "2026-07-01/2026-07-10", TrunkDiameterCM: 80, RootBallDiameterCM: 400, ProtectionMeasures: "仅树冠固定", TransportDurationHour: 13, DestinationReady: false}
	first := e.Evaluate("case-1", 2, plan)
	second := e.Evaluate("case-1", 2, plan)
	if len(first) != 5 {
		t.Fatalf("期望五项结果，得到 %d", len(first))
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Fatalf("结果 ID 不稳定")
		}
		if i > 0 && first[i-1].RuleCode > first[i].RuleCode {
			t.Fatalf("规则结果未排序")
		}
	}
}

func TestEvaluateAffectedRetainsUnchangedFinding(t *testing.T) {
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	e := NewEvaluator(func() time.Time { return now })
	plan := domain.Plan{ConstructionWindow: "2026-07-01/2026-07-10", TrunkDiameterCM: 80, RootBallDiameterCM: 400, ProtectionMeasures: "根、冠、伤口", TransportDurationHour: 4, DestinationReady: true}
	previous := e.Evaluate("case-1", 2, plan)
	plan.RootBallDiameterCM = 800
	results := e.EvaluateAffected("case-1", 3, plan, []domain.FieldChange{{Field: "root_ball_diameter_cm"}}, previous)
	if len(results) != 1 || results[0].RuleCode != "SEASON-001" || results[0].CaseRevision != 3 {
		t.Fatalf("受影响重算异常：%#v", results)
	}
}
