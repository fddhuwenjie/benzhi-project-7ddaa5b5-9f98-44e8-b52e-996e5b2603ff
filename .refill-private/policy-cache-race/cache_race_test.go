package policy_cache_race_test

import (
	"sync"
	"testing"
	"time"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/domain"
	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/policy"
)

func TestConcurrentEvaluatorCacheAccess(t *testing.T) {
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	evaluator := policy.NewEvaluator(func() time.Time { return now })
	plan := domain.Plan{
		TreeCode:              "GS-RACE",
		Species:               "香樟",
		AgeYears:              180,
		HealthGrade:           "良好",
		SourceLocation:        "甲",
		DestinationLocation:   "乙",
		MigrationReason:       "避让",
		ConstructionWindow:    "2026-11-01/2026-11-03",
		ProtectionMeasures:    "根系、树冠、伤口保护",
		TrunkDiameterCM:       80,
		RootBallDiameterCM:    800,
		TransportDurationHour: 4,
		DestinationReady:      true,
	}
	const workers = 16
	start := make(chan struct{})
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(workers)
	done.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func(worker int) {
			defer done.Done()
			ready.Done()
			<-start
			for round := 0; round < 32; round++ {
				findings := evaluator.Evaluate("case-"+itoa(worker), int64(round+1), plan)
				if len(findings) != 0 {
					t.Errorf("正常方案不应产生规则结果：%#v", findings)
					return
				}
			}
		}(worker)
	}
	ready.Wait()
	close(start)
	done.Wait()
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	buf := make([]byte, 0, 3)
	for value > 0 {
		buf = append([]byte{byte('0' + value%10)}, buf...)
		value /= 10
	}
	return string(buf)
}
