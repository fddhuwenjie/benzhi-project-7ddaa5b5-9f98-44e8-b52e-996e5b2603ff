package store

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

func storePlan() domain.Plan {
	return domain.Plan{TreeCode: "GS-1", Species: "香樟", AgeYears: 180, HealthGrade: "良好", SourceLocation: "甲", DestinationLocation: "乙", MigrationReason: "避让", ConstructionWindow: "2026-11-01/2026-11-03", ProtectionMeasures: "根系、树冠、伤口保护", TrunkDiameterCM: 80, RootBallDiameterCM: 800, TransportDurationHour: 4, DestinationReady: true}
}

func TestSnapshotRecoveryConflictAndIdempotency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "snapshot.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	c, err := domain.NewCase("case-1", storePlan(), now)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(c)
	if err := s.Create(c, "req-create", "create_case", b, now); err != nil {
		t.Fatal(err)
	}
	c.Plan.MigrationReason = "更新"
	c.Revision++
	b, _ = json.Marshal(c)
	if err := s.Save(c, 99, "req-wrong", "update_draft", domain.StatusDraft, b, now); err == nil {
		t.Fatal("期望 revision 冲突")
	} else {
		var conflict *domain.RevisionConflict
		if !errors.As(err, &conflict) || conflict.Current != 1 {
			t.Fatalf("冲突错误异常：%v", err)
		}
	}
	if err := s.Save(c, 1, "req-update", "update_draft", domain.StatusDraft, b, now); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := reopened.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Revision != 2 || restored.Plan.MigrationReason != "更新" {
		t.Fatalf("恢复结果异常：%#v", restored)
	}
	result, ok, err := reopened.LookupOperation("req-update")
	if err != nil || !ok || result.Revision != 2 {
		t.Fatalf("幂等结果未恢复：%#v %v", result, err)
	}
	events, _ := reopened.Events(c.ID)
	if len(events) != 2 {
		t.Fatalf("审计事件数为 %d", len(events))
	}
}
