package ingest

import (
	"encoding/json"
	"fmt"
	"strings"

	"rfguard/internal/normalize"
)

func ParseJSONBytes(data []byte) (*normalize.EventFields, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return ParseJSONMap(obj), nil
}

func ParseJSONMap(obj map[string]interface{}) *normalize.EventFields {
	fields := &normalize.EventFields{Extras: map[string]string{}}
	for key, val := range obj {
		fields.Extras[strings.ToLower(key)] = fmt.Sprint(val)
	}
	fields.Timestamp = firstNonEmpty(fields.Extras, "timestamp", "time", "ts")
	fields.ReaderID = firstNonEmpty(fields.Extras, "reader_id", "reader", "readerid", "device", "terminal")
	fields.UID = firstNonEmpty(fields.Extras, "uid", "card", "card_id", "cardid")
	fields.Result = firstNonEmpty(fields.Extras, "result", "status", "outcome")
	fields.ErrorCode = firstNonEmpty(fields.Extras, "error", "error_code", "err")
	return fields
}
