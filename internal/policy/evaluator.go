package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

type Evaluator struct {
	now   func() time.Time
	mu    sync.RWMutex
	cache map[string][]domain.PolicyFinding
}

func NewEvaluator(now func() time.Time) *Evaluator {
	if now == nil {
		now = time.Now
	}
	return &Evaluator{now: now, cache: make(map[string][]domain.PolicyFinding)}
}

func (e *Evaluator) Evaluate(caseID string, revision int64, plan domain.Plan) []domain.PolicyFinding {
	keyBytes, _ := json.Marshal(struct {
		CaseID  string      `json:"case_id"`
		Version int64       `json:"revision"`
		Plan    domain.Plan `json:"plan"`
	}{caseID, revision, plan})
	key := string(keyBytes)
	e.mu.RLock()
	cached, ok := e.cache[key]
	e.mu.RUnlock()
	if ok {
		return append([]domain.PolicyFinding(nil), cached...)
	}
	findings := e.evaluateRules(caseID, revision, plan, Catalog())
	stored := append([]domain.PolicyFinding(nil), findings...)
	e.mu.Lock()
	var duplicate bool
	if existing, ok := e.cache[key]; ok {
		cached = existing
		duplicate = true
	} else {
		e.cache[key] = stored
		cached = stored
	}
	e.mu.Unlock()
	if duplicate {
		return append([]domain.PolicyFinding(nil), cached...)
	}
	return findings
}

func (e *Evaluator) EvaluateAffected(caseID string, revision int64, plan domain.Plan, changes []domain.FieldChange, previous []domain.PolicyFinding) []domain.PolicyFinding {
	return e.EvaluateRevision(caseID, revision, plan, changes, previous).Findings
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
