package application

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
)

type Service struct {
	repo   Repository
	policy PolicyEvaluator
	now    func() time.Time
	newID  IDGenerator
}

func NewService(repo Repository, evaluator PolicyEvaluator, now func() time.Time, ids IDGenerator) *Service {
	if now == nil {
		now = time.Now
	}
	if ids == nil {
		ids = randomID
	}
	return &Service{repo: repo, policy: evaluator, now: now, newID: ids}
}

func (s *Service) GetCase(id string) (*CaseDetail, error) {
	c, err := s.repo.Get(id)
	if err != nil {
		return nil, err
	}
	events, err := s.repo.Events(id)
	if err != nil {
		return nil, err
	}
	return &CaseDetail{Case: c, AuditEvents: events, Summary: summarize(c), ApprovalVerification: c.ApprovalReadiness(policy.Version)}, nil
}

func (s *Service) ListCases() ([]*domain.MigrationCase, error) { return s.repo.List() }

func (s *Service) ensureTreeCodeAvailable(treeCode, excludeCaseID string) error {
	conflict, found, err := s.repo.FindByTreeCode(domain.NormalizeTreeCode(treeCode), excludeCaseID)
	if err != nil {
		return err
	}
	if found {
		return &domain.TreeCodeConflict{CaseID: conflict.ID, Status: conflict.Status}
	}
	return nil
}

func (s *Service) RuleCatalog() any {
	return struct {
		Version string `json:"version"`
		Rules   any    `json:"rules"`
	}{policy.Version, policy.RuleDescriptions()}
}

func (s *Service) replay(requestID, action, requestDigest string) (*domain.MigrationCase, bool, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, false, fieldError("request_id", "不能为空")
	}
	result, ok, err := s.repo.LookupOperation(requestID)
	if err != nil || !ok {
		return nil, false, err
	}
	if result.Action != action {
		return nil, false, fmt.Errorf("request_id 已用于操作 %s", result.Action)
	}
	if result.RequestDigest != "" && requestDigest != result.RequestDigest {
		return nil, false, &domain.IdempotencyConflict{RequestID: requestID, Action: action}
	}
	var c domain.MigrationCase
	if err := json.Unmarshal(result.Response, &c); err != nil {
		return nil, false, fmt.Errorf("读取幂等结果失败：%w", err)
	}
	return &c, true, nil
}

func (s *Service) loadForMutation(id string, meta CommandMeta, action, requestDigest string) (*domain.MigrationCase, bool, error) {
	if replay, ok, err := s.replay(meta.RequestID, action, requestDigest); ok || err != nil {
		if ok && replay.ID != id {
			return nil, false, fmt.Errorf("request_id 已用于其他个案")
		}
		return replay, ok, err
	}
	if meta.Revision < 1 {
		return nil, false, fieldError("revision", "须大于 0")
	}
	c, err := s.repo.Get(id)
	if err != nil {
		return nil, false, err
	}
	if c.Revision != meta.Revision {
		return nil, false, &domain.RevisionConflict{Expected: meta.Revision, Current: c.Revision}
	}
	return c, false, nil
}

func marshalCase(c *domain.MigrationCase) (json.RawMessage, error) { return json.Marshal(c) }

func digestPayload(payload any) (string, error) {
	if payload == nil {
		return "", nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func fieldError(field, message string) error {
	e := domain.NewValidationErrors()
	e.Add(field, message)
	return e
}

func randomID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic("系统随机源不可用：" + err.Error())
	}
	return prefix + "-" + hex.EncodeToString(b)
}

func validateIdentity(value, field string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fieldError(field, "不能为空")
	}
	if len(value) > 128 {
		return fieldError(field, "长度不能超过 128")
	}
	return nil
}

func isBlocking(err error) bool { return errors.Is(err, domain.ErrBlockingFindings) }

func summarize(c *domain.MigrationCase) ReviewSummary {
	summary := ReviewSummary{TotalSeats: len(c.ReviewSeats), RevisionVersions: len(c.RevisionSubmissions)}
	for _, seat := range c.ReviewSeats {
		if seat.Completed {
			summary.CompletedSeats++
		}
	}
	for _, finding := range c.Findings {
		if finding.Severity == domain.SeverityBlocker {
			summary.BlockingFindings++
		} else if finding.Severity == domain.SeverityWarning {
			summary.WarningFindings++
		}
	}
	for _, item := range c.ModificationItems {
		if item.Resolved {
			summary.ResolvedIssues++
		} else {
			summary.UnresolvedIssues++
		}
	}
	summary.Approvable = c.Status == domain.StatusResubmitted && summary.BlockingFindings == 0 && summary.UnresolvedIssues == 0
	return summary
}
