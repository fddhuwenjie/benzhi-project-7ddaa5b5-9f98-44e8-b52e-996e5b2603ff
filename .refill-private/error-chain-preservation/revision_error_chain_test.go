package error_chain_preservation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/application"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/httpui"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

func TestRevisionBlockingErrorPreservesChain(t *testing.T) {
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	repo, err := store.Open(filepath.Join(t.TempDir(), "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	sequence := 0
	service := application.NewService(repo, policy.NewEvaluator(func() time.Time { return now }), func() time.Time { return now }, func(prefix string) string {
		sequence++
		return prefix + "-" + string(rune('a'+sequence))
	})
	plan := domain.Plan{TreeCode: "CHAIN-001", Species: "香樟", AgeYears: 180, HealthGrade: "良好", SourceLocation: "甲", DestinationLocation: "乙", MigrationReason: "避让", ConstructionWindow: "2026-11-01/2026-11-03", ProtectionMeasures: "根系、树冠、伤口保护", TrunkDiameterCM: 80, RootBallDiameterCM: 800, TransportDurationHour: 4, DestinationReady: true}
	c, err := service.CreateCase(application.CreateCaseCommand{RequestID: "create-chain", Plan: plan})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.ValidateCase(c.ID, application.ValidateCommand{CommandMeta: application.CommandMeta{RequestID: "validate-chain", Revision: c.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.StartReview(c.ID, application.StartReviewCommand{CommandMeta: application.CommandMeta{RequestID: "review-chain", Revision: c.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	for i, discipline := range []domain.Discipline{domain.DisciplineArboriculture, domain.DisciplineConstruction} {
		c, err = service.SubmitOpinion(c.ID, application.SubmitOpinionCommand{CommandMeta: application.CommandMeta{RequestID: "opinion-chain-" + string(rune('a'+i)), Revision: c.Revision}, Discipline: discipline, ReviewerName: "审查人", Conclusion: "pass", Basis: "规范"})
		if err != nil {
			t.Fatal(err)
		}
	}
	plan.DestinationReady = false
	body, err := json.Marshal(application.SubmitRevisionCommand{CommandMeta: application.CommandMeta{RequestID: "revision-chain", Revision: c.Revision}, Plan: plan, SubmittedBy: "编制人"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+c.ID+"/revisions", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	httpui.New(service).Handler().ServeHTTP(response, req)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"blocking_findings"`) {
		t.Fatalf("阻断性整改错误链丢失：status=%d body=%s", response.Code, response.Body.String())
	}
}
