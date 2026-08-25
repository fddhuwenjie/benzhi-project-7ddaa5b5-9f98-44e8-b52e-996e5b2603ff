package httpui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/application"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	repo, err := store.Open(filepath.Join(t.TempDir(), "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo, policy.NewEvaluator(time.Now), time.Now, nil)
	return New(service).Handler()
}

func TestListQueryAndTreeCodeConflictBoundary(t *testing.T) {
	h := testHandler(t)
	invalid := httptest.NewRecorder()
	h.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/cases?sort=unknown", nil))
	if invalid.Code != http.StatusUnprocessableEntity || !strings.Contains(invalid.Body.String(), `"sort"`) {
		t.Fatalf("非法排序边界异常：%d %s", invalid.Code, invalid.Body.String())
	}
	plan := applicationTestPlan()
	createBody := map[string]any{"request_id": "http-create-1", "plan": plan}
	createJSON, _ := json.Marshal(createBody)
	created := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(string(createJSON)))
	request.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(created, request)
	if created.Code != http.StatusCreated {
		t.Fatalf("创建失败：%d %s", created.Code, created.Body.String())
	}
	plan["tree_code"] = "  gs-http-001  "
	conflictJSON, _ := json.Marshal(map[string]any{"request_id": "http-create-2", "plan": plan})
	conflict := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(string(conflictJSON)))
	request.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(conflict, request)
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), `"tree_code_conflict"`) || !strings.Contains(conflict.Body.String(), `"conflict_case_id"`) || !strings.Contains(conflict.Body.String(), `"tree_code"`) {
		t.Fatalf("编号冲突响应异常：%d %s", conflict.Code, conflict.Body.String())
	}
}

func applicationTestPlan() map[string]any {
	return map[string]any{
		"tree_code": "GS-HTTP-001", "species": "香樟", "age_years": 180, "health_grade": "良好",
		"source_location": "甲", "destination_location": "乙", "migration_reason": "建设避让",
		"construction_window": "2026-11-01/2026-11-03", "protection_measures": "根系、树冠、伤口保护",
		"trunk_diameter_cm": 80, "root_ball_diameter_cm": 800, "transport_duration_hours": 4, "destination_ready": true,
	}
}

func TestWorkbenchAndJSONBoundary(t *testing.T) {
	h := testHandler(t)
	page := httptest.NewRecorder()
	h.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "<body>") {
		t.Fatalf("工作台响应异常：%d", page.Code)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType || !strings.Contains(response.Body.String(), "unsupported_media_type") {
		t.Fatalf("内容类型边界异常：%d %s", response.Code, response.Body.String())
	}
}
