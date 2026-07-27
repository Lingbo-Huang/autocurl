package config

import (
	"fmt"
	"net/http"
	"strings"
)

// Header is a validated HTTP header supplied on the command line.
type Header struct {
	Name  string
	Value string
}

func ParseHeader(raw string) (Header, error) {
	name, value, ok := strings.Cut(raw, ":")
	if !ok {
		return Header{}, fmt.Errorf("header %q must use 'Name: value' format", raw)
	}

	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if name == "" {
		return Header{}, fmt.Errorf("header name cannot be empty")
	}
	if !validHeaderName(name) {
		return Header{}, fmt.Errorf("invalid HTTP header name %q", name)
	}
	if strings.ContainsAny(value, "\r\n") {
		return Header{}, fmt.Errorf("header %q contains a newline", name)
	}

	return Header{Name: http.CanonicalHeaderKey(name), Value: value}, nil
}

func ApplyHeaders(dst http.Header, headers []Header) {
	for _, header := range headers {
		dst.Set(header.Name, header.Value)
	}
}

func validHeaderName(name string) bool {
	for _, char := range name {
		if char >= 'a' && char <= 'z' ||
			char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' {
			continue
		}
		switch char {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}
