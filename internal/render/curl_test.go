package render

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Lingbo-Huang/autocurl/internal/config"
)

func TestCurlRedactsSecretsAndAddsReplayHeader(t *testing.T) {
	result := Curl(Request{
		Method: "POST",
		URL:    "https://api.example.test/orders?access_token=query-secret&view=full",
		Headers: http.Header{
			"Authorization": []string{"Bearer header-secret"},
			"Content-Type":  []string{"application/json"},
			"X-Request-Id":  []string{"request-123"},
		},
		Body: []byte(`{"name":"demo","password":"body-secret","nested":{"api_key":"key-secret"}}`),
		ReplayHeaders: []config.Header{
			{Name: "X-Debug-Info", Value: "true"},
		},
		IncludeWriteOut: true,
	})

	if !result.Redacted {
		t.Fatal("expected redaction to be reported")
	}
	for _, secret := range []string{"query-secret", "header-secret", "body-secret", "key-secret"} {
		if strings.Contains(result.Command, secret) {
			t.Fatalf("generated cURL leaked %q:\n%s", secret, result.Command)
		}
	}
	for _, wanted := range []string{
		"--header 'Authorization: [REDACTED]'",
		"--header 'X-Debug-Info: true'",
		`"password":"[REDACTED]"`,
		"%5BREDACTED%5D",
		"%{time_total}",
	} {
		if !strings.Contains(result.Command, wanted) {
			t.Fatalf("generated cURL does not contain %q:\n%s", wanted, result.Command)
		}
	}
}

func TestCurlShowSecrets(t *testing.T) {
	result := Curl(Request{
		Method:      "GET",
		URL:         "https://example.test/?token=visible",
		Headers:     http.Header{"Authorization": []string{"Bearer visible"}},
		ShowSecrets: true,
	})
	if result.Redacted {
		t.Fatal("did not expect redaction")
	}
	if !strings.Contains(result.Command, "Bearer visible") ||
		!strings.Contains(result.Command, "token=visible") {
		t.Fatalf("expected secrets to remain visible:\n%s", result.Command)
	}
}

func TestCurlEscapesSingleQuotes(t *testing.T) {
	result := Curl(Request{
		Method:  "POST",
		URL:     "https://example.test/",
		Headers: http.Header{"Content-Type": []string{"text/plain"}},
		Body:    []byte("it's valid"),
	})
	if !strings.Contains(result.Command, `'it'"'"'s valid'`) {
		t.Fatalf("body was not shell escaped:\n%s", result.Command)
	}
}

func TestCurlDecodesCompressedResponses(t *testing.T) {
	result := Curl(Request{
		Method:  "GET",
		URL:     "https://example.test/",
		Headers: http.Header{"Accept-Encoding": []string{"gzip"}},
	})
	if !strings.Contains(result.Command, "--compressed") {
		t.Fatalf("generated cURL should decode the requested encoding:\n%s", result.Command)
	}
}

func TestCurlPreservesHTTP2AndOmitsBinaryGRPCBody(t *testing.T) {
	result := Curl(Request{
		Method:   "POST",
		URL:      "https://example.test/demo.Service/Call",
		Protocol: "HTTP/2.0",
		Kind:     "grpc",
		Headers:  http.Header{"Content-Type": []string{"application/grpc"}},
		Body:     []byte{0, 0, 0, 0, 1, 'x'},
	})
	if !strings.Contains(result.Command, "--http2") {
		t.Fatalf("generated cURL does not request HTTP/2:\n%s", result.Command)
	}
	if strings.Contains(result.Command, "--data-binary") {
		t.Fatalf("generated cURL should not place binary gRPC bytes in an argument:\n%s", result.Command)
	}
	if !strings.Contains(result.Warning, "grpcurl") {
		t.Fatalf("gRPC replay warning = %q", result.Warning)
	}
}
