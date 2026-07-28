package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Lingbo-Huang/autocurl/internal/capture"
)

func TestHelpListsPrimaryCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	for _, command := range []string{"doctor", "proxy", "render", "run", "version"} {
		if !strings.Contains(stdout.String(), command) {
			t.Fatalf("help does not contain %q:\n%s", command, stdout.String())
		}
	}
}

func TestProxyHelpDocumentsIDEProtocol(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"proxy", "--help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	for _, wanted := range []string{"ready", "request", "--lifetime-stdin"} {
		if !strings.Contains(stderr.String(), wanted) {
			t.Fatalf("proxy help does not contain %q:\n%s", wanted, stderr.String())
		}
	}
}

func TestDoctorJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"--json", "doctor"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d; stderr: %s", exitCode, stderr.String())
	}
	var result struct {
		SchemaVersion string `json:"schema_version"`
		Version       string `json:"version"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("doctor output is not JSON: %v\n%s", err, stdout.String())
	}
	if result.SchemaVersion != "1" || result.Version == "" {
		t.Fatalf("unexpected doctor result: %#v", result)
	}
}

func TestRunRejectsInvalidReplayHeader(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run(
		[]string{"run", "--replay-header", "missing-colon", "--", "true"},
		&stdout,
		&stderr,
	)
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "Name: value") {
		t.Fatalf("unexpected error:\n%s", stderr.String())
	}
}

func TestEmitterRedactsURLOutsideCurl(t *testing.T) {
	var output bytes.Buffer
	emitter := newEmitter(emitterOptions{
		Writer:    &output,
		JSON:      true,
		All:       true,
		StatusMin: 400,
	})
	rawURL := "https://example.test/fail?token=top-secret"
	emitter.Emit(capture.Event{
		Timestamp: time.Now(),
		Method:    http.MethodGet,
		URL:       rawURL,
		Status:    0,
		Error:     errors.New("request " + rawURL + " failed"),
	})

	if strings.Contains(output.String(), "top-secret") {
		t.Fatalf("JSON event leaked a query secret:\n%s", output.String())
	}
	if !strings.Contains(output.String(), "%5BREDACTED%5D") {
		t.Fatalf("JSON event did not contain a redacted URL:\n%s", output.String())
	}
	if !strings.Contains(output.String(), `"type":"request"`) {
		t.Fatalf("JSON event has no request type:\n%s", output.String())
	}
}

func TestEmitterKeepsProxyErrorsAsJSON(t *testing.T) {
	var output bytes.Buffer
	emitter := newEmitter(emitterOptions{
		Writer: &output,
		JSON:   true,
		All:    true,
	})
	emitter.Emit(capture.Event{Error: errors.New("proxy stopped unexpectedly")})
	var event struct {
		Type  string `json:"type"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatalf("proxy error output is not JSON: %v\n%s", err, output.String())
	}
	if event.Type != "error" || event.Error != "proxy stopped unexpectedly" {
		t.Fatalf("unexpected proxy error event: %#v", event)
	}
}

func TestEnvironmentMapContainsOnlyProvidedOverrides(t *testing.T) {
	got := environmentMap([]string{"HTTP_PROXY=http://127.0.0.1:1234", "EMPTY=", "invalid"})
	if len(got) != 2 {
		t.Fatalf("environment map = %#v", got)
	}
	if got["HTTP_PROXY"] != "http://127.0.0.1:1234" {
		t.Fatalf("HTTP_PROXY = %q", got["HTTP_PROXY"])
	}
}

func TestMergeBypassTargetsPreservesExistingValues(t *testing.T) {
	got := mergeBypassTargets(
		[]string{
			"NO_PROXY=localhost,127.0.0.1",
			"no_proxy=127.0.0.1,.internal.example",
			"no_grpc_proxy=10.4.44.94",
		},
		[]string{"10.4.44.94", "10.61.98.0/24"},
	)
	want := "localhost,127.0.0.1,.internal.example,10.4.44.94,10.61.98.0/24"
	if got != want {
		t.Fatalf("merged bypass = %q, want %q", got, want)
	}
}

