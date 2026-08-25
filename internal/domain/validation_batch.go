package domain

import "sort"

func BuildValidationConclusions(current, previous []PolicyFinding, applicability map[string]string) []ValidationConclusion {
	currentByCode := make(map[string]PolicyFinding, len(current))
	previousByCode := make(map[string]PolicyFinding, len(previous))
	for _, finding := range current {
		currentByCode[finding.RuleCode] = finding
	}
	for _, finding := range previous {
		previousByCode[finding.RuleCode] = finding
	}
	codes := make(map[string]bool, len(currentByCode)+len(previousByCode))
	for code := range currentByCode {
		codes[code] = true
	}
	for code := range previousByCode {
		codes[code] = true
	}
	result := make([]ValidationConclusion, 0, len(codes))
	for code := range codes {
		currentFinding, hasCurrent := currentByCode[code]
		previousFinding, hadPrevious := previousByCode[code]
		conclusion := ValidationConclusion{RuleCode: code, Applicability: applicability[code]}
		if hadPrevious {
			conclusion.FieldPath = previousFinding.FieldPath
			conclusion.PreviousSeverity = previousFinding.Severity
			conclusion.PreviousEvidence = previousFinding.EvidenceValue
		}
		if hasCurrent {
			conclusion.FieldPath = currentFinding.FieldPath
			conclusion.CurrentSeverity = currentFinding.Severity
			conclusion.CurrentMessage = currentFinding.Message
			conclusion.CurrentEvidence = currentFinding.EvidenceValue
		}
		switch {
		case hasCurrent && !hadPrevious:
			conclusion.Change = ConclusionNew
		case hasCurrent:
			conclusion.Change = ConclusionContinued
		default:
			conclusion.Change = ConclusionEliminated
		}
		result = append(result, conclusion)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].RuleCode < result[j].RuleCode })
	return result
}
