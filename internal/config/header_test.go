package config

import (
	"net/http"
	"testing"
)

func TestParseHeader(t *testing.T) {
	header, err := ParseHeader("x-debug-info: true")
	if err != nil {
		t.Fatalf("ParseHeader returned an error: %v", err)
	}
	if header.Name != "X-Debug-Info" || header.Value != "true" {
		t.Fatalf("unexpected header: %#v", header)
	}
}

func TestParseHeaderRejectsNewline(t *testing.T) {
	if _, err := ParseHeader("X-Test: safe\r\nX-Evil: yes"); err == nil {
		t.Fatal("expected a newline validation error")
	}
}

func TestApplyHeadersOverridesExistingValue(t *testing.T) {
	headers := http.Header{"X-Test-Info": []string{"old"}}
	ApplyHeaders(headers, []Header{{Name: "X-Test-Info", Value: "new"}})
	if got := headers.Get("X-Test-Info"); got != "new" {
		t.Fatalf("got %q, want new", got)
	}
}