func TestBuildChildEnvironmentDoesNotClearBypassByDefault(t *testing.T) {
	environment, _ := buildChildEnvironment(
		nil,
		"127.0.0.1:1234",
		filepath.Join(t.TempDir(), "missing-ca.pem"),
		t.TempDir(),
		nil,
	)
	got := environmentMap(environment)
	for _, name := range []string{"NO_PROXY", "no_proxy", "no_grpc_proxy"} {
		if _, exists := got[name]; exists {
			t.Fatalf("%s should not be injected when no bypass is configured: %#v", name, got)
		}
	}
}

func TestBuildChildEnvironmentAppliesBypassToHTTPAndGRPC(t *testing.T) {
	environment, _ := buildChildEnvironment(
		[]string{"NO_PROXY=localhost"},
		"127.0.0.1:1234",
		filepath.Join(t.TempDir(), "missing-ca.pem"),
		t.TempDir(),
		[]string{"10.4.44.94", "10.61.98.0/24"},
	)
	got := environmentMap(environment)
	want := "localhost,10.4.44.94,10.61.98.0/24"
	for _, name := range []string{"NO_PROXY", "no_proxy", "no_grpc_proxy"} {
		if got[name] != want {
			t.Fatalf("%s = %q, want %q", name, got[name], want)
		}
	}
}

func TestJavaNonProxyHostsUsesJavaSeparators(t *testing.T) {
	got := javaNonProxyHosts("localhost,.internal.example,10.4.44.94,10.61.98.0/24")
	want := "localhost|*.internal.example|10.4.44.94|10.61.98.*"
	if got != want {
		t.Fatalf("Java non-proxy hosts = %q, want %q", got, want)
	}
}

func TestProxyLifetimeEndsWhenStdinCloses(t *testing.T) {
	stdinReader, stdinWriter := io.Pipe()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- proxyCommand(
			[]string{"--json", "--lifetime-stdin"},
			false,
			stdinReader,
			&stdout,
			&stderr,
		)
	}()
	if err := stdinWriter.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case exitCode := <-done:
		if exitCode != 0 {
			t.Fatalf("exit code = %d; stderr: %s", exitCode, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("proxy did not stop after stdin closed")
	}
	var ready proxyReadyEvent
	if err := json.Unmarshal(stdout.Bytes(), &ready); err != nil {
		t.Fatalf("ready output is not JSON: %v\n%s", err, stdout.String())
	}
	if ready.Type != "ready" || ready.ProxyURL == "" || len(ready.Environment) == 0 {
		t.Fatalf("unexpected ready event: %#v", ready)
	}
}

func TestProxyControlProtocolPausesAndResumesRecording(t *testing.T) {
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- proxyCommand(
			[]string{"--json", "--lifetime-stdin"},
			false,
			stdinReader,
			stdoutWriter,
			&stderr,
		)
		_ = stdoutWriter.Close()
	}()

	decoder := json.NewDecoder(stdoutReader)
	var ready proxyReadyEvent
	if err := decoder.Decode(&ready); err != nil {
		t.Fatalf("decode ready event: %v; stderr: %s", err, stderr.String())
	}

	for _, test := range []struct {
		command   string
		recording bool
	}{
		{command: "pause", recording: false},
		{command: "resume", recording: true},
	} {
		if _, err := fmt.Fprintf(stdinWriter, "{\"command\":%q}\n", test.command); err != nil {
			t.Fatalf("write %s control: %v", test.command, err)
		}
		var state struct {
			Type      string `json:"type"`
			Recording bool   `json:"recording"`
		}
		if err := decoder.Decode(&state); err != nil {
			t.Fatalf("decode %s state: %v; stderr: %s", test.command, err, stderr.String())
		}
		if state.Type != "state" || state.Recording != test.recording {
			t.Fatalf("%s state = %#v", test.command, state)
		}
	}

	if err := stdinWriter.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case exitCode := <-done:
		if exitCode != 0 {
			t.Fatalf("exit code = %d; stderr: %s", exitCode, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("proxy did not stop after control stdin closed")
	}
}

func TestCreateJavaTruststore(t *testing.T) {
	if _, err := exec.LookPath("keytool"); err != nil {
		t.Skip("keytool is not installed")
	}
	authority, err := capture.NewAuthority()
	if err != nil {
		t.Fatalf("create authority: %v", err)
	}
	tempDirectory := t.TempDir()
	caPath := filepath.Join(tempDirectory, "ca.pem")
	if err := os.WriteFile(caPath, authority.PEM(), 0o600); err != nil {
		t.Fatalf("write CA: %v", err)
	}

	truststorePath, err := createJavaTruststore(caPath, tempDirectory)
	if err != nil {
		t.Fatalf("createJavaTruststore returned an error: %v", err)
	}
	if info, err := os.Stat(truststorePath); err != nil || info.Size() == 0 {
		t.Fatalf("truststore was not created correctly: info=%v err=%v", info, err)
	}
}

