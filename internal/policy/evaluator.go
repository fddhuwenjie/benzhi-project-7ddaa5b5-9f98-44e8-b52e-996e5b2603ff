package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

type Evaluator struct {
	now func() time.Time
}

func NewEvaluator(now func() time.Time) *Evaluator {
	if now == nil {
		now = time.Now
	}
	return &Evaluator{now: now}
}

func (e *Evaluator) Evaluate(caseID string, revision int64, plan domain.Plan) []domain.PolicyFinding {
	return e.evaluateRules(caseID, revision, plan, Catalog())
}

func (e *Evaluator) EvaluateAffected(caseID string, revision int64, plan domain.Plan, changes []domain.FieldChange, previous []domain.PolicyFinding) []domain.PolicyFinding {
	return e.EvaluateRevision(caseID, revision, plan, changes, previous).Findings
}

// EvaluateRevisionWithVersion determines the evaluation strategy by comparing
// the aggregate rule version with the current policy version. When they differ,
// the whole rule catalog is re-evaluated against the current rules and a
// complete impact chain is produced that explicitly marks the version switch.
// Otherwise the revision is evaluated against only the affected fields and the
// previous conclusions for unaffected rules are carried forward.
func (e *Evaluator) EvaluateRevisionWithVersion(caseID string, revision int64, plan domain.Plan, changes []domain.FieldChange, previous []domain.PolicyFinding, aggregateRuleVersion string) domain.RevisionEvaluation {
	if aggregateRuleVersion != Version {
		return e.evaluateVersionSwitch(caseID, revision, plan, changes, previous, aggregateRuleVersion)
	}
	return e.EvaluateRevision(caseID, revision, plan, changes, previous)
}

// evaluateVersionSwitch re-evaluates every rule in the current catalog because
// the aggregate was last validated under a different rule version. Previous
// findings may have been produced by an older rule set, so carrying them
// forward would risk treating a now-missing finding as a pass. Each rule is
// recalculated and the impact chain records the version switch so the
// resulting audit trail is explicit about the change in rule versions.
func (e *Evaluator) evaluateVersionSwitch(caseID string, revision int64, plan domain.Plan, changes []domain.FieldChange, previous []domain.PolicyFinding, aggregateRuleVersion string) domain.RevisionEvaluation {
	changed := make(map[string]bool)
	for _, change := range changes {
		changed[change.Field] = true
	}
	previousByCode := make(map[string]domain.PolicyFinding, len(previous))
	for _, finding := range previous {
		previousByCode[finding.RuleCode] = finding
	}

	fieldImpacts := make([]domain.FieldRuleImpact, 0, len(changes))
	for _, change := range changes {
		codes := make([]string, 0)
		for _, rule := range Catalog() {
			for _, field := range rule.Fields {
				if field == change.Field {
					codes = append(codes, rule.Code)
					break
				}
			}
		}
		sort.Strings(codes)
		fieldImpacts = append(fieldImpacts, domain.FieldRuleImpact{Field: change.Field, RuleCodes: codes, NoRecalculation: len(codes) == 0})
	}
	sort.Slice(fieldImpacts, func(i, j int) bool { return fieldImpacts[i].Field < fieldImpacts[j].Field })

	findings := make([]domain.PolicyFinding, 0)
	impacts := make([]domain.RuleImpact, 0, len(Catalog()))
	for _, rule := range Catalog() {
		prior, hadPrior := previousByCode[rule.Code]
		beforeOutcome, beforeEvidence := outcome(hadPrior, prior)
		current := e.evaluateRules(caseID, revision, plan, []Rule{rule})
		impact := domain.RuleImpact{RuleCode: rule.Code, RuleVersion: Version, BeforeOutcome: beforeOutcome, BeforeEvidence: beforeEvidence}
		impact.Disposition = domain.RuleRecalculated
		fieldNames := make([]string, 0, len(rule.Fields))
		for _, field := range rule.Fields {
			if changed[field] {
				fieldNames = append(fieldNames, field)
			}
		}
		if len(fieldNames) > 0 {
			impact.Reason = "规则版本由 " + aggregateRuleVersion + " 切换至 " + Version + "，且字段 " + strings.Join(fieldNames, "、") + " 变化，按当前规则全集复算"
		} else {
			impact.Reason = "规则版本由 " + aggregateRuleVersion + " 切换至 " + Version + "，按当前规则全集复算"
		}
		if len(current) == 1 {
			impact.AfterOutcome, impact.AfterEvidence = outcome(true, current[0])
			findings = append(findings, current[0])
		} else {
			impact.AfterOutcome = domain.RuleOutcomePass
		}
		impacts = append(impacts, impact)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].RuleCode < findings[j].RuleCode })
	sort.Slice(impacts, func(i, j int) bool { return impacts[i].RuleCode < impacts[j].RuleCode })
	return domain.RevisionEvaluation{Findings: findings, FieldRuleImpacts: fieldImpacts, RuleImpacts: impacts}
}

