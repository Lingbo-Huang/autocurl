package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Lingbo-Huang/autocurl/internal/config"
)

type Request struct {
	Method          string
	URL             string
	Protocol        string
	Kind            string
	Headers         http.Header
	Body            []byte
	BodyTruncated   bool
	ReplayHeaders   []config.Header
	ShowSecrets     bool
	IncludeWriteOut bool
}

type Result struct {
	Command    string
	DisplayURL string
	Redacted   bool
	Warning    string
}

var sensitiveNames = map[string]struct{}{
	"api-key":             {},
	"apikey":              {},
	"authorization":       {},
	"client-secret":       {},
	"cookie":              {},
	"password":            {},
	"passwd":              {},
	"proxy-authorization": {},
	"refresh-token":       {},
	"secret":              {},
	"set-cookie":          {},
	"token":               {},
	"x-api-key":           {},
	"x-auth-token":        {},
}

func Curl(request Request) Result {
	headers := request.Headers.Clone()
	config.ApplyHeaders(headers, request.ReplayHeaders)

	redacted := false
	requestURL := request.URL
	body := append([]byte(nil), request.Body...)

	if !request.ShowSecrets {
		var changed bool
		requestURL, changed = redactURL(requestURL)
		redacted = redacted || changed

		for name := range headers {
			if isSensitive(name) {
				headers.Set(name, "[REDACTED]")
				redacted = true
			}
		}

		body, changed = redactBody(body, headers.Get("Content-Type"))
		redacted = redacted || changed
	}

	parts := []string{
		"curl",
		"--request " + shellQuote(request.Method),
		"--url " + shellQuote(requestURL),
	}
	if protocolOption := curlProtocolOption(request.Protocol); protocolOption != "" {
		parts = append(parts, protocolOption)
	}

	headerNames := make([]string, 0, len(headers))
	for name := range headers {
		if skipHeader(name) {
			continue
		}
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)
	for _, name := range headerNames {
		for _, value := range headers.Values(name) {
			parts = append(parts, "--header "+shellQuote(name+": "+value))
		}
	}

	binaryBody := len(body) > 0 && (!utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0)
	if len(body) > 0 && !binaryBody {
		parts = append(parts, "--data-binary "+shellQuote(string(body)))
	}

	acceptEncoding := strings.ToLower(headers.Get("Accept-Encoding"))
	if acceptEncoding != "" && acceptEncoding != "identity" {
		parts = append(parts, "--compressed")
	}

	parts = append(parts,
		"--silent",
		"--show-error",
		"--fail-with-body",
		"--connect-timeout 5",
		"--max-time 30",
	)

	if request.IncludeWriteOut {
		parts = append(parts, "--write-out "+shellQuote(
			"\nDNS: %{time_namelookup}s"+
				"\nConnect: %{time_connect}s"+
				"\nTLS: %{time_appconnect}s"+
				"\nTTFB: %{time_starttransfer}s"+
				"\nTotal: %{time_total}s"+
				"\nHTTP: %{http_code}\n",
		))
	}

	warnings := make([]string, 0, 2)
	if request.BodyTruncated {
		warnings = append(warnings,
			"request body exceeded the capture limit; the generated cURL contains only the captured prefix")
	}
	if binaryBody {
		message := "binary request body was omitted because shell arguments cannot safely contain it"
		if request.Kind == "grpc" {
			message += "; use grpcurl with the service .proto or reflection for semantic gRPC replay"
		}
		warnings = append(warnings, message)
	}

	return Result{
		Command:    strings.Join(parts, " \\\n  "),
		DisplayURL: requestURL,
		Redacted:   redacted,
		Warning:    strings.Join(warnings, "; "),
	}
}

func curlProtocolOption(protocol string) string {
	switch protocol {
	case "HTTP/1.0":
		return "--http1.0"
	case "HTTP/1.1":
		return "--http1.1"
	case "HTTP/2", "HTTP/2.0":
		return "--http2"
	default:
		return ""
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func skipHeader(name string) bool {
	switch strings.ToLower(name) {
	case "content-length", "host", "proxy-connection", "transfer-encoding":
		return true
	default:
		return false
	}
}

func isSensitive(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	if _, ok := sensitiveNames[normalized]; ok {
		return true
	}
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password")
}

func redactURL(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, false
	}
	query := parsed.Query()
	changed := false
	for name := range query {
		if !isSensitive(name) {
			continue
		}
		query.Set(name, "[REDACTED]")
		changed = true
	}
	if changed {
		parsed.RawQuery = query.Encode()
	}
	return parsed.String(), changed
}

func redactBody(body []byte, contentType string) ([]byte, bool) {
	if len(body) == 0 {
		return body, false
	}

	mediaType, _, _ := mime.ParseMediaType(contentType)
	switch mediaType {
	case "application/json", "application/ld+json", "application/problem+json":
		return redactJSON(body)
	case "application/x-www-form-urlencoded":
		return redactForm(body)
	default:
		if json.Valid(body) {
			return redactJSON(body)
		}
		return body, false
	}
}

func redactJSON(body []byte) ([]byte, bool) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return body, false
	}
	if !redactJSONValue(value) {
		return body, false
	}
	redacted, err := json.Marshal(value)
	if err != nil {
		return body, false
	}
	return redacted, true
}

func redactJSONValue(value any) bool {
	changed := false
	switch typed := value.(type) {
	case map[string]any:
		for name, child := range typed {
			if isSensitive(name) {
				typed[name] = "[REDACTED]"
				changed = true
				continue
			}
			changed = redactJSONValue(child) || changed
		}
	case []any:
		for _, child := range typed {
			changed = redactJSONValue(child) || changed
		}
	}
	return changed
}

func redactForm(body []byte) ([]byte, bool) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return body, false
	}
	changed := false
	for name := range values {
		if isSensitive(name) {
			values.Set(name, "[REDACTED]")
			changed = true
		}
	}
	if !changed {
		return body, false
	}
	return []byte(values.Encode()), true
}

func Summary(method, requestURL string, status int, duration string, requestErr error) string {
	outcome := fmt.Sprintf("%d", status)
	if requestErr != nil {
		outcome = "ERROR: " + requestErr.Error()
	}
	return fmt.Sprintf("%s %s -> %s (%s)", method, requestURL, outcome, duration)
}
