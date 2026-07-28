package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const goDiscoveryTimeout = 3 * time.Second

// addGoTrustOverlay works around a deliberate Go standard-library limitation:
// SSL_CERT_FILE is ignored by crypto/x509 on macOS and Windows. It affects only
// Go programs built by a go command inside the wrapped process tree. Existing
// binaries still require an OS-trusted CA or application-specific TLS setup.
func addGoTrustOverlay(environment []string, tempDirectory, caPath string) ([]string, string, error) {
	return addGoTrustOverlayWithCandidates(
		environment,
		tempDirectory,
		caPath,
		defaultGoExecutableCandidates(),
	)
}

func addGoTrustOverlayWithCandidates(
	environment []string,
	tempDirectory,
	caPath string,
	candidates []string,
) ([]string, string, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return environment, "", nil
	}

	goExecutable, err := discoverGoExecutable(environment, candidates)
	if err != nil {
		return environment,
			"Go build-process CA overlay unavailable: " + err.Error() +
				"; HTTPS interception may fail for Go binaries",
			nil
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

func discoverGoExecutable(environment, candidates []string) (string, error) {
	if configured := environmentValue(environment, "AUTOCURL_GO_EXECUTABLE"); configured != "" {
		if executable, ok := existingExecutable(configured); ok {
			return executable, nil
		}
		return "", fmt.Errorf("AUTOCURL_GO_EXECUTABLE does not point to an executable file")
	}

	if goRoot := environmentValue(environment, "GOROOT"); goRoot != "" {
		if executable, ok := existingExecutable(filepath.Join(goRoot, "bin", goExecutableName())); ok {
			return executable, nil
		}
	}

	for _, directory := range filepath.SplitList(environmentPath(environment)) {
		if executable, ok := existingExecutable(filepath.Join(directory, goExecutableName())); ok {
			return executable, nil
		}
	}
	for _, candidate := range candidates {
		if executable, ok := existingExecutable(candidate); ok {
			return executable, nil
		}
	}

	if runtime.GOOS != "windows" {
		if executable, ok := goExecutableFromLoginShell(environment); ok {
			return executable, nil
		}
	}
	return "", fmt.Errorf("Go SDK was not found in GOROOT, PATH, standard locations, or the login shell")
}

func environmentPath(environment []string) string {
	if value := environmentValue(environment, "PATH"); value != "" {
		return value
	}
	return environmentValue(environment, "Path")
}

func goExecutableName() string {
	if runtime.GOOS == "windows" {
		return "go.exe"
	}
	return "go"
}

func existingExecutable(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(absolute)
	if err != nil || info.IsDir() {
		return "", false
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return "", false
	}
	return absolute, true
}

func defaultGoExecutableCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/opt/homebrew/bin/go",
			"/opt/homebrew/opt/go/libexec/bin/go",
			"/usr/local/bin/go",
			"/usr/local/go/bin/go",
		}
	case "windows":
		candidates := make([]string, 0, 2)
		if programFiles := os.Getenv("ProgramFiles"); programFiles != "" {
			candidates = append(candidates, filepath.Join(programFiles, "Go", "bin", "go.exe"))
		}
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			candidates = append(candidates,
				filepath.Join(localAppData, "Programs", "Go", "bin", "go.exe"))
		}
		return candidates
	default:
		return []string{"/usr/local/go/bin/go", "/usr/local/bin/go"}
	}
}

func goExecutableFromLoginShell(environment []string) (string, bool) {
	shell := environmentValue(environment, "SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	if _, ok := existingExecutable(shell); !ok {
		return "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), goDiscoveryTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, shell, "-lc", "command -v go")
	command.Env = environment
	output, err := command.Output()
	if err != nil {
		return "", false
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if executable, ok := existingExecutable(strings.TrimSpace(lines[index])); ok {
			return executable, true
		}
	}
	return "", false
}
