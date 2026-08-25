package domain

import (
	"regexp"
	"sort"
	"strings"
)

var allowedAttachmentTypes = map[string]bool{
	"application/pdf": true,
	"image/jpeg":      true,
	"image/png":       true,
}

var attachmentDigestPattern = regexp.MustCompile(`^(sha256:)?[a-fA-F0-9]{64}$`)

func ValidateRevisionSubmission(sub RevisionSubmission, items []ModificationItem) error {
	errs := NewValidationErrors()
	if strings.TrimSpace(sub.SubmittedBy) == "" {
		errs.Add("submitted_by", "不能为空")
	}
	required := make(map[string]bool, len(items))
	for _, item := range items {
		required[item.IssueCode] = true
	}
	seen := make(map[string]bool)
	for i, response := range sub.IssueResponses {
		field := "issue_responses[" + itoa(i) + "]"
		if !required[response.IssueCode] {
			errs.Add(field+".issue_code", "包含未知问题代码 "+response.IssueCode)
			continue
		}
		if seen[response.IssueCode] {
			errs.Add(field+".issue_code", "问题代码 "+response.IssueCode+" 重复响应")
		}
		seen[response.IssueCode] = true
		if strings.TrimSpace(response.Response) == "" {
			errs.Add(field+".response", "问题 "+response.IssueCode+" 的响应不能为空")
		}
	}
	for code := range required {
		if !seen[code] {
			errs.Add("issue_responses."+code, "每个问题必须恰有一条响应")
		}
	}
	attachmentNames := make(map[string]bool)
	for i, attachment := range sub.AttachmentMetadata {
		field := "attachment_metadata[" + itoa(i) + "]"
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			errs.Add(field+".name", "附件名称不能为空")
		}
		nameKey := strings.ToLower(name)
		if attachmentNames[nameKey] {
			errs.Add(field+".name", "附件名称不可重复")
		}
		attachmentNames[nameKey] = true
		if !allowedAttachmentTypes[attachment.ContentType] {
			errs.Add(field+".content_type", "附件 "+name+" 的 Content-Type 不受支持")
		}
		if attachment.SizeBytes <= 0 || attachment.SizeBytes > 50<<20 {
			errs.Add(field+".size_bytes", "附件 "+name+" 大小须在 1 byte 至 50 MiB 之间")
		}
		if !attachmentDigestPattern.MatchString(strings.TrimSpace(attachment.Digest)) {
			errs.Add(field+".digest", "附件 "+name+" 摘要须为 SHA-256 十六进制值")
		}
	}
	if errs.Empty() {
		return nil
	}
	return errs
}

func ValidateResolvedResponses(sub RevisionSubmission, items []ModificationItem, changes []FieldChange) error {
	errs := NewValidationErrors()
	changed := make(map[string]bool, len(changes))
	for _, change := range changes {
		changed[change.Field] = true
	}
	itemByCode := make(map[string]ModificationItem, len(items))
	for _, item := range items {
		itemByCode[item.IssueCode] = item
	}
	for i, response := range sub.IssueResponses {
		item, ok := itemByCode[response.IssueCode]
		if !ok || !response.Resolved || changed[item.FieldPath] {
			continue
		}
		if strings.TrimSpace(response.NoChangeReason) == "" {
			errs.Add("issue_responses["+itoa(i)+"].no_change_reason", "标记已解决但关联字段未变化时须说明无需变更的具体原因")
		}
	}
	if errs.Empty() {
		return nil
	}
	return errs
}

