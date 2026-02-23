package engine

import (
	"testing"
	"time"

	"rfguard/internal/alerts"
	"rfguard/internal/config"
	"rfguard/internal/metrics"
	"rfguard/internal/model"
)

func testConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Detection.Windows = []time.Duration{1 * time.Second}
	cfg.Detection.APSThreshold = 10
	cfg.Detection.FailureRatioThreshold = 0.8
	cfg.Detection.UIDDiversityThreshold = 0.8
	cfg.Detection.TimingVarianceThreshold = 0.0001
	cfg.Detection.AttackScoreThreshold = 20
	cfg.Detection.MinAttempts = 5
	cfg.Detection.APSElevatedThreshold = 5
	cfg.Detection.AlertCooldown = 0
	cfg.Detection.DedupeWindow = 0
	cfg.Detection.MaxClockSkew = 0
	cfg.Detection.MaxFutureSkew = 0
	cfg.Detection.Weights = config.WeightsConfig{APS: 1, FR: 10, UDS: 10, TV: 1}
	return cfg
}

func newEngineForTest(cfg *config.Config) *Engine {
	return NewEngine(cfg, nil, metrics.NewStore(100), alerts.NewStore(100), nil)
}

func TestNormalTrafficNoAlert(t *testing.T) {
	cfg := testConfig()
	eng := newEngineForTest(cfg)
	base := time.Now().Add(-2 * time.Second)
	for i := 0; i < 5; i++ {
		ev := model.NormalizedEvent{
			Timestamp: base.Add(time.Duration(i) * time.Second),
			ReaderID:  "reader01",
			UID:       "AABBCC",
			Result:    model.ResultSuccess,
		}
		alerts := eng.ProcessEvent(ev)
		if len(alerts) > 0 {
			t.Fatalf("unexpected alert")
		}
	}
}

func TestBruteforceAlert(t *testing.T) {
	cfg := testConfig()
	eng := newEngineForTest(cfg)
	base := time.Now().Add(-500 * time.Millisecond)
	var got bool
	for i := 0; i < 20; i++ {
		ev := model.NormalizedEvent{
			Timestamp: base.Add(time.Duration(i) * 20 * time.Millisecond),
			ReaderID:  "reader01",
			UID:       "AABBCC",
			Result:    model.ResultFailure,
			ErrorCode: "AUTH_FAIL",
		}
		alerts := eng.ProcessEvent(ev)
		for _, a := range alerts {
			if containsRule(a.Rules, "excessive_attempt_rate") {
				got = true
			}
		}
	}
	if !got {
		t.Fatalf("expected bruteforce alert")
	}
}

func TestUIDSprayAlert(t *testing.T) {
	cfg := testConfig()
	eng := newEngineForTest(cfg)
	base := time.Now().Add(-500 * time.Millisecond)
	var got bool
	for i := 0; i < 15; i++ {
		ev := model.NormalizedEvent{
			Timestamp: base.Add(time.Duration(i) * 30 * time.Millisecond),
			ReaderID:  "reader01",
			UID:       "UID" + itoa(i),
			Result:    model.ResultFailure,
			ErrorCode: "AUTH_FAIL",
		}
		alerts := eng.ProcessEvent(ev)
		for _, a := range alerts {
			if containsRule(a.Rules, "uid_spraying") {
				got = true
			}
		}
	}
	if !got {
		t.Fatalf("expected uid spray alert")
	}
}

func TestMachineTimingAlert(t *testing.T) {
	cfg := testConfig()
	eng := newEngineForTest(cfg)
	base := time.Now().Add(-500 * time.Millisecond)
	var got bool
	for i := 0; i < 20; i++ {
		ev := model.NormalizedEvent{
			Timestamp: base.Add(time.Duration(i) * 50 * time.Millisecond),
			ReaderID:  "reader01",
			UID:       "AABBCC",
			Result:    model.ResultFailure,
			ErrorCode: "TIMEOUT",
		}
		alerts := eng.ProcessEvent(ev)
		for _, a := range alerts {
			if containsRule(a.Rules, "machine_timing") {
				got = true
			}
		}
	}
	if !got {
		t.Fatalf("expected machine timing alert")
	}
}

func TestAccessControlBlacklist(t *testing.T) {
	cfg := testConfig()
	cfg.AccessControl.Enabled = true
	cfg.AccessControl.Blacklist = []string{"DEADBEEF"}
	eng := newEngineForTest(cfg)
	ev := model.NormalizedEvent{
		Timestamp: time.Now(),
		ReaderID:  "reader01",
		UID:       "DEAD-BEEF",
		Result:    model.ResultFailure,
	}
	alerts := eng.ProcessEvent(ev)
	if !hasRule(alerts, "blacklisted_uid") {
		t.Fatalf("expected blacklisted_uid alert")
	}
}

func TestAccessControlWhitelistViolation(t *testing.T) {
	cfg := testConfig()
	cfg.AccessControl.Enabled = true
	cfg.AccessControl.WhitelistOnly = true
	cfg.AccessControl.Whitelist = []string{"AABBCC"}
	eng := newEngineForTest(cfg)
	ev := model.NormalizedEvent{
		Timestamp: time.Now(),
		ReaderID:  "reader01",
		UID:       "BEEF01",
		Result:    model.ResultFailure,
	}
	alerts := eng.ProcessEvent(ev)
	if !hasRule(alerts, "whitelist_violation") {
		t.Fatalf("expected whitelist_violation alert")
	}
}

func TestRepeatedAuthFailureAlert(t *testing.T) {
	cfg := testConfig()
	cfg.Detection.AlertCooldown = 0
	eng := newEngineForTest(cfg)
	now := time.Now()
	ev1 := model.NormalizedEvent{
		Timestamp: now,
		ReaderID:  "reader01",
		UID:       "AA11BB22",
		Result:    model.ResultFailure,
		ErrorCode: "AUTH_FAIL",
	}
	ev2 := model.NormalizedEvent{
		Timestamp: now.Add(10 * time.Millisecond),
		ReaderID:  "reader01",
		UID:       "AA11BB22",
		Result:    model.ResultFailure,
		ErrorCode: "AUTH_FAIL",
	}
	if alerts := eng.ProcessEvent(ev1); hasRule(alerts, "repeated_auth_failure") {
		t.Fatalf("unexpected repeated_auth_failure on first error")
	}
	if alerts := eng.ProcessEvent(ev2); !hasRule(alerts, "repeated_auth_failure") {
		t.Fatalf("expected repeated_auth_failure alert")
	}
}

func containsRule(rules []string, target string) bool {
	for _, r := range rules {
		if r == target {
			return true
		}
	}
	return false
}

func hasRule(alerts []model.Alert, target string) bool {
	for _, a := range alerts {
		if containsRule(a.Rules, target) {
			return true
		}
	}
	return false
}
