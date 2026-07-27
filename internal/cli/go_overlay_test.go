package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAddGoTrustOverlay(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("Go trust overlay is only needed on macOS and Windows")
	}
	tempDirectory := t.TempDir()
	caPath := filepath.Join(tempDirectory, "ca.pem")
	if err := os.WriteFile(caPath, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	environment, note, err := addGoTrustOverlay(os.Environ(), tempDirectory, caPath)
	if err != nil {
		t.Fatalf("addGoTrustOverlay returned an error: %v", err)
	}
	if !strings.Contains(note, runtime.GOOS) {
		t.Fatalf("note = %q", note)
	}
	if !strings.Contains(environmentValue(environment, "GOFLAGS"), "-overlay=") {
		t.Fatalf("GOFLAGS does not contain an overlay: %q", environmentValue(environment, "GOFLAGS"))
	}
	if got := environmentValue(environment, "AUTOCURL_CA_FILE"); got != caPath {
		t.Fatalf("AUTOCURL_CA_FILE = %q, want %q", got, caPath)
	}
}
