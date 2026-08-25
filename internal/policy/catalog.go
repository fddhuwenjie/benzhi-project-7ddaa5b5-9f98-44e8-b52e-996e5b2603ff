package policy

import "benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"

const Version = "2026.1"

type Rule struct {
	Code        string                             `json:"code"`
	Name        string                             `json:"name"`
	Description string                             `json:"description"`
	Fields      []string                           `json:"fields"`
	Evaluate    func(domain.Plan) *findingTemplate `json:"-"`
}

type findingTemplate struct {
	Severity  domain.FindingSeverity
	Message   string
	FieldPath string
	Evidence  string
}

func Catalog() []Rule {
	return []Rule{
		{Code: "SEASON-001", Name: "适宜施工季节", Description: "施工窗口应位于休眠期或适宜移栽月份", Fields: []string{"construction_window"}, Evaluate: evaluateSeason},
		{Code: "ROOTBALL-001", Name: "土球规格", Description: "土球直径不得小于胸径的八倍", Fields: []string{"trunk_diameter_cm", "root_ball_diameter_cm"}, Evaluate: evaluateRootBall},
		{Code: "CROWN-001", Name: "根冠保护措施", Description: "保护方案应明确根系、树冠和伤口保护", Fields: []string{"protection_measures"}, Evaluate: evaluateProtection},
		{Code: "TRANSPORT-001", Name: "运输时长", Description: "运输时长超过六小时会显著增加失水风险", Fields: []string{"transport_duration_hours", "protection_measures"}, Evaluate: evaluateTransport},
		{Code: "SITE-001", Name: "迁入地条件", Description: "迁入地应在施工前完成准备", Fields: []string{"destination_location", "destination_ready"}, Evaluate: evaluateDestination},
	}
}

func RuleDescriptions() []Rule {
	rules := Catalog()
	for i := range rules {
		rules[i].Evaluate = nil
	}
	return rules
}

func Applicability() map[string]string {
	result := make(map[string]string)
	for _, rule := range Catalog() {
		result[rule.Code] = rule.Description
	}
	return result
}
