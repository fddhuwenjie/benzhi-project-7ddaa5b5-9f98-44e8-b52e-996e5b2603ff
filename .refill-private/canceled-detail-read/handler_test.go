package canceled_detail_read

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/application"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/httpui"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

type countingRepository struct {
	*store.JSONStore
	getCalls atomic.Int32
	entered  chan struct{}
	release  chan struct{}
}

func (r *countingRepository) Get(id string) (*domain.MigrationCase, error) {
	r.getCalls.Add(1)
	close(r.entered)
	<-r.release
	return r.JSONStore.Get(id)
}

func detailPlan() domain.Plan {
	return domain.Plan{
		TreeCode: "GS-CANCEL-DETAIL", Species: "香樟", AgeYears: 180, HealthGrade: "良好",
		SourceLocation: "甲", DestinationLocation: "乙", MigrationReason: "避让",
		ConstructionWindow: "2026-11-01/2026-11-03", ProtectionMeasures: "根系、树冠、伤口保护",
		TrunkDiameterCM: 80, RootBallDiameterCM: 800, TransportDurationHour: 4, DestinationReady: true,
	}
}

func TestCanceledDetailDoesNotTouchRepository(t *testing.T) {
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	repo, err := store.Open(filepath.Join(t.TempDir(), "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := domain.NewCase("case-canceled-detail", detailPlan(), now)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(c)
	if err := repo.Create(c, "create-canceled-detail", "create_case", payload, now); err != nil {
		t.Fatal(err)
	}
	counting := &countingRepository{JSONStore: repo, entered: make(chan struct{}), release: make(chan struct{})}
	service := application.NewService(counting, policy.NewEvaluator(func() time.Time { return now }), func() time.Time { return now }, func(prefix string) string { return prefix + "-private" })
	server := httpui.New(service)
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/api/cases/case-canceled-detail", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.Handler().ServeHTTP(response, request)
		close(done)
	}()
	<-counting.entered
	cancel()
	close(counting.release)
	<-done
	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("取消请求应返回 request_canceled，状态码=%d", response.Code)
	}
	if got := counting.getCalls.Load(); got != 0 {
		t.Fatalf("取消请求不应触达仓储，实际 Get 调用 %d 次", got)
	}
}
