package ingest

import (
	"encoding/csv"
	"regexp"
	"strings"

	"rfguard/internal/normalize"
)

var (
	reTimestamp = regexp.MustCompile(`^\s*([0-9]{4}-[0-9]{2}-[0-9]{2}[ T][0-9:.+-Z]+)`)
	reKV        = regexp.MustCompile(`(?i)([a-zA-Z_]+)=([^\s]+)`)
	reSyslogTS  = regexp.MustCompile(`^\s*([A-Za-z]{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})`)
)

type Parser struct {
	csv *CSVParser
}

func NewParser() *Parser {
	return &Parser{csv: NewCSVParser()}
}

func (p *Parser) ParseLine(line string) (*normalize.EventFields, error) {
	trim := strings.TrimSpace(line)
	if trim == "" {
		return nil, nil
	}
	if looksLikeJSON(trim) {
		if fields, err := parseJSON(trim); err == nil {
			fields.Raw = line
			return fields, nil
		}
	}
	if strings.Contains(trim, ",") {
		fields, err := p.csv.Parse(trim)
		if err == nil {
			if fields == nil {
				return nil, nil
			}
			fields.Raw = line
			return fields, nil
		}
	}
	fields, err := parsePlain(trim)
	if err != nil {
		return nil, err
	}
	fields.Raw = line
	return fields, nil
}

func looksLikeJSON(s string) bool {
	for _, ch := range s {
		if ch == '{' || ch == '[' {
			return true
		}
		if ch > ' ' {
			return false
		}
	}
	return false
}

func parseJSON(line string) (*normalize.EventFields, error) {
	return ParseJSONBytes([]byte(line))
}

func parsePlain(line string) (*normalize.EventFields, error) {
	fields := &normalize.EventFields{Extras: map[string]string{}}
	ts, rest := extractTimestamp(line)
	fields.Timestamp = ts

	kv := map[string]string{}
	for _, match := range reKV.FindAllStringSubmatch(line, -1) {
		key := strings.ToLower(match[1])
		kv[key] = match[2]
	}
	fields.ReaderID = firstNonEmpty(kv, "reader_id", "reader", "readerid", "device", "terminal")
	fields.UID = firstNonEmpty(kv, "uid", "card", "card_id", "cardid")
	fields.Result = firstNonEmpty(kv, "result", "status", "outcome")
	fields.ErrorCode = firstNonEmpty(kv, "error", "error_code", "err")
	for k, v := range kv {
		fields.Extras[k] = v
	}

	if fields.ReaderID == "" && rest != "" {
		tokens := strings.Fields(rest)
		if len(tokens) > 0 {
			fields.ReaderID = tokens[0]
		}
	}
	if fields.Timestamp == "" {
		if ts2, _ := extractTimestamp(rest); ts2 != "" {
			fields.Timestamp = ts2
		}
	}
	return fields, nil
}

func extractTimestamp(line string) (string, string) {
	m := reTimestamp.FindStringSubmatchIndex(line)
	if len(m) >= 4 {
		ts := strings.TrimSpace(line[m[2]:m[3]])
		rest := strings.TrimSpace(line[m[3]:])
		return ts, rest
	}
	m = reSyslogTS.FindStringSubmatchIndex(line)
	if len(m) >= 4 {
		ts := strings.TrimSpace(line[m[2]:m[3]])
		rest := strings.TrimSpace(line[m[3]:])
		return ts, rest
	}
	return "", line
}

func firstNonEmpty(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(m[k]); v != "" {
			return v
		}
	}
	return ""
}

type CSVParser struct {
	header []string
}

func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

func (p *CSVParser) Parse(line string) (*normalize.EventFields, error) {
	r := csv.NewReader(strings.NewReader(line))
	r.TrimLeadingSpace = true
	record, err := r.Read()
	if err != nil {
		return nil, err
	}
	if len(record) == 0 {
		return nil, nil
	}
	if p.header == nil && looksLikeHeader(record) {
		p.header = normalizeHeader(record)
		return nil, nil
	}
	fields := &normalize.EventFields{Extras: map[string]string{}}
	if p.header != nil {
		for i, name := range p.header {
			if i >= len(record) {
				break
			}
			assignField(fields, name, record[i])
		}
	} else {
		if len(record) >= 1 {
			fields.Timestamp = record[0]
		}
		if len(record) >= 2 {
			fields.ReaderID = record[1]
		}
		if len(record) >= 3 {
			fields.UID = record[2]
		}
		if len(record) >= 4 {
			fields.Result = record[3]
		}
		if len(record) >= 5 {
			fields.ErrorCode = record[4]
		}
	}
	return fields, nil
}

func looksLikeHeader(record []string) bool {
	for _, v := range record {
		v = strings.ToLower(strings.TrimSpace(v))
		switch v {
		case "timestamp", "time", "ts", "reader", "reader_id", "uid", "card", "result", "status", "error", "error_code":
			return true
		}
	}
	return false
}

func normalizeHeader(record []string) []string {
	out := make([]string, len(record))
	for i, v := range record {
		out[i] = strings.ToLower(strings.TrimSpace(v))
	}
	return out
}

func assignField(fields *normalize.EventFields, name string, value string) {
	name = strings.ToLower(strings.TrimSpace(name))
	value = strings.TrimSpace(value)
	switch name {
	case "timestamp", "time", "ts":
		fields.Timestamp = value
	case "reader", "reader_id", "readerid", "device", "terminal":
		fields.ReaderID = value
	case "uid", "card", "card_id", "cardid":
		fields.UID = value
	case "result", "status", "outcome":
		fields.Result = value
	case "error", "error_code", "err":
		fields.ErrorCode = value
	default:
		if fields.Extras != nil {
			fields.Extras[name] = value
		}
	}
}
