package policy

import (
	"fmt"
	"strings"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
)

func evaluateSeason(plan domain.Plan) *findingTemplate {
	start, err := time.Parse("2006-01-02", strings.Split(plan.ConstructionWindow, "/")[0])
	if err != nil {
		return &findingTemplate{domain.SeverityBlocker, "施工窗口格式无法评估", "construction_window", plan.ConstructionWindow}
	}
	month := start.Month()
	if month >= time.May && month <= time.September {
		return &findingTemplate{domain.SeverityWarning, "施工窗口处于高温生长期，需强化蒸腾抑制与补水监测", "construction_window", plan.ConstructionWindow}
	}
	return nil
}

func evaluateRootBall(plan domain.Plan) *findingTemplate {
	minimum := plan.TrunkDiameterCM * 8
	if plan.RootBallDiameterCM < minimum {
		return &findingTemplate{domain.SeverityBlocker, fmt.Sprintf("土球直径不足，应至少为 %d cm", minimum), "root_ball_diameter_cm", fmt.Sprintf("%d", plan.RootBallDiameterCM)}
	}
	if plan.RootBallDiameterCM < plan.TrunkDiameterCM*10 {
		return &findingTemplate{domain.SeverityWarning, "土球规格达到最低值但余量较小", "root_ball_diameter_cm", fmt.Sprintf("%d", plan.RootBallDiameterCM)}
	}
	return nil
}

func evaluateProtection(plan domain.Plan) *findingTemplate {
	text := strings.ToLower(plan.ProtectionMeasures)
	groups := [][]string{{"根", "root"}, {"冠", "crown"}, {"伤口", "切口", "wound"}}
	for _, alternatives := range groups {
		matched := false
		for _, keyword := range alternatives {
			if strings.Contains(text, keyword) {
				matched = true
				break
			}
		}
		if !matched {
			return &findingTemplate{domain.SeverityBlocker, "保护措施必须覆盖根系、树冠和伤口保护", "protection_measures", plan.ProtectionMeasures}
		}
	}
	return nil
}

func evaluateTransport(plan domain.Plan) *findingTemplate {
	if plan.TransportDurationHour > 12 {
		return &findingTemplate{domain.SeverityBlocker, "运输时长超过 12 小时，须调整路线或设置中转养护", "transport_duration_hours", fmt.Sprintf("%d", plan.TransportDurationHour)}
	}
	if plan.TransportDurationHour > 6 {
		return &findingTemplate{domain.SeverityWarning, "运输时长超过 6 小时，应增加保湿和途中检查", "transport_duration_hours", fmt.Sprintf("%d", plan.TransportDurationHour)}
	}
	return nil
}

func evaluateDestination(plan domain.Plan) *findingTemplate {
	if !plan.DestinationReady {
		return &findingTemplate{domain.SeverityBlocker, "迁入地尚未完成土壤、排水和支撑条件准备", "destination_ready", "false"}
	}
	return nil
}