func ValidateImpactChain(sub RevisionSubmission) error {
	if sub.RuleVersion == "" && len(sub.FieldRuleImpacts) == 0 && len(sub.RuleImpacts) == 0 {
		return nil
	}
	errs := NewValidationErrors()
	if strings.TrimSpace(sub.RuleVersion) == "" {
		errs.Add("rule_version", "规则影响链缺少规则版本")
	}
	impacts := make(map[string]RuleImpact, len(sub.RuleImpacts))
	previousCode := ""
	for i, impact := range sub.RuleImpacts {
		field := "rule_impacts[" + itoa(i) + "]"
		if impact.RuleCode == "" || impact.RuleVersion != sub.RuleVersion {
			errs.Add(field, "规则代码或规则版本无效")
		}
		if previousCode != "" && impact.RuleCode <= previousCode {
			errs.Add("rule_impacts", "规则影响须按规则代码稳定排序且不可重复")
		}
		if impact.Disposition != RuleRecalculated && impact.Disposition != RuleCarried {
			errs.Add(field+".disposition", "影响类型无效")
		}
		previousCode = impact.RuleCode
		impacts[impact.RuleCode] = impact
	}
	previousField := ""
	for i, fieldImpact := range sub.FieldRuleImpacts {
		field := "field_rule_impacts[" + itoa(i) + "]"
		if previousField != "" && fieldImpact.Field <= previousField {
			errs.Add("field_rule_impacts", "字段影响须稳定排序且不可重复")
		}
		previousField = fieldImpact.Field
		if fieldImpact.NoRecalculation != (len(fieldImpact.RuleCodes) == 0) {
			errs.Add(field, "无需复算标记与规则列表不一致")
		}
		if !sort.StringsAreSorted(fieldImpact.RuleCodes) {
			errs.Add(field+".rule_codes", "规则代码须稳定排序")
		}
		for _, code := range fieldImpact.RuleCodes {
			impact, ok := impacts[code]
			if !ok || impact.Disposition != RuleRecalculated {
				errs.Add(field+".rule_codes", "声明受影响规则 "+code+" 缺少重算结果")
			}
		}
	}
	if errs.Empty() {
		return nil
	}
	return errs
}

func ValidateAggregate(c *MigrationCase) error {
	if c == nil || strings.TrimSpace(c.ID) == "" {
		return errorsFor("id", "聚合标识无效")
	}
	if c.Revision < 1 {
		return errorsFor("revision", "须大于 0")
	}
	validStatus := map[Status]bool{StatusDraft: true, StatusValidated: true, StatusUnderReview: true, StatusRevisionRequired: true, StatusResubmitted: true, StatusApproved: true}
	if !validStatus[c.Status] {
		return errorsFor("status", "未知状态")
	}
	if err := ValidateDraft(c.Plan); err != nil {
		return err
	}
	if c.CreatedAt.IsZero() || c.UpdatedAt.Before(c.CreatedAt) {
		return errorsFor("updated_at", "聚合时间无效")
	}
	previousRevision := int64(0)
	for _, entry := range c.Timeline {
		if entry.Revision < previousRevision || entry.Revision > c.Revision {
			return errorsFor("timeline", "时间线 revision 顺序无效")
		}
		previousRevision = entry.Revision
	}
	for i, batch := range c.ValidationBatches {
		if batch.ID == "" || batch.CaseID != c.ID || batch.CaseRevision < 1 || batch.CaseRevision > c.Revision || batch.RuleVersion == "" || batch.EvaluatedAt.IsZero() {
			return errorsFor("validation_batches["+itoa(i)+"]", "校验批次引用无效")
		}
		if i > 0 && batch.EvaluatedAt.Before(c.ValidationBatches[i-1].EvaluatedAt) {
			return errorsFor("validation_batches", "校验批次时间顺序无效")
		}
		for j := 1; j < len(batch.Findings); j++ {
			if batch.Findings[j-1].RuleCode >= batch.Findings[j].RuleCode {
				return errorsFor("validation_batches", "批次规则结果未稳定排序")
			}
		}
		for j := 1; j < len(batch.Conclusions); j++ {
			if batch.Conclusions[j-1].RuleCode >= batch.Conclusions[j].RuleCode {
				return errorsFor("validation_batches", "批次结论未稳定排序")
			}
		}
	}
	for i, submission := range c.RevisionSubmissions {
		if submission.CaseID != c.ID || submission.FromRevision < 1 || submission.ToRevision <= submission.FromRevision || submission.ToRevision > c.Revision {
			return errorsFor("revision_submissions["+itoa(i)+"]", "整改版本引用无效")
		}
		if err := ValidateImpactChain(submission); err != nil {
			return err
		}
		if submission.RuleVersion == "" {
			return errorsFor("revision_submissions["+itoa(i)+"].rule_version", "整改版本缺少规则影响链")
		}
	}
	if c.Status == StatusApproved {
		if c.Approval == nil || c.Approval.Decision != "approved" || c.Approval.ApprovedRevision != c.Revision || len(c.Approval.ContentDigest) != 64 {
			return errorsFor("approval", "批准档案不完整")
		}
	} else if c.Approval != nil {
		return errorsFor("approval", "未批准状态不应包含批准档案")
	}
	return nil
}