func TestRenderDebuggerRequestEnvelope(t *testing.T) {
	tempDirectory := t.TempDir()
	requestPath := filepath.Join(tempDirectory, "request.json")
	requestJSON := `{
		"method": "POST",
		"url": "https://api.example.test/orders?token=query-secret",
		"protocol": "HTTP/2",
		"headers": {
			"Authorization": "Bearer header-secret",
			"X-Request-Id": ["request-42"]
		},
		"body": {
			"order_id": "demo-42",
			"password": "body-secret"
		}
	}`
	if err := os.WriteFile(requestPath, []byte(requestJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{"render", "--request", requestPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d; stderr: %s", exitCode, stderr.String())
	}
	for _, wanted := range []string{
		"--request 'POST'",
		"--http2",
		"--header 'Content-Type: application/json'",
		`"order_id":"demo-42"`,
	} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Fatalf("rendered cURL does not contain %q:\n%s", wanted, stdout.String())
		}
	}
	for _, secret := range []string{"query-secret", "header-secret", "body-secret"} {
		if strings.Contains(stdout.String(), secret) {
			t.Fatalf("rendered cURL leaked %q:\n%s", secret, stdout.String())
		}
	}
}

func TestRenderFlagsWriteCurlFile(t *testing.T) {
	tempDirectory := t.TempDir()
	bodyPath := filepath.Join(tempDirectory, "body.json")
	outputPath := filepath.Join(tempDirectory, "request.sh")
	if err := os.WriteFile(bodyPath, []byte(`{"order_id":"demo-42"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{
		"render",
		"--url", "https://api.example.test/orders",
		"--header", "Content-Type: application/json",
		"--body-file", bodyPath,
		"--output", outputPath,
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d; stderr: %s", exitCode, stderr.String())
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "--request 'POST'") ||
		!strings.Contains(string(output), `"order_id":"demo-42"`) {
		t.Fatalf("unexpected cURL file:\n%s", output)
	}
}

func TestEmitterFiltersCopiesAndWritesSelectedCurl(t *testing.T) {
	var output, diagnostics bytes.Buffer
	var copied string
	outputPath := filepath.Join(t.TempDir(), "selected.sh")
	emitter := newEmitter(emitterOptions{
		Writer:           &output,
		DiagnosticWriter: &diagnostics,
		All:              true,
		StatusMin:        400,
		Match:            "/orders",
		Method:           "POST",
		Copy:             true,
		OutputPath:       outputPath,
		CopyText: func(value string) error {
			copied = value
			return nil
		},
	})
	emitter.Emit(capture.Event{
		Timestamp: time.Now(),
		Method:    http.MethodGet,
		URL:       "https://api.example.test/orders",
		Status:    http.StatusOK,
	})
	emitter.Emit(capture.Event{
		Timestamp: time.Now(),
		Method:    http.MethodPost,
		URL:       "https://api.example.test/health",
		Status:    http.StatusOK,
	})
	emitter.Emit(capture.Event{
		Timestamp: time.Now(),
		Method:    http.MethodPost,
		URL:       "https://api.example.test/orders",
		Status:    http.StatusCreated,
	})

	if !strings.Contains(output.String(), "[autocurl #1]") ||
		strings.Contains(output.String(), "/health") {
		t.Fatalf("unexpected filtered output:\n%s", output.String())
	}
	if !strings.Contains(copied, "https://api.example.test/orders") {
		t.Fatalf("clipboard did not receive selected cURL: %q", copied)
	}
	saved, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != copied+"\n" {
		t.Fatalf("saved cURL != copied cURL:\n%s", saved)
	}
	for _, wanted := range []string{"copied cURL", "wrote cURL"} {
		if !strings.Contains(diagnostics.String(), wanted) {
			t.Fatalf("diagnostics do not contain %q:\n%s", wanted, diagnostics.String())
		}
	}
}
