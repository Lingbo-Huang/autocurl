package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// addGoTrustOverlay works around a deliberate Go standard-library limitation:
// SSL_CERT_FILE is ignored by crypto/x509 on macOS and Windows. It affects only
// Go programs built by a go command inside the wrapped process tree. Existing
// binaries still require an OS-trusted CA or application-specific TLS setup.
func addGoTrustOverlay(environment []string, tempDirectory, caPath string) ([]string, string, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return environment, "", nil
	}

	goExecutable, err := exec.LookPath("go")
	if err != nil {
		return environment, "", nil
	}
	goEnvironment := exec.Command(goExecutable, "env", "GOROOT")
	goEnvironment.Env = environment
	output, err := goEnvironment.Output()
	if err != nil {
		return environment, "", fmt.Errorf("read Go GOROOT: %w", err)
	}
	goRoot := strings.TrimSpace(string(output))
	target := filepath.Join(goRoot, "src", "crypto", "x509", "root_"+runtime.GOOS+".go")
	if _, err := os.Stat(target); err != nil {
		return environment, "", fmt.Errorf("find Go x509 platform roots: %w", err)
	}

	replacementPath := filepath.Join(tempDirectory, "autocurl-go-roots.go")
	replacement := `package x509

import (
	"errors"
	"os"
)

func (c *Certificate) systemVerify(opts *VerifyOptions) (chains [][]*Certificate, err error) {
	return nil, errors.New("autocurl: platform verifier disabled for process-scoped CA")
}

func loadSystemRoots() (*CertPool, error) {
	path := os.Getenv("AUTOCURL_CA_FILE")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	roots := NewCertPool()
	if !roots.AppendCertsFromPEM(data) {
		return nil, errors.New("autocurl: temporary CA file contains no certificates")
	}
	return roots, nil
}
`
	if err := os.WriteFile(replacementPath, []byte(replacement), 0o600); err != nil {
		return environment, "", fmt.Errorf("write Go trust overlay source: %w", err)
	}

	overlayPath := filepath.Join(tempDirectory, "autocurl-go-overlay.json")
	overlay := struct {
		Replace map[string]string `json:"Replace"`
	}{
		Replace: map[string]string{target: replacementPath},
	}
	overlayData, err := json.Marshal(overlay)
	if err != nil {
		return environment, "", fmt.Errorf("encode Go trust overlay: %w", err)
	}
	if err := os.WriteFile(overlayPath, overlayData, 0o600); err != nil {
		return environment, "", fmt.Errorf("write Go trust overlay: %w", err)
	}

	goFlags := strings.TrimSpace(environmentValue(environment, "GOFLAGS") + " -overlay=" + overlayPath)
	environment = mergeEnvironment(environment, map[string]string{
		"AUTOCURL_CA_FILE": caPath,
		"GOFLAGS":          goFlags,
	})
	return environment, "Go build-process CA overlay enabled for " + runtime.GOOS, nil
}
