package canceledrequest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/application"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/httpui"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

type cancelingRepository struct {
	application.Repository
	cancel context.CancelFunc
}

func (r *cancelingRepository) CreateWithAudit(c *domain.MigrationCase, requestID, action string, response json.RawMessage, now time.Time, audit store.AuditContext) error {
	r.cancel()
	return r.Repository.CreateWithAudit(c, requestID, action, response, now, audit)
}

func TestCanceledCreateDoesNotPersistSideEffect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	baseRepo, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)

	body, err := json.Marshal(application.CreateCaseCommand{
		RequestID: "req-canceled-create",
		Plan: domain.Plan{
			TreeCode: "GS-CANCEL-1", Species: "香樟", AgeYears: 180, HealthGrade: "良好",
			SourceLocation: "甲地", DestinationLocation: "乙地", MigrationReason: "避让",
			ConstructionWindow: "2026-11-01/2026-11-03", ProtectionMeasures: "根系保护",
			TrunkDiameterCM: 80, RootBallDiameterCM: 800, TransportDurationHour: 4, DestinationReady: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	repo := &cancelingRepository{Repository: baseRepo, cancel: cancel}
	service := application.NewService(repo, policy.NewEvaluator(func() time.Time { return now }), func() time.Time { return now }, func(prefix string) string { return prefix + "-test" })
	server := httpui.New(service)
	req := httptest.NewRequest(http.MethodPost, "/api/cases", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Handler().ServeHTTP(resp, req)

	cases, err := repo.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 0 {
		t.Fatalf("已取消的创建请求不应持久化个案，实际数量=%d", len(cases))
	}
}
