package application

import (
	"fmt"
	"strings"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/store"
)

func (s *Service) ValidateCase(id string, cmd ValidateCommand) (*domain.MigrationCase, error) {
	const action = "validate_case"
	c, replayed, err := s.loadForMutation(id, cmd.CommandMeta, action, "")
	if err != nil {
		return c, err
	}
	if replayed {
		if domain.HasBlockers(c.Findings) {
			return c, domain.ErrBlockingFindings
		}
		return c, nil
	}
	expected, from, now := c.Revision, c.Status, s.now().UTC()
	findings := s.policy.Evaluate(c.ID, c.Revision, c.Plan)
	var previous []domain.PolicyFinding
	if count := len(c.ValidationBatches); count > 0 {
		previous = c.ValidationBatches[count-1].Findings
	}
	batch := domain.ValidationBatch{ID: s.newID("validation"), CaseID: c.ID, CaseRevision: c.Revision, RuleVersion: policy.Version, EvaluatedAt: now, Findings: findings, Conclusions: domain.BuildValidationConclusions(findings, previous, policy.Applicability())}
	transitionErr := c.ApplyValidationBatch(batch, now)
	if transitionErr != nil && !isBlocking(transitionErr) {
		return nil, transitionErr
	}
	response, err := marshalCase(c)
	if err != nil {
		return nil, err
	}
	result := fmt.Sprintf("完成规则批次 %s：%d 个阻断项，%d 个警示项", batch.ID, countSeverity(findings, domain.SeverityBlocker), countSeverity(findings, domain.SeverityWarning))
	if err := s.repo.SaveWithAudit(c, expected, cmd.RequestID, action, "", from, response, now, store.AuditContext{Actor: "系统角色：技术规则校验", Result: result}); err != nil {
		return s.replayAfterConflict(cmd.RequestID, action, "", err)
	}
	return c, transitionErr
}

func (s *Service) StartReview(id string, cmd StartReviewCommand) (*domain.MigrationCase, error) {
	const action = "start_review"
	c, replayed, err := s.loadForMutation(id, cmd.CommandMeta, action, "")
	if err != nil || replayed {
		return c, err
	}
	expected, from, now := c.Revision, c.Status, s.now().UTC()
	if err := c.StartReview(now); err != nil {
		return nil, err
	}
	return s.save(c, expected, cmd.RequestID, action, from, now, "系统角色：审查流转", "提交双专业联合会审")
}

func (s *Service) SubmitOpinion(id string, cmd SubmitOpinionCommand) (*domain.MigrationCase, error) {
	const action = "submit_opinion"
	digest, err := opinionDigest(cmd)
	if err != nil {
		return nil, err
	}
	c, replayed, err := s.loadForMutation(id, cmd.CommandMeta, action, digest)
	if err != nil || replayed {
		return c, err
	}
	expected, from, now := c.Revision, c.Status, s.now().UTC()
	op := domain.ReviewOpinion{ID: s.newID("opinion"), CaseID: id, ReviewRound: c.ReviewRound, Discipline: cmd.Discipline, ReviewerName: strings.TrimSpace(cmd.ReviewerName), Conclusion: cmd.Conclusion, Issues: cmd.Issues, Basis: strings.TrimSpace(cmd.Basis), SubmittedAt: now}
	if err := c.AddOpinion(op, now); err != nil {
		return nil, err
	}
	return s.saveWithDigest(c, expected, cmd.RequestID, action, digest, from, now, strings.TrimSpace(cmd.ReviewerName), fmt.Sprintf("提交%s专业意见，共 %d 个问题", cmd.Discipline, len(cmd.Issues)))
}

func (s *Service) SubmitRevision(id string, cmd SubmitRevisionCommand) (*domain.MigrationCase, error) {
	const action = "submit_revision"
	digest, err := revisionDigest(cmd)
	if err != nil {
		return nil, err
	}
	c, replayed, err := s.loadForMutation(id, cmd.CommandMeta, action, digest)
	if err != nil || replayed {
		return c, err
	}
	if err := validateIdentity(cmd.SubmittedBy, "submitted_by"); err != nil {
		return nil, err
	}
	expected, from, now := c.Revision, c.Status, s.now().UTC()
	changes := domain.DiffPlans(c.Plan, cmd.Plan)
	evaluation := s.policy.EvaluateRevision(c.ID, c.Revision+1, cmd.Plan, changes, c.Findings)
	sub := domain.RevisionSubmission{ID: s.newID("revision"), CaseID: id, FromRevision: expected, IssueResponses: cmd.IssueResponses, AttachmentMetadata: cmd.AttachmentMetadata, RuleVersion: policy.Version, FieldRuleImpacts: evaluation.FieldRuleImpacts, RuleImpacts: evaluation.RuleImpacts, SubmittedBy: strings.TrimSpace(cmd.SubmittedBy), SubmittedAt: now}
	if err := c.SubmitRevision(sub, cmd.Plan, evaluation.Findings, now); err != nil {
		return nil, err
	}
	response, err := marshalCase(c)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveWithAudit(c, expected, cmd.RequestID, action, digest, from, response, now, store.AuditContext{Actor: strings.TrimSpace(cmd.SubmittedBy), Result: fmt.Sprintf("提交整改版本，变更 %d 个字段并响应 %d 个问题", len(changes), len(cmd.IssueResponses))}); err != nil {
		return s.replayAfterConflict(cmd.RequestID, action, digest, err)
	}
	return c, nil
}

