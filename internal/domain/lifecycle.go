package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func NewCase(id string, plan Plan, now time.Time) (*MigrationCase, error) {
	if err := ValidateDraft(plan); err != nil {
		return nil, err
	}
	c := &MigrationCase{ID: id, Plan: plan, Status: StatusDraft, Revision: 1, CreatedAt: now, UpdatedAt: now}
	c.record("创建草稿", now)
	return c, nil
}

func (c *MigrationCase) ensureMutable() error {
	if c.Status == StatusApproved {
		return ErrAlreadyApproved
	}
	return nil
}

func (c *MigrationCase) ApplyValidation(findings []PolicyFinding, ruleVersion string, now time.Time) error {
	return c.applyValidation(findings, ruleVersion, nil, now)
}

func (c *MigrationCase) ApplyValidationBatch(batch ValidationBatch, now time.Time) error {
	if batch.ID == "" || batch.CaseID != c.ID || batch.CaseRevision != c.Revision || batch.RuleVersion == "" || batch.EvaluatedAt.IsZero() {
		return errorsFor("validation_batch", "校验批次与当前个案 revision 不匹配")
	}
	for i := 1; i < len(batch.Findings); i++ {
		if batch.Findings[i-1].RuleCode >= batch.Findings[i].RuleCode {
			return errorsFor("validation_batch.findings", "规则结果须按规则代码稳定排序")
		}
	}
	for i := 1; i < len(batch.Conclusions); i++ {
		if batch.Conclusions[i-1].RuleCode >= batch.Conclusions[i].RuleCode {
			return errorsFor("validation_batch.conclusions", "结论变化须按规则代码稳定排序")
		}
	}
	return c.applyValidation(batch.Findings, batch.RuleVersion, &batch, now)
}

func (c *MigrationCase) applyValidation(findings []PolicyFinding, ruleVersion string, batch *ValidationBatch, now time.Time) error {
	if err := c.ensureMutable(); err != nil {
		return err
	}
	if c.Status != StatusDraft && c.Status != StatusValidated {
		return fmt.Errorf("%w：%s 不能执行技术校验", ErrInvalidTransition, c.Status)
	}
	c.Findings = findings
	c.RuleVersion = ruleVersion
	if batch != nil {
		c.ValidationBatches = append(c.ValidationBatches, *batch)
	}
	if HasBlockers(findings) {
		c.Revision++
		c.UpdatedAt = now
		c.record("技术规则校验存在阻断项", now)
		return ErrBlockingFindings
	}
	c.advance(StatusValidated, "技术规则校验通过", now)
	return nil
}

func (c *MigrationCase) StartReview(now time.Time) error {
	if err := c.ensureMutable(); err != nil {
		return err
	}
	if c.Status != StatusValidated {
		return fmt.Errorf("%w：仅 validated 可提交会审", ErrInvalidTransition)
	}
	c.ReviewRound++
	c.ReviewSeats = []ReviewSeat{{Discipline: DisciplineArboriculture}, {Discipline: DisciplineConstruction}}
	c.advance(StatusUnderReview, "提交联合会审并冻结版本", now)
	return nil
}

func (c *MigrationCase) AddOpinion(op ReviewOpinion, now time.Time) error {
	if err := c.ensureMutable(); err != nil {
		return err
	}
	if c.Status != StatusUnderReview {
		return fmt.Errorf("%w：当前不在会审中", ErrInvalidTransition)
	}
	if err := ValidateOpinion(op); err != nil {
		return err
	}
	for i := range op.Issues {
		op.Issues[i].Code = strings.ToUpper(strings.TrimSpace(op.Issues[i].Code))
		op.Issues[i].Title = strings.TrimSpace(op.Issues[i].Title)
		op.Issues[i].FieldPath = strings.TrimSpace(op.Issues[i].FieldPath)
		op.Issues[i].Suggestion = strings.TrimSpace(op.Issues[i].Suggestion)
	}
	for _, existing := range c.Opinions {
		if existing.ReviewRound == c.ReviewRound && existing.Discipline == op.Discipline {
			return ErrDuplicateDiscipline
		}
		if existing.ReviewRound == c.ReviewRound {
			for _, priorIssue := range existing.Issues {
				for _, newIssue := range op.Issues {
					if strings.EqualFold(strings.TrimSpace(priorIssue.Code), strings.TrimSpace(newIssue.Code)) {
						return errorsFor("issues", "问题代码 "+newIssue.Code+" 已被其他席位使用")
					}
				}
			}
		}
	}
	found := false
	for i := range c.ReviewSeats {
		if c.ReviewSeats[i].Discipline == op.Discipline {
			c.ReviewSeats[i].Completed, found = true, true
		}
	}
	if !found {
		return fmt.Errorf("未知审查席位：%s", op.Discipline)
	}
	c.Opinions = append(c.Opinions, op)
	c.Revision++
	c.UpdatedAt = now
	c.record("提交"+string(op.Discipline)+"专业意见", now)
	if c.reviewsComplete() {
		c.ModificationItems = buildModificationItems(c.Opinions, c.ReviewRound)
		c.Status = StatusRevisionRequired
		c.record("双专业意见汇总，要求整改", now)
	}
	return nil
}

