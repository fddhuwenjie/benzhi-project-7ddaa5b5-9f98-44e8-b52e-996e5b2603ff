package idempotencypayloadreuse_test

import (
	"bytes"
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

func TestIdempotencyRejectsChangedPayload(t *testing.T) {
	h := newHandler(t)
	first := validPlan("GS-IDEM-001")
	second := validPlan("GS-IDEM-002")

	if status := postJSON(t, h, "/api/cases", map[string]any{"request_id": "same-request", "plan": first}); status != http.StatusCreated {
		t.Fatalf("首次创建返回 %d", status)
	}
	if status := postJSON(t, h, "/api/cases", map[string]any{"request_id": "same-request", "plan": second}); status != http.StatusConflict {
		t.Fatalf("相同 request_id 携带不同载荷应返回冲突，实际为 %d", status)
	}
}

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	repo, err := store.Open(filepath.Join(t.TempDir(), "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) }
	return httpui.New(application.NewService(repo, policy.NewEvaluator(now), now, nil)).Handler()
}

func validPlan(treeCode string) domain.Plan {
	return domain.Plan{TreeCode: treeCode, Species: "香樟", AgeYears: 180, HealthGrade: "良好", SourceLocation: "甲地", DestinationLocation: "乙地", MigrationReason: "建设避让", ConstructionWindow: "2026-11-01/2026-11-03", ProtectionMeasures: "根系、树冠和伤口保护", TrunkDiameterCM: 80, RootBallDiameterCM: 800, TransportDurationHour: 4, DestinationReady: true}
}

func postJSON(t *testing.T, h http.Handler, path string, body any) int {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code
}
