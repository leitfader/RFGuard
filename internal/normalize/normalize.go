package normalize

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"rfguard/internal/config"
	"rfguard/internal/model"
)

type EventFields struct {
	Timestamp string
	ReaderID  string
	UID       string
	Result    string
	ErrorCode string
	Extras    map[string]string
	Raw       string
}

func Normalize(fields EventFields, cfg *config.Config) (model.NormalizedEvent, error) {
	reader := strings.TrimSpace(fields.ReaderID)
	if reader == "" {
		reader = cfg.Ingest.Parser.DefaultReaderID
	}

	loc := time.UTC
	if cfg.Ingest.Parser.Timezone != "" {
		if l, err := time.LoadLocation(cfg.Ingest.Parser.Timezone); err == nil {
			loc = l
		}
	}

	ts := time.Now().UTC()
	if fields.Timestamp != "" {
		parsed, err := ParseTimestamp(fields.Timestamp, loc)
		if err != nil {
			return model.NormalizedEvent{}, fmt.Errorf("parse timestamp: %w", err)
		}
		ts = parsed.UTC()
	}

	res := ParseResult(fields.Result, fields.ErrorCode)
	errCode := strings.TrimSpace(fields.ErrorCode)

	return model.NormalizedEvent{
		Timestamp: ts,
		ReaderID:  reader,
		UID:       strings.TrimSpace(fields.UID),
		Result:    res,
		ErrorCode: errCode,
		Source:    "log",
		Raw:       fields.Raw,
	}, nil
}

func ParseResult(result string, errorCode string) model.Result {
	n := strings.ToLower(strings.TrimSpace(result))
	switch n {
	case "ok", "success", "allow", "allowed", "granted", "pass":
		return model.ResultSuccess
	case "fail", "failure", "denied", "reject", "rejected", "timeout", "error":
		return model.ResultFailure
	}
	if strings.TrimSpace(errorCode) != "" {
		return model.ResultFailure
	}
	return model.ResultSuccess
}

var timestampLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02 15:04:05.000",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05.000",
	"2006-01-02T15:04:05Z0700",
	"2006-01-02 15:04:05Z0700",
	"Jan 02 15:04:05",
	"Jan 2 15:04:05",
}

func ParseTimestamp(value string, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("empty timestamp")
	}
	if isNumeric(value) {
		if ts, err := parseUnix(value); err == nil {
			return ts, nil
		}
	}
	for _, layout := range timestampLayouts {
		if layout == "Jan 02 15:04:05" || layout == "Jan 2 15:04:05" {
			if t, err := time.ParseInLocation(layout, value, loc); err == nil {
				now := time.Now().In(loc)
				return time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc), nil
			}
			continue
		}
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp format: %q", value)
}

func isNumeric(value string) bool {
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return len(value) > 0
}

func parseUnix(value string) (time.Time, error) {
	if len(value) >= 13 {
		ms, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(0, ms*int64(time.Millisecond)).UTC(), nil
	}
	sec, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, 0).UTC(), nil
}
