package application

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

func appPlan() domain.Plan {
	return domain.Plan{TreeCode: "GS-1", Species: "香樟", AgeYears: 180, HealthGrade: "良好", SourceLocation: "甲", DestinationLocation: "乙", MigrationReason: "避让", ConstructionWindow: "2026-11-01/2026-11-03", ProtectionMeasures: "根系、树冠、伤口保护", TrunkDiameterCM: 80, RootBallDiameterCM: 800, TransportDurationHour: 4, DestinationReady: true}
}

func TestConcurrentEquivalentTreeCodesAndReplay(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	var idMu sync.Mutex
	sequence := 0
	s := NewService(repo, policy.NewEvaluator(func() time.Time { return now }), func() time.Time { return now }, func(prefix string) string {
		idMu.Lock()
		defer idMu.Unlock()
		sequence++
		return prefix + "-" + itoaForTest(sequence)
	})
	commands := []CreateCaseCommand{{RequestID: "req-a", Plan: appPlan()}, {RequestID: "req-b", Plan: appPlan()}}
	commands[0].Plan.TreeCode = "GS-2026-001"
	commands[1].Plan.TreeCode = "  gs-2026-001  "
	type result struct {
		c   *domain.MigrationCase
		err error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, cmd := range commands {
		wg.Add(1)
		go func(cmd CreateCaseCommand) {
			defer wg.Done()
			c, err := s.CreateCase(cmd)
			results <- result{c, err}
		}(cmd)
	}
	wg.Wait()
	close(results)
	var success *domain.MigrationCase
	successRequest := ""
	conflicts := 0
	for result := range results {
		if result.err == nil {
			success = result.c
			if result.c.Plan.TreeCode == commands[0].Plan.TreeCode {
				successRequest = commands[0].RequestID
			} else {
				successRequest = commands[1].RequestID
			}
			continue
		}
		var conflict *domain.TreeCodeConflict
		if !errors.As(result.err, &conflict) || conflict.CaseID == "" || conflict.Status != domain.StatusDraft {
			t.Fatalf("冲突信息无效：%v", result.err)
		}
		conflicts++
	}
	if success == nil || conflicts != 1 {
		t.Fatalf("并发创建结果异常：success=%v conflicts=%d", success, conflicts)
	}
	replayed, err := s.CreateCase(CreateCaseCommand{RequestID: successRequest, Plan: success.Plan})
	if err != nil || replayed.ID != success.ID || replayed.Revision != success.Revision {
		t.Fatalf("成功请求重放不稳定：%#v %v", replayed, err)
	}
}

func TestValidationBatchReplayAndQueryPaging(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	sequence := 0
	s := NewService(repo, policy.NewEvaluator(func() time.Time { return now }), func() time.Time { return now }, func(prefix string) string { sequence++; return prefix + "-" + itoaForTest(sequence) })
	for i := 1; i <= 3; i++ {
		plan := appPlan()
		plan.TreeCode = "GS-PAGE-" + itoaForTest(i)
		plan.Species = "银杏 Alpha"
		if _, err := s.CreateCase(CreateCaseCommand{RequestID: "create-" + itoaForTest(i), Plan: plan}); err != nil {
			t.Fatal(err)
		}
	}
	first := mustFindCase(t, s, "GS-PAGE-1")
	validated, err := s.ValidateCase(first.ID, ValidateCommand{CommandMeta{RequestID: "validate-once", Revision: first.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := s.ValidateCase(first.ID, ValidateCommand{CommandMeta{RequestID: "validate-once", Revision: first.Revision}})
	if err != nil || len(validated.ValidationBatches) != 1 || len(replayed.ValidationBatches) != 1 || replayed.ValidationBatches[0].ID != validated.ValidationBatches[0].ID {
		t.Fatalf("校验批次重放异常：%#v %v", replayed.ValidationBatches, err)
	}
	page1, err := s.QueryCases(ListCasesQuery{Species: "  银杏   alpha ", Sort: "tree_code", Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	page2, err := s.QueryCases(ListCasesQuery{Species: "银杏 alpha", Sort: "tree_code", Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page1.Total != 3 || len(page1.Cases) != 2 || len(page2.Cases) != 1 || page1.Cases[1].ID == page2.Cases[0].ID {
		t.Fatalf("稳定分页异常：%#v %#v", page1, page2)
	}
	if _, err := s.QueryCases(ListCasesQuery{Sort: "unknown", Page: 1, PageSize: 20}); err == nil {
		t.Fatal("未知排序字段未被拒绝")
	}
}

func mustFindCase(t *testing.T, s *Service, treeCode string) *domain.MigrationCase {
	t.Helper()
	result, err := s.QueryCases(ListCasesQuery{TreeCode: treeCode, Page: 1, PageSize: 20})
	if err != nil || len(result.Cases) != 1 {
		t.Fatalf("未找到测试个案 %s：%v", treeCode, err)
	}
	return result.Cases[0]
}

func itoaForTest(value int) string {
	if value < 10 {
		return string(rune('0' + value))
	}
	return "n"
}

func TestIdempotencyAndRevisionConflict(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	sequence := 0
	s := NewService(repo, policy.NewEvaluator(func() time.Time { return now }), func() time.Time { return now }, func(prefix string) string { sequence++; return prefix + "-fixed" })
	created, err := s.CreateCase(CreateCaseCommand{RequestID: "req-1", Plan: appPlan()})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := s.CreateCase(CreateCaseCommand{RequestID: "req-1", Plan: appPlan()})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != replayed.ID || sequence != 1 {
		t.Fatalf("创建未幂等：%s %s sequence=%d", created.ID, replayed.ID, sequence)
	}
	_, err = s.ValidateCase(created.ID, ValidateCommand{CommandMeta{RequestID: "req-conflict", Revision: 99}})
	var conflict *domain.RevisionConflict
	if !errors.As(err, &conflict) || conflict.Current != 1 {
		t.Fatalf("未返回当前 revision：%v", err)
	}
	validated, err := s.ValidateCase(created.ID, ValidateCommand{CommandMeta{RequestID: "req-validate", Revision: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if validated.Status != domain.StatusValidated {
		t.Fatalf("状态为 %s", validated.Status)
	}
	replayedValidation, err := s.ValidateCase(created.ID, ValidateCommand{CommandMeta{RequestID: "req-validate", Revision: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if replayedValidation.Revision != validated.Revision {
		t.Fatal("状态变更未返回首次结果")
	}
}
