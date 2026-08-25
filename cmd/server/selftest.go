package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

type selftestCase struct {
	ID       string        `json:"id"`
	Status   domain.Status `json:"status"`
	Revision int64         `json:"revision"`
	Plan     domain.Plan   `json:"plan"`
}

func executeSelftest(baseURL string) error {
	client := &http.Client{Timeout: 4 * time.Second}
	var c selftestCase
	plan := domain.Plan{TreeCode: "SELFTEST-001", Species: "香樟", AgeYears: 180, HealthGrade: "良好", SourceLocation: "自检迁出地", DestinationLocation: "自检迁入地", MigrationReason: "市政建设避让并保护古树", ConstructionWindow: "2026-11-10/2026-11-18", ProtectionMeasures: "根系湿包保护，树冠收拢固定，伤口消毒并涂保护剂", TrunkDiameterCM: 80, RootBallDiameterCM: 800, TransportDurationHour: 4, DestinationReady: true}
	if err := selftestRequest(client, http.MethodPost, baseURL+"/api/cases", map[string]any{"request_id": "selftest-create", "plan": plan}, http.StatusCreated, &c); err != nil {
		return err
	}
	if c.Status != domain.StatusDraft {
		return fmt.Errorf("创建后状态为 %s", c.Status)
	}
	steps := []struct {
		path, requestID string
		body            map[string]any
		expected        domain.Status
	}{
		{"validate", "selftest-validate", nil, domain.StatusValidated},
		{"review", "selftest-review", nil, domain.StatusUnderReview},
	}
	for _, step := range steps {
		body := map[string]any{"request_id": step.requestID, "revision": c.Revision}
		for key, value := range step.body {
			body[key] = value
		}
		if err := selftestRequest(client, http.MethodPost, baseURL+"/api/cases/"+c.ID+"/"+step.path, body, http.StatusOK, &c); err != nil {
			return err
		}
		if c.Status != step.expected {
			return fmt.Errorf("%s 后状态为 %s", step.path, c.Status)
		}
	}
	opinions := []map[string]any{
		{"request_id": "selftest-opinion-arbor", "revision": c.Revision, "discipline": "arboriculture", "reviewer_name": "林工", "conclusion": "revise", "basis": "古树迁移养护规范", "issues": []map[string]string{{"code": "ARB-01", "title": "补充保湿复核", "field_path": "protection_measures", "suggestion": "明确保湿复核频次"}}},
		{"request_id": "selftest-opinion-construction", "discipline": "construction", "reviewer_name": "周工", "conclusion": "revise", "basis": "迁移施工协调要点", "issues": []map[string]string{{"code": "CON-01", "title": "明确现场协调", "field_path": "migration_reason", "suggestion": "补充现场协调责任"}}},
	}
	for i, body := range opinions {
		body["revision"] = c.Revision
		if err := selftestRequest(client, http.MethodPost, baseURL+"/api/cases/"+c.ID+"/opinions", body, http.StatusOK, &c); err != nil {
			return err
		}
		if i == 0 && c.Status != domain.StatusUnderReview {
			return fmt.Errorf("单席位意见不应结束会审")
		}
	}
	if c.Status != domain.StatusRevisionRequired {
		return fmt.Errorf("双席位完成后状态为 %s", c.Status)
	}
	plan.MigrationReason += "；整改后由项目负责人承担现场协调。"
	plan.ProtectionMeasures += "；每两小时复核湿度并记录。"
	revisionBody := map[string]any{"request_id": "selftest-revision", "revision": c.Revision, "plan": plan, "submitted_by": "自检编制组", "issue_responses": []map[string]any{{"issue_code": "ARB-01", "response": "已明确每两小时复核", "resolved": true}, {"issue_code": "CON-01", "response": "已明确项目负责人", "resolved": true}}, "attachment_metadata": []map[string]any{{"name": "整改说明.pdf", "content_type": "application/pdf", "size_bytes": 1024, "digest": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}}}
	if err := selftestRequest(client, http.MethodPost, baseURL+"/api/cases/"+c.ID+"/revisions", revisionBody, http.StatusOK, &c); err != nil {
		return err
	}
	if c.Status != domain.StatusResubmitted {
		return fmt.Errorf("整改后状态为 %s", c.Status)
	}
	approveBody := map[string]any{"request_id": "selftest-approval", "revision": c.Revision, "approver_name": "自检批准人", "decision_note": "问题均已关闭"}
	if err := selftestRequest(client, http.MethodPost, baseURL+"/api/cases/"+c.ID+"/approval", approveBody, http.StatusOK, &c); err != nil {
		return err
	}
	if c.Status != domain.StatusApproved {
		return fmt.Errorf("批准后状态为 %s", c.Status)
	}
	var detail struct {
		Case                 selftestCase `json:"case"`
		ApprovalVerification struct {
			ArchiveStatus string `json:"archive_status"`
		} `json:"approval_verification"`
	}
	if err := selftestRequest(client, http.MethodGet, baseURL+"/api/cases/"+c.ID, nil, http.StatusOK, &detail); err != nil {
		return err
	}
	if detail.Case.Status != domain.StatusApproved {
		return fmt.Errorf("详情未恢复 approved 状态")
	}
	if detail.ApprovalVerification.ArchiveStatus != "verified" {
		return fmt.Errorf("批准档案摘要未通过验真：%s", detail.ApprovalVerification.ArchiveStatus)
	}
	return nil
}

func selftestRequest(client *http.Client, method, url string, body any, expected int, target any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	b, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != expected {
		return fmt.Errorf("%s %s 返回 %d：%s", method, url, response.StatusCode, string(b))
	}
	if target != nil && len(b) > 0 {
		if err := json.Unmarshal(b, target); err != nil {
			return fmt.Errorf("解析自检响应失败：%w", err)
		}
	}
	return nil
}
