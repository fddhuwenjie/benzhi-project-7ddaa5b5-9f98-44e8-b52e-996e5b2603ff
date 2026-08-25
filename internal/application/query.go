package application

import (
	"sort"
	"strings"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

var queueStatuses = []domain.Status{
	domain.StatusDraft, domain.StatusValidated, domain.StatusUnderReview,
	domain.StatusRevisionRequired, domain.StatusResubmitted, domain.StatusApproved,
}

func (s *Service) QueryCases(query ListCasesQuery) (CaseListResult, error) {
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 20
	}
	if query.Page < 1 {
		return CaseListResult{}, fieldError("page", "须大于 0")
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		return CaseListResult{}, fieldError("page_size", "须在 1 至 100 之间")
	}
	if query.Sort == "" {
		query.Sort = "updated_at"
	}
	if query.Sort == "updated_at_desc" {
		query.Sort = "updated_at"
	}
	if query.Sort != "updated_at" && query.Sort != "construction_window" && query.Sort != "tree_code" {
		return CaseListResult{}, fieldError("sort", "须为 updated_at、construction_window 或 tree_code")
	}
	validStatuses := make(map[domain.Status]bool, len(queueStatuses))
	for _, status := range queueStatuses {
		validStatuses[status] = true
	}
	selectedStatuses := make(map[domain.Status]bool, len(query.Statuses))
	for _, status := range query.Statuses {
		if !validStatuses[status] {
			return CaseListResult{}, fieldError("status", "未知状态 "+string(status))
		}
		selectedStatuses[status] = true
	}
	cases, err := s.repo.List()
	if err != nil {
		return CaseListResult{}, err
	}
	textMatched := make([]*domain.MigrationCase, 0, len(cases))
	for _, c := range cases {
		if !containsNormalized(c.Plan.TreeCode, query.TreeCode) || !containsNormalized(c.Plan.Species, query.Species) {
			continue
		}
		if query.Location != "" && !containsNormalized(c.Plan.SourceLocation, query.Location) && !containsNormalized(c.Plan.DestinationLocation, query.Location) {
			continue
		}
		textMatched = append(textMatched, c)
	}
	result := CaseListResult{StatusCounts: make(map[domain.Status]int), Page: query.Page, PageSize: query.PageSize}
	for _, status := range queueStatuses {
		result.StatusCounts[status] = 0
	}
	for _, c := range textMatched {
		result.StatusCounts[c.Status]++
		if len(selectedStatuses) > 0 && !selectedStatuses[c.Status] {
			continue
		}
		result.Cases = append(result.Cases, c)
		if domain.HasBlockers(c.Findings) {
			result.Attention.BlockingFindings++
		}
		for _, seat := range c.ReviewSeats {
			if !seat.Completed {
				result.Attention.PendingSeats++
			}
		}
		for _, item := range c.ModificationItems {
			if !item.Resolved {
				result.Attention.PendingResponses++
			}
		}
	}
	sort.SliceStable(result.Cases, func(i, j int) bool {
		left, right := result.Cases[i], result.Cases[j]
		switch query.Sort {
		case "construction_window":
			if left.Plan.ConstructionWindow != right.Plan.ConstructionWindow {
				return left.Plan.ConstructionWindow < right.Plan.ConstructionWindow
			}
		case "tree_code":
			leftCode, rightCode := domain.NormalizeTreeCode(left.Plan.TreeCode), domain.NormalizeTreeCode(right.Plan.TreeCode)
			if leftCode != rightCode {
				return leftCode < rightCode
			}
		default:
			if !left.UpdatedAt.Equal(right.UpdatedAt) {
				return left.UpdatedAt.After(right.UpdatedAt)
			}
		}
		return left.ID < right.ID
	})
	result.Total = len(result.Cases)
	result.TotalPages = (result.Total + query.PageSize - 1) / query.PageSize
	start := (query.Page - 1) * query.PageSize
	if start >= result.Total {
		result.Cases = []*domain.MigrationCase{}
		return result, nil
	}
	end := start + query.PageSize
	if end > result.Total {
		end = result.Total
	}
	result.Cases = result.Cases[start:end]
	return result, nil
}

func containsNormalized(value, query string) bool {
	query = normalizeSearchText(query)
	return query == "" || strings.Contains(normalizeSearchText(value), query)
}

func normalizeSearchText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}
