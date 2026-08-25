package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

func (c *MigrationCase) ApprovalReadiness(expectedRuleVersion string) ApprovalVerification {
	checks := make([]ApprovalCheck, 0, 5)
	add := func(code, label string, passed bool, success, failure string) {
		message := success
		if !passed {
			message = failure
		}
		checks = append(checks, ApprovalCheck{Code: code, Label: label, Passed: passed, Message: message})
	}
	statusReady := c.Status == StatusResubmitted || c.Status == StatusApproved
	add("status", "整改状态", statusReady, "个案已提交整改版本", "个案须处于 resubmitted 状态")

	responsesComplete := len(c.RevisionSubmissions) > 0
	if responsesComplete {
		latest := c.RevisionSubmissions[len(c.RevisionSubmissions)-1]
		responses := make(map[string]IssueResponse, len(latest.IssueResponses))
		for _, response := range latest.IssueResponses {
			responses[response.IssueCode] = response
		}
		for _, item := range c.ModificationItems {
			response, ok := responses[item.IssueCode]
			if !ok || !response.Resolved || strings.TrimSpace(response.Response) == "" || !item.Resolved {
				responsesComplete = false
				break
			}
		}
	}
	add("responses", "问题响应", responsesComplete, "全部会审问题均已响应并关闭", "存在未响应或未关闭的会审问题")

	noBlockers := !HasBlockers(c.Findings)
	add("findings", "当前规则结果", noBlockers, "当前规则结果无阻断项", "当前规则结果仍有阻断项")

	versionReady := c.RuleVersion != "" && (expectedRuleVersion == "" || c.RuleVersion == expectedRuleVersion)
	add("rule_version", "规则版本", versionReady, "规则版本有效："+c.RuleVersion, "规则版本缺失或不是当前适用版本")

	expectedRevision := c.Revision
	if c.Status == StatusApproved {
		expectedRevision--
	}
	revisionComplete := len(c.RevisionSubmissions) > 0
	if revisionComplete {
		latest := c.RevisionSubmissions[len(c.RevisionSubmissions)-1]
		revisionComplete = latest.ToRevision == expectedRevision && latest.FromRevision < latest.ToRevision && latest.RuleVersion == c.RuleVersion && ValidateImpactChain(latest) == nil
	}
	add("revision", "整改版本完整性", revisionComplete, "整改版本、差异、响应和规则影响链完整", "整改版本与当前 revision 或规则影响链不完整")

	verification := ApprovalVerification{Approvable: true, Checks: checks, ArchiveStatus: "not_applicable"}
	for _, check := range checks {
		if !check.Passed {
			verification.Approvable = false
			verification.Reasons = append(verification.Reasons, check.Message)
		}
	}
	if c.Status == StatusApproved && c.Approval != nil {
		digest, err := c.ApprovalDigest()
		if err != nil {
			verification.ArchiveStatus = "mismatch"
			verification.Reasons = append(verification.Reasons, "批准档案摘要无法重新计算")
		} else {
			verification.ComputedDigest = digest
			if digest == c.Approval.ContentDigest {
				verification.ArchiveStatus = "verified"
			} else {
				verification.ArchiveStatus = "mismatch"
				verification.Reasons = append(verification.Reasons, "批准档案摘要不一致")
			}
		}
	}
	return verification
}

func (c *MigrationCase) ApprovalDigest() (string, error) {
	if c.Approval == nil {
		return c.pendingApprovalDigest()
	}
	record := *c.Approval
	record.ContentDigest = ""
	return c.digestPayload(c.Revision, record)
}

func (c *MigrationCase) approvalDigestFor(record ApprovalRecord) (string, error) {
	return c.digestPayload(c.Revision+1, record)
}

func (c *MigrationCase) pendingApprovalDigest() (string, error) {
	return c.digestPayload(c.Revision, ApprovalRecord{})
}

func (c *MigrationCase) digestPayload(finalRevision int64, approval ApprovalRecord) (string, error) {
	findings := append([]PolicyFinding(nil), c.Findings...)
	opinions := append([]ReviewOpinion(nil), c.Opinions...)
	items := append([]ModificationItem(nil), c.ModificationItems...)
	batches := append([]ValidationBatch(nil), c.ValidationBatches...)
	submissions := c.RevisionSubmissions
	sort.Slice(findings, func(i, j int) bool { return findings[i].RuleCode < findings[j].RuleCode })
	sort.Slice(opinions, func(i, j int) bool {
		if opinions[i].ReviewRound != opinions[j].ReviewRound {
			return opinions[i].ReviewRound < opinions[j].ReviewRound
		}
		return opinions[i].Discipline < opinions[j].Discipline
	})
	for i := range opinions {
		sort.Slice(opinions[i].Issues, func(left, right int) bool {
			return opinions[i].Issues[left].Code < opinions[i].Issues[right].Code
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].IssueCode < items[j].IssueCode })
	sort.Slice(batches, func(i, j int) bool {
		if batches[i].EvaluatedAt.Equal(batches[j].EvaluatedAt) {
			return batches[i].ID < batches[j].ID
		}
		return batches[i].EvaluatedAt.Before(batches[j].EvaluatedAt)
	})
	sort.Slice(submissions, func(i, j int) bool { return submissions[i].ToRevision < submissions[j].ToRevision })
	for i := range submissions {
		sort.Slice(submissions[i].FieldChanges, func(left, right int) bool {
			return submissions[i].FieldChanges[left].Field < submissions[i].FieldChanges[right].Field
		})
		sort.Slice(submissions[i].IssueResponses, func(left, right int) bool {
			return submissions[i].IssueResponses[left].IssueCode < submissions[i].IssueResponses[right].IssueCode
		})
		sort.Slice(submissions[i].AttachmentMetadata, func(left, right int) bool {
			return submissions[i].AttachmentMetadata[left].Name < submissions[i].AttachmentMetadata[right].Name
		})
	}
	payload := struct {
		ID                string               `json:"id"`
		FinalRevision     int64                `json:"final_revision"`
		Plan              Plan                 `json:"plan"`
		RuleVersion       string               `json:"rule_version"`
		Findings          []PolicyFinding      `json:"findings"`
		ValidationBatches []ValidationBatch    `json:"validation_batches"`
		Opinions          []ReviewOpinion      `json:"opinions"`
		ModificationItems []ModificationItem   `json:"modification_items"`
		Submissions       []RevisionSubmission `json:"revision_submissions"`
		Approval          ApprovalRecord       `json:"approval"`
	}{c.ID, finalRevision, c.Plan, c.RuleVersion, findings, batches, opinions, items, submissions, approval}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