func (c *MigrationCase) SubmitRevision(sub RevisionSubmission, newPlan Plan, findings []PolicyFinding, now time.Time) error {
	if err := c.ensureMutable(); err != nil {
		return err
	}
	if c.Status != StatusRevisionRequired {
		return fmt.Errorf("%w：当前无需整改", ErrInvalidTransition)
	}
	if err := ValidateDraft(newPlan); err != nil {
		return err
	}
	changes := DiffPlans(c.Plan, newPlan)
	sub.FieldChanges = changes
	sub.ToRevision = c.Revision + 1
	if sub.RuleVersion == "" {
		sub.RuleVersion = c.RuleVersion
	}
	if err := ValidateRevisionSubmission(sub, c.ModificationItems); err != nil {
		return err
	}
	if err := ValidateResolvedResponses(sub, c.ModificationItems, changes); err != nil {
		return err
	}
	if err := ValidateImpactChain(sub); err != nil {
		return err
	}
	responses := make(map[string]IssueResponse)
	for _, response := range sub.IssueResponses {
		if strings.TrimSpace(response.Response) == "" {
			return fmt.Errorf("问题 %s 的响应不能为空", response.IssueCode)
		}
		responses[response.IssueCode] = response
	}
	for i := range c.ModificationItems {
		response, ok := responses[c.ModificationItems[i].IssueCode]
		if !ok || !response.Resolved {
			return fmt.Errorf("%w：%s", ErrUnresolvedIssues, c.ModificationItems[i].IssueCode)
		}
		c.ModificationItems[i].Resolved = true
	}
	if HasBlockers(findings) {
		return ErrBlockingFindings
	}
	c.Plan = newPlan
	c.Findings = findings
	c.RuleVersion = sub.RuleVersion
	c.RevisionSubmissions = append(c.RevisionSubmissions, sub)
	c.advance(StatusResubmitted, "提交整改版本并复算规则", now)
	return nil
}

func (c *MigrationCase) Approve(record ApprovalRecord, now time.Time) error {
	if err := c.ensureMutable(); err != nil {
		return err
	}
	if strings.TrimSpace(record.ApproverName) == "" {
		return errorsFor("approver_name", "不能为空")
	}
	if record.Decision != "approved" {
		return errorsFor("decision", "须为 approved")
	}
	verification := c.ApprovalReadiness("")
	if !verification.Approvable {
		return errorsFor("approval", strings.Join(verification.Reasons, "；"))
	}
	record.ApprovedRevision = c.Revision + 1
	record.ContentDigest = ""
	digest, err := c.approvalDigestFor(record)
	if err != nil {
		return err
	}
	record.ContentDigest = digest
	c.Approval = &record
	c.advance(StatusApproved, "复核通过并批准归档", now)
	return nil
}

func HasBlockers(findings []PolicyFinding) bool {
	for _, finding := range findings {
		if finding.Severity == SeverityBlocker {
			return true
		}
	}
	return false
}

func (c *MigrationCase) reviewsComplete() bool {
	return len(c.ReviewSeats) == 2 && c.ReviewSeats[0].Completed && c.ReviewSeats[1].Completed
}

func buildModificationItems(ops []ReviewOpinion, round int) []ModificationItem {
	var items []ModificationItem
	for _, op := range ops {
		if op.ReviewRound != round {
			continue
		}
		for _, issue := range op.Issues {
			items = append(items, ModificationItem{IssueCode: issue.Code, Discipline: op.Discipline, Title: issue.Title, FieldPath: issue.FieldPath, Suggestion: issue.Suggestion, ReviewerName: op.ReviewerName, Basis: op.Basis})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].IssueCode < items[j].IssueCode })
	return items
}

func (c *MigrationCase) advance(status Status, action string, now time.Time) {
	c.Status = status
	c.Revision++
	c.UpdatedAt = now
	c.record(action, now)
}

func (c *MigrationCase) record(action string, now time.Time) {
	c.Timeline = append(c.Timeline, TimelineEntry{Status: c.Status, Revision: c.Revision, Action: action, OccurredAt: now})
}

func errorsFor(field, message string) error {
	e := NewValidationErrors()
	e.Add(field, message)
	return e
}
