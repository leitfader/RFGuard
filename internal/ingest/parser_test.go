package ingest

import "testing"

func TestParsePlainText(t *testing.T) {
	p := NewParser()
	line := "2026-02-23 12:34:56 Reader1 UID=04AABBCC RESULT=FAIL ERROR=TIMEOUT"
	fields, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if fields.ReaderID != "Reader1" {
		t.Fatalf("reader id: %s", fields.ReaderID)
	}
	if fields.UID != "04AABBCC" {
		t.Fatalf("uid: %s", fields.UID)
	}
	if fields.Result == "" || fields.ErrorCode == "" {
		t.Fatalf("result/error missing")
	}
}

func TestParseCSV(t *testing.T) {
	p := NewParser()
	if fields, _ := p.ParseLine("timestamp,reader_id,uid,result,error"); fields != nil {
		t.Fatalf("expected header to return nil")
	}
	fields, err := p.ParseLine("2026-02-23T12:34:56Z,reader01,04AABBCC,failure,AUTH_TIMEOUT")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if fields.ReaderID != "reader01" || fields.UID != "04AABBCC" {
		t.Fatalf("csv parse mismatch")
	}
}

func TestParseJSON(t *testing.T) {
	p := NewParser()
	line := `{"timestamp":"2026-02-23T12:34:56Z","reader":"reader01","card":"04AABBCC","status":"denied"}`
	fields, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if fields.ReaderID != "reader01" || fields.UID != "04AABBCC" {
		t.Fatalf("json parse mismatch")
	}
}
