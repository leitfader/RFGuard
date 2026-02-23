package model

import "time"

type Result string

const (
	ResultSuccess Result = "success"
	ResultFailure Result = "failure"
)

type NormalizedEvent struct {
	Timestamp time.Time `json:"timestamp"`
	ReaderID  string    `json:"reader_id"`
	UID       string    `json:"uid,omitempty"`
	Result    Result    `json:"result"`
	ErrorCode string    `json:"error_code,omitempty"`
	Source    string    `json:"source,omitempty"`
	Raw       string    `json:"raw,omitempty"`
}

type WindowMetrics struct {
	WindowSec int     `json:"window_sec"`
	Attempts  int     `json:"attempts"`
	Failures  int     `json:"failures"`
	APS       float64 `json:"aps"`
	FR        float64 `json:"fr"`
	UDS       float64 `json:"uds"`
	TV        float64 `json:"tv"`
}

type Alert struct {
	Timestamp time.Time         `json:"timestamp"`
	ReaderID  string            `json:"reader_id"`
	Severity  string            `json:"severity"`
	AlertType string            `json:"alert_type"`
	WindowSec int               `json:"window_sec"`
	Metrics   WindowMetrics     `json:"metrics"`
	Score     float64           `json:"score"`
	Rules     []string          `json:"rules"`
	Context   map[string]string `json:"context,omitempty"`
}
