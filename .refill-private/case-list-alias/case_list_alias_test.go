package caselistaliastest

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/application"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

func TestCaseListRemainsIsolatedFromStore(t *testing.T) {
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	repo, err := store.Open(filepath.Join(t.TempDir(), "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	plan := domain.Plan{
		TreeCode: "GS-PRIVATE-001", Species: "香樟", AgeYears: 180, HealthGrade: "良好",
		SourceLocation: "甲", DestinationLocation: "乙", MigrationReason: "避让",
		ConstructionWindow: "2026-11-01/2026-11-03", ProtectionMeasures: "根系、树冠、伤口保护",
		TrunkDiameterCM: 80, RootBallDiameterCM: 800, TransportDurationHour: 4, DestinationReady: true,
	}
	c, err := domain.NewCase("case-private", plan, now)
	if err != nil {
		t.Fatal(err)
	}
	response, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(c, "request-private", "create_case", response, now); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, policy.NewEvaluator(func() time.Time { return now }), func() time.Time { return now }, nil)
	list, err := service.QueryCases(application.ListCasesQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Cases) != 1 {
		t.Fatalf("查询个案数量异常：%d", len(list.Cases))
	}
	list.Cases[0].Plan.MigrationReason = "调用方伪造的理由"
	latest, err := service.GetCase(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Case.Plan.MigrationReason == "调用方伪造的理由" {
		t.Fatalf("调用方修改查询结果污染了仓储个案状态：%q", latest.Case.Plan.MigrationReason)
	}
}
