package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Lingbo-Huang/autocurl/internal/capture"
)

type captureSession struct {
	Proxy         *capture.Proxy
	Address       string
	CAPath        string
	TempDirectory string
}

func startCaptureSession(options capture.Options) (*captureSession, error) {
	authority, err := capture.NewAuthority()
	if err != nil {
		return nil, err
	}
	tempDirectory, err := os.MkdirTemp("", "autocurl-")
	if err != nil {
		return nil, fmt.Errorf("create temporary directory: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(tempDirectory)
	}

	caPath := filepath.Join(tempDirectory, "autocurl-ca.pem")
	if err := os.WriteFile(caPath, authority.PEM(), 0o600); err != nil {
		cleanup()
		return nil, fmt.Errorf("write temporary CA: %w", err)
	}
	options.Authority = authority
	proxy, err := capture.NewProxy(options)
	if err != nil {
		cleanup()
		return nil, err
	}
	address, err := proxy.Start()
	if err != nil {
		cleanup()
		return nil, err
	}
	return &captureSession{
		Proxy:         proxy,
		Address:       address,
		CAPath:        caPath,
		TempDirectory: tempDirectory,
	}, nil
}

func (session *captureSession) ProxyURL() string {
	return "http://" + session.Address
}

func (session *captureSession) Environment(base []string) ([]string, []string, error) {
	environment, javaNote := buildChildEnvironment(
		base,
		session.Address,
		session.CAPath,
		session.TempDirectory,
	)
	notes := make([]string, 0, 2)
	if javaNote != "" {
		notes = append(notes, javaNote)
	}
	environment, goNote, goErr := addGoTrustOverlay(
		environment,
		session.TempDirectory,
		session.CAPath,
	)
	if goNote != "" {
		notes = append(notes, goNote)
	}
	return environment, notes, goErr
}

func (session *captureSession) Close() {
	if session.Proxy != nil {
		_ = session.Proxy.Close()
	}
	if session.TempDirectory != "" {
		_ = os.RemoveAll(session.TempDirectory)
	}
}
