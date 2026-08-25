package domain

import (
	"errors"
	"sort"
	"strings"
	"time"
)

var allowedHealthGrades = map[string]bool{"良好": true, "一般": true, "衰弱": true}

var planFieldPaths = map[string]bool{
	"tree_code": true, "species": true, "age_years": true, "health_grade": true,
	"source_location": true, "destination_location": true, "migration_reason": true,
	"construction_window": true, "protection_measures": true, "trunk_diameter_cm": true,
	"root_ball_diameter_cm": true, "transport_duration_hours": true, "destination_ready": true,
}

func NormalizeTreeCode(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }

func PlanFieldPaths() []string {
	result := make([]string, 0, len(planFieldPaths))
	for field := range planFieldPaths {
		result = append(result, field)
	}
	sort.Strings(result)
	return result
}

func ValidateDraft(plan Plan) error {
	errs := NewValidationErrors()
	required := map[string]string{
		"tree_code": plan.TreeCode, "species": plan.Species,
		"health_grade": plan.HealthGrade, "source_location": plan.SourceLocation,
		"destination_location": plan.DestinationLocation, "migration_reason": plan.MigrationReason,
		"construction_window": plan.ConstructionWindow, "protection_measures": plan.ProtectionMeasures,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			errs.Add(field, "不能为空")
		}
	}
	if plan.AgeYears < 100 || plan.AgeYears > 3000 {
		errs.Add("age_years", "古树年龄须在 100 至 3000 年之间")
	}
	if !allowedHealthGrades[plan.HealthGrade] {
		errs.Add("health_grade", "须为良好、一般或衰弱")
	}
	if plan.TrunkDiameterCM <= 0 {
		errs.Add("trunk_diameter_cm", "胸径须大于 0")
	}
	if plan.RootBallDiameterCM <= 0 {
		errs.Add("root_ball_diameter_cm", "土球直径须大于 0")
	}
	if plan.TransportDurationHour <= 0 {
		errs.Add("transport_duration_hours", "运输时长须大于 0")
	}
	if start, end, err := parseWindow(plan.ConstructionWindow); err != nil {
		errs.Add("construction_window", "格式须为 YYYY-MM-DD/YYYY-MM-DD")
	} else if end.Before(start) {
		errs.Add("construction_window", "结束日期不得早于开始日期")
	} else if end.Sub(start) > 45*24*time.Hour {
		errs.Add("construction_window", "施工窗口不得超过 45 天")
	}
	if errs.Empty() {
		return nil
	}
	return errs
}

func parseWindow(value string) (time.Time, time.Time, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, errors.New("施工窗口格式无效")
	}
	start, err := time.Parse("2006-01-02", parts[0])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := time.Parse("2006-01-02", parts[1])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, end, nil
}

func ValidateOpinion(op ReviewOpinion) error {
	errs := NewValidationErrors()
	if op.Discipline != DisciplineArboriculture && op.Discipline != DisciplineConstruction {
		errs.Add("discipline", "不支持的专业席位")
	}
	if strings.TrimSpace(op.ReviewerName) == "" {
		errs.Add("reviewer_name", "不能为空")
	}
	if op.Conclusion != "pass" && op.Conclusion != "revise" {
		errs.Add("conclusion", "须为 pass 或 revise")
	}
	if strings.TrimSpace(op.Basis) == "" {
		errs.Add("basis", "不能为空")
	}
	seen := make(map[string]bool)
	for i, issue := range op.Issues {
		field := "issues[" + itoa(i) + "]"
		if strings.TrimSpace(issue.Code) == "" || strings.TrimSpace(issue.Title) == "" || strings.TrimSpace(issue.FieldPath) == "" || strings.TrimSpace(issue.Suggestion) == "" {
			errs.Add(field, "问题代码、标题、关联字段和修改建议均不能为空")
		}
		code := strings.ToUpper(strings.TrimSpace(issue.Code))
		if seen[code] {
			errs.Add(field+".code", "问题代码不可重复")
		}
		seen[code] = true
		if !planFieldPaths[issue.FieldPath] {
			errs.Add(field+".field_path", "不是可关联的方案字段")
		}
	}
	if op.Conclusion == "revise" && len(op.Issues) == 0 {
		errs.Add("issues", "结论为 revise 时至少需要一条问题")
	}
	if op.Conclusion == "pass" && len(op.Issues) > 0 {
		errs.Add("issues", "结论为 pass 时不得包含问题")
	}
	if errs.Empty() {
		return nil
	}
	return errs
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 4)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}