func (s *Service) Approve(id string, cmd ApproveCommand) (*domain.MigrationCase, error) {
	const action = "approve_case"
	digest, err := approveDigest(cmd)
	if err != nil {
		return nil, err
	}
	c, replayed, err := s.loadForMutation(id, cmd.CommandMeta, action, digest)
	if err != nil || replayed {
		return c, err
	}
	if err := validateIdentity(cmd.ApproverName, "approver_name"); err != nil {
		return nil, err
	}
	verification := c.ApprovalReadiness(policy.Version)
	if !verification.Approvable {
		return nil, fieldError("approval", strings.Join(verification.Reasons, "；"))
	}
	expected, from, now := c.Revision, c.Status, s.now().UTC()
	record := domain.ApprovalRecord{ID: s.newID("approval"), CaseID: id, ApproverName: strings.TrimSpace(cmd.ApproverName), Decision: "approved", DecisionNote: strings.TrimSpace(cmd.DecisionNote), ApprovedAt: now}
	if err := c.Approve(record, now); err != nil {
		return nil, err
	}
	return s.saveWithDigest(c, expected, cmd.RequestID, action, digest, from, now, strings.TrimSpace(cmd.ApproverName), "批准前核验全部通过并形成不可变摘要")
}

func (s *Service) save(c *domain.MigrationCase, expected int64, requestID, action string, from domain.Status, now time.Time, actor, result string) (*domain.MigrationCase, error) {
	response, err := marshalCase(c)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveWithAudit(c, expected, requestID, action, "", from, response, now, store.AuditContext{Actor: actor, Result: result}); err != nil {
		return s.replayAfterConflict(requestID, action, "", err)
	}
	return c, nil
}

func (s *Service) saveWithDigest(c *domain.MigrationCase, expected int64, requestID, action, digest string, from domain.Status, now time.Time, actor, result string) (*domain.MigrationCase, error) {
	response, err := marshalCase(c)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveWithAudit(c, expected, requestID, action, digest, from, response, now, store.AuditContext{Actor: actor, Result: result}); err != nil {
		return s.replayAfterConflict(requestID, action, digest, err)
	}
	return c, nil
}

func opinionDigest(cmd SubmitOpinionCommand) (string, error) {
	return digestPayload(struct {
		Discipline   domain.Discipline    `json:"discipline"`
		ReviewerName string               `json:"reviewer_name"`
		Conclusion   string               `json:"conclusion"`
		Issues       []domain.ReviewIssue `json:"issues"`
		Basis        string               `json:"basis"`
	}{cmd.Discipline, cmd.ReviewerName, cmd.Conclusion, cmd.Issues, cmd.Basis})
}

func revisionDigest(cmd SubmitRevisionCommand) (string, error) {
	return digestPayload(struct {
		Plan               domain.Plan                 `json:"plan"`
		IssueResponses     []domain.IssueResponse      `json:"issue_responses"`
		AttachmentMetadata []domain.AttachmentMetadata `json:"attachment_metadata"`
		SubmittedBy        string                      `json:"submitted_by"`
	}{cmd.Plan, cmd.IssueResponses, cmd.AttachmentMetadata, cmd.SubmittedBy})
}

func approveDigest(cmd ApproveCommand) (string, error) {
	return digestPayload(struct {
		ApproverName string `json:"approver_name"`
		DecisionNote string `json:"decision_note"`
	}{cmd.ApproverName, cmd.DecisionNote})
}

func countSeverity(findings []domain.PolicyFinding, severity domain.FindingSeverity) int {
	count := 0
	for _, finding := range findings {
		if finding.Severity == severity {
			count++
		}
	}
	return count
}
