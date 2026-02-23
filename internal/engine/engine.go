package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"rfguard/internal/alerts"
	"rfguard/internal/config"
	"rfguard/internal/metrics"
	"rfguard/internal/model"
	"rfguard/internal/storage"
)

type Engine struct {
	logger   *slog.Logger
	metrics  *metrics.Store
	alerts   *alerts.Store
	store    storage.Store
	cfg      atomic.Value
	access   atomic.Value
	readers  map[string]*ReaderState
	mu       sync.Mutex
	started  time.Time
	cooldown *Cooldown
	deDupe   *DedupeCache
}

type ReaderState struct {
	id               string
	windows          map[int]*WindowState
	uidFailureStreak map[string]int
}

func NewEngine(cfg *config.Config, logger *slog.Logger, metricsStore *metrics.Store, alertsStore *alerts.Store, store storage.Store) *Engine {
	e := &Engine{
		logger:   logger,
		metrics:  metricsStore,
		alerts:   alertsStore,
		store:    store,
		readers:  make(map[string]*ReaderState),
		started:  time.Now().UTC(),
		cooldown: NewCooldown(),
		deDupe:   NewDedupeCache(),
	}
	e.cfg.Store(cfg)
	e.access.Store(buildAccessControl(cfg))
	return e
}

func (e *Engine) UpdateConfig(cfg *config.Config) {
	e.cfg.Store(cfg)
	e.access.Store(buildAccessControl(cfg))
}

func (e *Engine) config() *config.Config {
	if v := e.cfg.Load(); v != nil {
		return v.(*config.Config)
	}
	return config.DefaultConfig()
}

