package cli

import (
	"bytes"
	"encoding/json"
	"errors"
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
	for _, command := range []string{"doctor", "run", "version"} {
		if !strings.Contains(stdout.String(), command) {
			t.Fatalf("help does not contain %q:\n%s", command, stdout.String())
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
