package cli

import (
	"fmt"
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

func TestDiscoverGoExecutableFromLoginShellWithGUIPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("regression covers macOS GUI applications with a minimal PATH")
	}

	tempDirectory := t.TempDir()
	goRoot := filepath.Join(tempDirectory, "go-root")
	rootSource := filepath.Join(goRoot, "src", "crypto", "x509", "root_darwin.go")
	if err := os.MkdirAll(filepath.Dir(rootSource), 0o700); err != nil {
		t.Fatal(err)
	}
	fakePlatformRoots := `package x509

import (
	"errors"
)

func (c *Certificate) systemVerify(opts *VerifyOptions) (chains [][]*Certificate, err error) {
	return nil, errors.New("platform verifier")
}
`
	if err := os.WriteFile(rootSource, []byte(fakePlatformRoots), 0o600); err != nil {
		t.Fatal(err)
	}

	fakeGo := filepath.Join(tempDirectory, "go")
	goScript := "#!/bin/sh\nprintf '%s\\n' \"$FAKE_GOROOT\"\n"
	if err := os.WriteFile(fakeGo, []byte(goScript), 0o700); err != nil {
		t.Fatal(err)
	}
	fakeShell := filepath.Join(tempDirectory, "fake-shell")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %q\n", fakeGo)
	if err := os.WriteFile(fakeShell, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	environment := []string{
		"HOME=" + tempDirectory,
		"PATH=/usr/bin:/bin",
		"SHELL=" + fakeShell,
		"FAKE_GOROOT=" + goRoot,
	}
	caPath := filepath.Join(tempDirectory, "ca.pem")
	if err := os.WriteFile(caPath, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	environment, note, err := addGoTrustOverlayWithCandidates(
		environment,
		tempDirectory,
		caPath,
		nil,
	)
	if err != nil {
		t.Fatalf("addGoTrustOverlayWithCandidates returned an error: %v", err)
	}
	if !strings.Contains(note, "enabled for darwin") {
		t.Fatalf("note = %q", note)
	}
	if !strings.Contains(environmentValue(environment, "GOFLAGS"), "-overlay=") {
		t.Fatalf("GOFLAGS does not contain an overlay: %q", environmentValue(environment, "GOFLAGS"))
	}
	if got := environmentValue(environment, "AUTOCURL_CA_FILE"); got != caPath {
		t.Fatalf("AUTOCURL_CA_FILE = %q, want %q", got, caPath)
	}
}

func TestBuildGoTrustOverlaySourcePreservesPlatformVerification(t *testing.T) {
	platformRoots := []byte(`package x509

import (
	"errors"
)

func (c *Certificate) systemVerify(opts *VerifyOptions) (chains [][]*Certificate, err error) {
	return nil, errors.New("platform verifier")
}

func loadSystemRoots() (*CertPool, error) {
	return &CertPool{systemPool: true}, nil
}
`)

	replacement, err := buildGoTrustOverlaySource(platformRoots)
	if err != nil {
		t.Fatalf("buildGoTrustOverlaySource returned an error: %v", err)
	}
	source := string(replacement)
	for _, expected := range []string{
		`"os"`,
		"func (c *Certificate) autocurlPlatformVerify",
		"chains, platformErr := c.autocurlPlatformVerify(opts)",
		"fallback.Roots = roots",
		"return c.Verify(fallback)",
		"return &CertPool{systemPool: true}, nil",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("overlay source does not contain %q:\n%s", expected, source)
		}
	}
	if strings.Count(source, "func (c *Certificate) systemVerify") != 1 {
		t.Fatalf("overlay source should define exactly one systemVerify:\n%s", source)
	}
}

func TestBuildGoTrustOverlaySourceRejectsUnexpectedPlatformSource(t *testing.T) {
	_, err := buildGoTrustOverlaySource([]byte("package x509\n"))
	if err == nil {
		t.Fatal("buildGoTrustOverlaySource accepted a source without systemVerify")
	}
}