func (e *Engine) Start(ctx context.Context, in <-chan model.NormalizedEvent) {
	go func() {
		for {
			select {
			case ev := <-in:
				e.ProcessEvent(ev)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (e *Engine) ProcessEvent(ev model.NormalizedEvent) []model.Alert {
	cfg := e.config()
	now := time.Now().UTC()
	clamped := clampTimestamp(ev.Timestamp, now, cfg.Detection.MaxClockSkew, cfg.Detection.MaxFutureSkew)
	ev.Timestamp = clamped

	if e.isDuplicate(ev, cfg.Detection.DedupeWindow) {
		return nil
	}

	alertsOut := make([]model.Alert, 0)
	if alert, ok := e.evaluateAccess(cfg, ev); ok {
		alertsOut = append(alertsOut, alert)
		e.alerts.Add(alert)
		if e.logger != nil {
			e.logger.Warn("access control alert",
				"reader_id", alert.ReaderID,
				"severity", alert.Severity,
				"rules", alert.Rules,
			)
		}
		if e.store != nil {
			_ = e.store.SaveAlert(context.Background(), alert)
		}
	}

	reader := e.getReader(ev.ReaderID, cfg)
	if alert, ok := e.evaluateAuthStreak(cfg, reader, ev); ok {
		alertsOut = append(alertsOut, alert)
		e.alerts.Add(alert)
		if e.logger != nil {
			e.logger.Warn("auth streak alert",
				"reader_id", alert.ReaderID,
				"uid", alert.Context["uid"],
				"rules", alert.Rules,
			)
		}
		if e.store != nil {
			_ = e.store.SaveAlert(context.Background(), alert)
		}
	}

	metricsList := make([]model.WindowMetrics, 0, len(reader.windows))
	for _, window := range reader.sortedWindows() {
		cutoff := ev.Timestamp.Add(-window.duration)
		window.Evict(cutoff)
		window.Add(EventEntry{Timestamp: ev.Timestamp, UID: ev.UID, Result: ev.Result})
		wm := window.Metrics()
		metricsList = append(metricsList, wm)
		alert, ok := e.evaluate(cfg, ev.ReaderID, wm)
		if ok {
			alertsOut = append(alertsOut, alert)
			e.alerts.Add(alert)
			if e.logger != nil {
				e.logger.Warn("alert triggered",
					"reader_id", alert.ReaderID,
					"window_sec", alert.WindowSec,
					"severity", alert.Severity,
					"rules", alert.Rules,
					"score", alert.Score,
				)
			}
			if e.store != nil {
				_ = e.store.SaveAlert(context.Background(), alert)
			}
		}
	}
	if len(metricsList) > 0 {
		e.metrics.Update(ev.ReaderID, metricsList)
		if e.store != nil {
			_ = e.store.SaveMetrics(context.Background(), ev.ReaderID, metricsList)
		}
	}
	return alertsOut
}

func (e *Engine) Reset() {
	e.mu.Lock()
	e.readers = make(map[string]*ReaderState)
	e.mu.Unlock()
	e.cooldown = NewCooldown()
	e.deDupe = NewDedupeCache()
}

func (e *Engine) getReader(readerID string, cfg *config.Config) *ReaderState {
	if readerID == "" {
		readerID = "unknown"
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if r, ok := e.readers[readerID]; ok {
		for _, win := range cfg.Detection.Windows {
			sec := int(win.Seconds())
			if _, exists := r.windows[sec]; !exists {
				r.windows[sec] = NewWindowState(win)
			}
		}
		return r
	}
	r := &ReaderState{
		id:               readerID,
		windows:          make(map[int]*WindowState),
		uidFailureStreak: make(map[string]int),
	}
	for _, win := range cfg.Detection.Windows {
		sec := int(win.Seconds())
		r.windows[sec] = NewWindowState(win)
	}
	e.readers[readerID] = r
	return r
}

func (r *ReaderState) sortedWindows() []*WindowState {
	keys := make([]int, 0, len(r.windows))
	for k := range r.windows {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]*WindowState, 0, len(keys))
	for _, k := range keys {
		out = append(out, r.windows[k])
	}
	return out
}

func (e *Engine) evaluate(cfg *config.Config, readerID string, wm model.WindowMetrics) (model.Alert, bool) {
	if wm.Attempts == 0 {
		return model.Alert{}, false
	}
	var rules []string
	if wm.APS > cfg.Detection.APSThreshold {
		rules = append(rules, "excessive_attempt_rate")
	}
	if wm.FR > cfg.Detection.FailureRatioThreshold && wm.Attempts >= cfg.Detection.MinAttempts {
		rules = append(rules, "failure_spike")
	}
	if wm.UDS > cfg.Detection.UIDDiversityThreshold && wm.APS > cfg.Detection.APSElevatedThreshold {
		rules = append(rules, "uid_spraying")
	}
	if wm.TV < cfg.Detection.TimingVarianceThreshold && wm.APS > cfg.Detection.APSElevatedThreshold {
		rules = append(rules, "machine_timing")
	}

	score := cfg.Detection.Weights.APS*wm.APS +
		cfg.Detection.Weights.FR*wm.FR +
		cfg.Detection.Weights.UDS*wm.UDS +
		cfg.Detection.Weights.TV*(1.0/(wm.TV+cfg.Detection.Epsilon))
	if score > cfg.Detection.AttackScoreThreshold &&
		wm.Attempts >= cfg.Detection.MinAttempts &&
		wm.APS > cfg.Detection.APSElevatedThreshold {
		rules = append(rules, "attack_score")
	}
	if len(rules) == 0 {
		return model.Alert{}, false
	}
	if !e.cooldown.Allow(readerID, wm.WindowSec, cfg.Detection.AlertCooldown) {
		return model.Alert{}, false
	}
	severity := "medium"
	if score > cfg.Detection.AttackScoreThreshold*2 || wm.APS > cfg.Detection.APSThreshold*2 {
		severity = "critical"
	} else if score > cfg.Detection.AttackScoreThreshold || len(rules) >= 2 {
		severity = "high"
	}
	alert := model.Alert{
		Timestamp: time.Now().UTC(),
		ReaderID:  readerID,
		Severity:  severity,
		AlertType: "possible_bruteforce",
		WindowSec: wm.WindowSec,
		Metrics:   wm,
		Score:     score,
		Rules:     rules,
		Context:   map[string]string{"engine": "rfguard"},
	}
	return alert, true
}

func (e *Engine) evaluateAccess(cfg *config.Config, ev model.NormalizedEvent) (model.Alert, bool) {
	ac := e.accessSet()
	if ac == nil || !ac.Enabled {
		return model.Alert{}, false
	}
	if ev.UID == "" {
		return model.Alert{}, false
	}
	uid := normalizeUID(ev.UID)
	if uid == "" {
		return model.Alert{}, false
	}
	if ac.IsBlacklisted(ev.ReaderID, uid) {
		if !e.cooldown.Allow(ev.ReaderID, 0, cfg.Detection.AlertCooldown) {
			return model.Alert{}, false
		}
		return model.Alert{
			Timestamp: time.Now().UTC(),
			ReaderID:  ev.ReaderID,
			Severity:  "critical",
			AlertType: "blacklisted_uid",
			WindowSec: 0,
			Metrics:   model.WindowMetrics{},
			Score:     0,
			Rules:     []string{"blacklisted_uid"},
			Context: map[string]string{
				"uid":        uid,
				"uid_raw":    ev.UID,
				"source":     ev.Source,
				"result":     string(ev.Result),
				"error_code": ev.ErrorCode,
			},
		}, true
	}
	if ac.WhitelistOnly && !ac.IsWhitelisted(ev.ReaderID, uid) {
		if !e.cooldown.Allow(ev.ReaderID, 0, cfg.Detection.AlertCooldown) {
			return model.Alert{}, false
		}
		return model.Alert{
			Timestamp: time.Now().UTC(),
			ReaderID:  ev.ReaderID,
			Severity:  "high",
			AlertType: "whitelist_violation",
			WindowSec: 0,
			Metrics:   model.WindowMetrics{},
			Score:     0,
			Rules:     []string{"whitelist_violation"},
			Context: map[string]string{
				"uid":        uid,
				"uid_raw":    ev.UID,
				"source":     ev.Source,
				"result":     string(ev.Result),
				"error_code": ev.ErrorCode,
			},
		}, true
	}
	return model.Alert{}, false
}

func (e *Engine) evaluateAuthStreak(cfg *config.Config, reader *ReaderState, ev model.NormalizedEvent) (model.Alert, bool) {
	if reader == nil || ev.UID == "" {
		return model.Alert{}, false
	}
	uid := normalizeUID(ev.UID)
	if uid == "" {
		return model.Alert{}, false
	}
	if ev.Result != model.ResultFailure || ev.ErrorCode == "" {
		reader.uidFailureStreak[uid] = 0
		return model.Alert{}, false
	}
	reader.uidFailureStreak[uid]++
	if reader.uidFailureStreak[uid] < 2 {
		return model.Alert{}, false
	}
	if !e.cooldown.AllowKey("authfail|"+reader.id+"|"+uid, cfg.Detection.AlertCooldown) {
		return model.Alert{}, false
	}
	return model.Alert{
		Timestamp: time.Now().UTC(),
		ReaderID:  reader.id,
		Severity:  "medium",
		AlertType: "repeated_auth_failure",
		WindowSec: 0,
		Metrics:   model.WindowMetrics{},
		Score:     0,
		Rules:     []string{"repeated_auth_failure"},
		Context: map[string]string{
			"uid":        uid,
			"uid_raw":    ev.UID,
			"source":     ev.Source,
			"result":     string(ev.Result),
			"error_code": ev.ErrorCode,
		},
	}, true
}

func (e *Engine) accessSet() *AccessControlSet {
	if v := e.access.Load(); v != nil {
		if ac, ok := v.(*AccessControlSet); ok {
			return ac
		}
	}
	return nil
}

func (e *Engine) isDuplicate(ev model.NormalizedEvent, dedupeWindow time.Duration) bool {
	if dedupeWindow <= 0 {
		return false
	}
	key := hashEvent(ev)
	return e.deDupe.Seen(key, time.Now().UTC(), dedupeWindow)
}

func hashEvent(ev model.NormalizedEvent) string {
	parts := []string{
		ev.ReaderID,
		ev.UID,
		string(ev.Result),
		ev.ErrorCode,
		ev.Timestamp.UTC().Format(time.RFC3339Nano),
	}
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])
}

func clampTimestamp(ts, now time.Time, maxPast, maxFuture time.Duration) time.Time {
	if ts.IsZero() {
		return now
	}
	if maxPast > 0 {
		if now.Sub(ts) > maxPast {
			return now
		}
	}
	if maxFuture > 0 {
		if ts.Sub(now) > maxFuture {
			return now
		}
	}
	return ts
}