func (e *Evaluator) EvaluateRevision(caseID string, revision int64, plan domain.Plan, changes []domain.FieldChange, previous []domain.PolicyFinding) domain.RevisionEvaluation {
	changed := make(map[string]bool)
	for _, change := range changes {
		changed[change.Field] = true
	}
	affected := make(map[string]bool)
	reasons := make(map[string][]string)
	fieldImpacts := make([]domain.FieldRuleImpact, 0, len(changes))
	for _, rule := range Catalog() {
		for _, field := range rule.Fields {
			if changed[field] {
				affected[rule.Code] = true
				reasons[rule.Code] = append(reasons[rule.Code], field)
			}
		}
	}
	for _, change := range changes {
		codes := make([]string, 0)
		for _, rule := range Catalog() {
			for _, field := range rule.Fields {
				if field == change.Field {
					codes = append(codes, rule.Code)
					break
				}
			}
		}
		sort.Strings(codes)
		fieldImpacts = append(fieldImpacts, domain.FieldRuleImpact{Field: change.Field, RuleCodes: codes, NoRecalculation: len(codes) == 0})
	}
	sort.Slice(fieldImpacts, func(i, j int) bool { return fieldImpacts[i].Field < fieldImpacts[j].Field })

	previousByCode := make(map[string]domain.PolicyFinding, len(previous))
	for _, finding := range previous {
		previousByCode[finding.RuleCode] = finding
	}
	findings := make([]domain.PolicyFinding, 0)
	impacts := make([]domain.RuleImpact, 0, len(Catalog()))
	for _, rule := range Catalog() {
		prior, hadPrior := previousByCode[rule.Code]
		beforeOutcome, beforeEvidence := outcome(hadPrior, prior)
		impact := domain.RuleImpact{RuleCode: rule.Code, RuleVersion: Version, BeforeOutcome: beforeOutcome, BeforeEvidence: beforeEvidence}
		if affected[rule.Code] {
			current := e.evaluateRules(caseID, revision, plan, []Rule{rule})
			impact.Disposition = domain.RuleRecalculated
			impact.Reason = "字段 " + strings.Join(reasons[rule.Code], "、") + " 变化触发复算"
			if len(current) == 1 {
				impact.AfterOutcome, impact.AfterEvidence = outcome(true, current[0])
				findings = append(findings, current[0])
			} else {
				impact.AfterOutcome = domain.RuleOutcomePass
			}
		} else {
			impact.Disposition = domain.RuleCarried
			impact.Reason = "本次字段变化未影响该规则，沿用上一 revision 结论"
			impact.AfterOutcome, impact.AfterEvidence = beforeOutcome, beforeEvidence
			if hadPrior {
				prior.CaseRevision = revision
				findings = append(findings, prior)
			}
		}
		impacts = append(impacts, impact)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].RuleCode < findings[j].RuleCode })
	sort.Slice(impacts, func(i, j int) bool { return impacts[i].RuleCode < impacts[j].RuleCode })
	return domain.RevisionEvaluation{Findings: findings, FieldRuleImpacts: fieldImpacts, RuleImpacts: impacts}
}

func outcome(hasFinding bool, finding domain.PolicyFinding) (domain.RuleOutcome, string) {
	if !hasFinding {
		return domain.RuleOutcomePass, ""
	}
	if finding.Severity == domain.SeverityBlocker {
		return domain.RuleOutcomeBlocker, finding.EvidenceValue
	}
	return domain.RuleOutcomeWarning, finding.EvidenceValue
}

func (e *Evaluator) evaluateRules(caseID string, revision int64, plan domain.Plan, rules []Rule) []domain.PolicyFinding {
	var findings []domain.PolicyFinding
	for _, rule := range rules {
		template := rule.Evaluate(plan)
		if template == nil {
			continue
		}
		findings = append(findings, domain.PolicyFinding{
			ID: findingID(caseID, revision, rule.Code, template.Evidence), CaseID: caseID,
			CaseRevision: revision, RuleCode: rule.Code, Severity: template.Severity,
			Message: template.Message, FieldPath: template.FieldPath,
			EvidenceValue: template.Evidence, EvaluatedAt: e.now().UTC(),
		})
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].RuleCode < findings[j].RuleCode })
	return findings
}

func findingID(caseID string, revision int64, code, evidence string) string {
	sum := sha256.Sum256([]byte(caseID + "|" + code + "|" + evidence + "|" + strconv.FormatInt(revision, 10)))
	return "finding-" + hex.EncodeToString(sum[:8])
}
