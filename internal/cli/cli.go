package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Lingbo-Huang/autocurl/internal/capture"
	"github.com/Lingbo-Huang/autocurl/internal/config"
	"github.com/Lingbo-Huang/autocurl/internal/render"
)

const Version = "0.3.1"

type stringList []string

func (values *stringList) String() string {
	return strings.Join(*values, ", ")
}

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func Run(arguments []string, stdout, stderr io.Writer) int {
	jsonOutput, arguments := takeGlobalJSON(arguments)
	if len(arguments) == 0 {
		printHelp(stdout)
		return 0
	}

	switch arguments[0] {
	case "help", "--help", "-h":
		printHelp(stdout)
		return 0
	case "version", "--version":
		fmt.Fprintln(stdout, Version)
		return 0
	case "doctor":
		return doctor(jsonOutput, stdout)
	case "proxy":
		return proxyCommand(arguments[1:], jsonOutput, os.Stdin, stdout, stderr)
	case "render":
		return renderCommand(arguments[1:], jsonOutput, os.Stdin, stdout, stderr)
	case "run":
		return runCommand(arguments[1:], jsonOutput, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "autocurl: unknown command %q\n\n", arguments[0])
		printHelp(stderr)
		return 2
	}
}

func printHelp(writer io.Writer) {
	fmt.Fprint(writer, `autocurl captures outbound HTTP(S), gRPC, and WebSocket handshakes and emits safe replay cURLs.

Usage:
  autocurl [--json] <command>

Commands:
  doctor   Check runtime compatibility and available language tools
  proxy    Start an IDE-controlled capture session
  render   Build a cURL from debugger-visible request data without sending it
  run      Capture requests made by one child process
  version  Print the autocurl version

Examples:
  autocurl doctor
  autocurl --json proxy --lifetime-stdin
  autocurl run --all --match '/orders' --copy -- python3 app.py
  autocurl render --request request.json --copy
  autocurl --json run --all -- python3 app.py

Run "autocurl <command> --help" for details.
`)
}

func doctor(jsonOutput bool, stdout io.Writer) int {
	type executable struct {
		Name          string `json:"name"`
		Available     bool   `json:"available"`
		Path          string `json:"path,omitempty"`
		Version       string `json:"version,omitempty"`
		Compatibility string `json:"compatibility"`
		Note          string `json:"note,omitempty"`
	}
	names := []string{"go", "python3", "node", "java", "keytool"}
	executables := make([]executable, 0, len(names))
	for _, name := range names {
		path, err := exec.LookPath(name)
		version := ""
		compatibility := "unavailable"
		note := ""
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			version = executableVersion(ctx, name, path)
			cancel()
			compatibility = "supported"
			if name == "node" && !nodeSupportsEnvironmentProxy(version) {
				compatibility = "client-dependent"
				note = "Built-in environment proxying requires Node 22.21+ or 24.5+; older Node clients need explicit proxy configuration."
			}
			if name == "keytool" {
				compatibility = "support-tool"
				note = "Used to create the temporary Java truststore."
			}
		}
		executables = append(executables, executable{
			Name:          name,
			Available:     err == nil,
			Path:          path,
			Version:       version,
			Compatibility: compatibility,
			Note:          note,
		})
	}
	result := struct {
		SchemaVersion string       `json:"schema_version"`
		Version       string       `json:"version"`
		OS            string       `json:"os"`
		Architecture  string       `json:"architecture"`
		Executables   []executable `json:"executables"`
		Notes         []string     `json:"notes"`
	}{
		SchemaVersion: "1",
		Version:       Version,
		OS:            runtime.GOOS,
		Architecture:  runtime.GOARCH,
		Executables:   executables,
		Notes: []string{
			"Go and Python usually honor the injected proxy and CA environment variables.",
			"Java support uses proxy system properties and a temporary PKCS12 truststore when keytool is available.",
			"Node.js built-in environment proxying requires Node 22.21+ or 24.5+; custom agents and older clients may ignore it.",
			"Safe mode keeps detected mTLS, pinned/untrusted TLS, and incompatible TLS connections end-to-end.",
			"HTTP/1.1, HTTP/2, TLS/h2c gRPC, and classic ws/wss handshake proxying are enabled.",
		},
	}

	if jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(result)
		return 0
	}
	fmt.Fprintf(stdout, "autocurl %s · %s/%s\n", result.Version, result.OS, result.Architecture)
	for _, executable := range executables {
		status := "missing"
		if executable.Available {
			status = executable.Path
			if executable.Version != "" {
				status += " · " + executable.Version
			}
			status += " · " + executable.Compatibility
		}
		fmt.Fprintf(stdout, "  %-8s %s\n", executable.Name, status)
		if executable.Note != "" {
			fmt.Fprintf(stdout, "           %s\n", executable.Note)
		}
	}
	fmt.Fprintln(stdout)
	for _, note := range result.Notes {
		fmt.Fprintf(stdout, "- %s\n", note)
	}
	return 0
}

func executableVersion(ctx context.Context, name, path string) string {
	arguments := []string{"--version"}
	if name == "go" {
		arguments = []string{"version"}
	} else if name == "java" {
		arguments = []string{"-version"}
	} else if name == "keytool" {
		return ""
	}
	output, err := exec.CommandContext(ctx, path, arguments...).CombinedOutput()
	if err != nil && len(output) == 0 {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(output)), "\n")
	return strings.TrimSpace(line)
}

func nodeSupportsEnvironmentProxy(version string) bool {
	version = strings.TrimSpace(version)
	if strings.HasPrefix(version, "node ") {
		version = strings.TrimSpace(strings.TrimPrefix(version, "node "))
	}
	version = strings.TrimPrefix(version, "v")
	var major, minor int
	if count, _ := fmt.Sscanf(version, "%d.%d", &major, &minor); count != 2 {
		return false
	}
	if major == 22 {
		return minor >= 21
	}
	if major == 24 {
		return minor >= 5
	}
	return major >= 25
}

func runCommand(arguments []string, globalJSON bool, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("autocurl run", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var replayHeaderValues stringList
	var liveHeaderValues stringList
	var bypassValues stringList
	all := flags.Bool("all", false, "emit every captured request, not only failures and slow requests")
	statusMin := flags.Int("status-min", 400, "minimum HTTP status considered a failure")
	slow := flags.Duration("slow", 2*time.Second, "emit successful requests slower than this duration; 0 disables")
	maxBody := flags.Int64("max-body", 1024*1024, "maximum request-body bytes retained for cURL generation")
	showSecrets := flags.Bool("show-secrets", false, "include credentials and sensitive fields in output (unsafe)")
	jsonOutput := flags.Bool("json", globalJSON, "emit stable JSON Lines on stdout")
	verbose := flags.Bool("verbose", false, "print proxy setup details")
	match := flags.String("match", "", "emit only requests whose URL contains this text")
	method := flags.String("method", "", "emit only requests with this HTTP method")
	mode := flags.String("mode", string(capture.ModeSafe), `capture mode: "safe" bypasses incompatible TLS; "strict" forces interception`)
	copyCurl := flags.Bool("copy", false, "copy each emitted cURL to the clipboard; the latest match remains")
	outputPath := flags.String("output", "", "write the latest emitted cURL to this file")
	flags.Var(&replayHeaderValues, "replay-header", "header added only to generated cURL; repeatable")
	flags.Var(&liveHeaderValues, "live-header", "header injected into live traffic and generated cURL; repeatable")
	flags.Var(&bypassValues, "bypass", "host, IP, domain suffix, or CIDR excluded from capture; repeatable")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), `Usage:
  autocurl run [options] -- <command> [args...]

Options:
`)
		flags.PrintDefaults()
		fmt.Fprint(flags.Output(), `
Examples:
  autocurl run --all -- python3 app.py
  autocurl run --all --match '/orders' --copy -- python3 app.py
  autocurl run --all --method POST --output request.sh -- java App.java
  autocurl run --replay-header 'x-debug-info: true' -- go run .
  autocurl run --live-header 'x-debug: 1' --status-min 500 -- ./service
`)
	}

	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	commandArguments := flags.Args()
	if len(commandArguments) == 0 {
		fmt.Fprintln(stderr, "autocurl run: missing command after --")
		flags.Usage()
		return 2
	}
	if *statusMin < 100 || *statusMin > 599 {
		fmt.Fprintln(stderr, "autocurl run: --status-min must be between 100 and 599")
		return 2
	}
	if *maxBody < 1 {
		fmt.Fprintln(stderr, "autocurl run: --max-body must be greater than zero")
		return 2
	}
	if err := validateBypassTargets(bypassValues); err != nil {
		fmt.Fprintf(stderr, "autocurl run: invalid --bypass: %v\n", err)
		return 2
	}
	captureMode, err := parseCaptureMode(*mode)
	if err != nil {
		fmt.Fprintf(stderr, "autocurl run: %v\n", err)
		return 2
	}

	replayHeaders, err := parseHeaders(replayHeaderValues)
	if err != nil {
		fmt.Fprintf(stderr, "autocurl run: invalid --replay-header: %v\n", err)
		return 2
	}
	liveHeaders, err := parseHeaders(liveHeaderValues)
	if err != nil {
		fmt.Fprintf(stderr, "autocurl run: invalid --live-header: %v\n", err)
		return 2
	}

	eventWriter := stderr
	if *jsonOutput {
		eventWriter = stdout
	}
	emitter := newEmitter(emitterOptions{
		Writer:           eventWriter,
		JSON:             *jsonOutput,
		All:              *all,
		StatusMin:        *statusMin,
		Slow:             *slow,
		ReplayHeaders:    replayHeaders,
		ShowSecrets:      *showSecrets,
		Match:            *match,
		Method:           strings.ToUpper(strings.TrimSpace(*method)),
		Copy:             *copyCurl,
		OutputPath:       *outputPath,
		DiagnosticWriter: stderr,
	})

	session, err := startCaptureSession(capture.Options{
		LiveHeaders:  liveHeaders,
		MaxBody:      *maxBody,
		Mode:         captureMode,
		OnEvent:      emitter.Emit,
		OnDiagnostic: emitter.EmitDiagnostic,
	})
	if err != nil {
		fmt.Fprintf(stderr, "autocurl run: %v\n", err)
		return 1
	}
	defer session.Close()

	childEnvironment, environmentNotes, environmentErr := session.Environment(os.Environ(), bypassValues)
	if *verbose {
		fmt.Fprintf(stderr, "[autocurl] proxy %s\n", session.ProxyURL())
		fmt.Fprintf(stderr, "[autocurl] ephemeral CA %s\n", session.CAPath)
		for _, note := range environmentNotes {
			fmt.Fprintf(stderr, "[autocurl] %s\n", note)
		}
	}
	if environmentErr != nil && *verbose {
		fmt.Fprintf(stderr, "[autocurl] environment compatibility warning: %v\n", environmentErr)
	}

	command := exec.Command(commandArguments[0], commandArguments[1:]...)
	command.Env = childEnvironment
	command.Stdin = os.Stdin
	if *jsonOutput {
		command.Stdout = stderr
	} else {
		command.Stdout = stdout
	}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		fmt.Fprintf(stderr, "autocurl run: start %q: %v\n", commandArguments[0], err)
		return 1
	}

	signals := make(chan os.Signal, 2)
	signal.Notify(signals, forwardedSignals()...)
	defer signal.Stop(signals)
	done := make(chan struct{})
	go func() {
		select {
		case received := <-signals:
			_ = command.Process.Signal(received)
		case <-done:
		}
	}()

	waitErr := command.Wait()
	close(done)
	if waitErr == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(waitErr, &exitError) {
		return exitError.ExitCode()
	}
	fmt.Fprintf(stderr, "autocurl run: wait for child: %v\n", waitErr)
	return 1
}

type emitterOptions struct {
	Writer           io.Writer
	DiagnosticWriter io.Writer
	JSON             bool
	All              bool
	StatusMin        int
	Slow             time.Duration
	ReplayHeaders    []config.Header
	ShowSecrets      bool
	Match            string
	Method           string
	Copy             bool
	OutputPath       string
	CopyText         func(string) error
}

type emitter struct {
	options emitterOptions
	encoder *json.Encoder
	mu      sync.Mutex
	nextID  int64
}

func newEmitter(options emitterOptions) *emitter {
	if options.DiagnosticWriter == nil {
		options.DiagnosticWriter = options.Writer
	}
	if options.CopyText == nil {
		options.CopyText = copyToClipboard
	}
	instance := &emitter{options: options}
	if options.JSON {
		instance.encoder = json.NewEncoder(options.Writer)
	}
	return instance
}

func (e *emitter) Emit(event capture.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if event.Method == "" && event.URL == "" {
		if event.Error != nil {
			if e.options.JSON {
				_ = e.encoder.Encode(struct {
					SchemaVersion string `json:"schema_version"`
					Type          string `json:"type"`
					Error         string `json:"error"`
				}{
					SchemaVersion: "1",
					Type:          "error",
					Error:         event.Error.Error(),
				})
			} else {
				fmt.Fprintf(e.options.Writer, "[autocurl] %v\n", event.Error)
			}
		}
		return
	}
	if !e.shouldEmit(event) {
		return
	}
	kind := fallback(event.Kind, "http")
	e.nextID++
	id := e.nextID

	curl := render.Curl(render.Request{
		Method:          event.Method,
		URL:             event.URL,
		Protocol:        event.Protocol,
		Kind:            kind,
		Headers:         event.Headers,
		Body:            event.Body,
		BodyTruncated:   event.BodyTruncated,
		ReplayHeaders:   e.options.ReplayHeaders,
		ShowSecrets:     e.options.ShowSecrets,
		IncludeWriteOut: true,
	})
	safeError := sanitizeEventError(event.Error, event.URL, curl.DisplayURL)
	copyError := ""
	if e.options.Copy {
		if err := e.options.CopyText(curl.Command); err != nil {
			copyError = err.Error()
		}
	}
	outputError := ""
	if e.options.OutputPath != "" {
		if err := os.WriteFile(e.options.OutputPath, []byte(curl.Command+"\n"), 0o600); err != nil {
			outputError = err.Error()
		}
	}

	if e.options.JSON {
		result := struct {
			SchemaVersion string `json:"schema_version"`
			Type          string `json:"type"`
			ID            int64  `json:"id"`
			Timestamp     string `json:"timestamp"`
			Method        string `json:"method"`
			URL           string `json:"url"`
			Protocol      string `json:"protocol,omitempty"`
			UpstreamProto string `json:"upstream_protocol,omitempty"`
			Kind          string `json:"kind"`
			Status        int    `json:"status,omitempty"`
			GRPCStatus    string `json:"grpc_status,omitempty"`
			DurationMS    int64  `json:"duration_ms"`
			Error         string `json:"error,omitempty"`
			Curl          string `json:"curl"`
			Redacted      bool   `json:"redacted"`
			BodyTruncated bool   `json:"body_truncated"`
			Warning       string `json:"warning,omitempty"`
			Copied        bool   `json:"copied,omitempty"`
			CopyError     string `json:"copy_error,omitempty"`
			OutputFile    string `json:"output_file,omitempty"`
			OutputError   string `json:"output_error,omitempty"`
		}{
			SchemaVersion: "1",
			Type:          "request",
			ID:            id,
			Timestamp:     event.Timestamp.UTC().Format(time.RFC3339Nano),
			Method:        event.Method,
			URL:           curl.DisplayURL,
			Protocol:      event.Protocol,
			UpstreamProto: event.UpstreamProto,
			Kind:          kind,
			Status:        event.Status,
			GRPCStatus:    event.GRPCStatus,
			DurationMS:    event.Duration.Milliseconds(),
			Curl:          curl.Command,
			Redacted:      curl.Redacted,
			BodyTruncated: event.BodyTruncated,
			Warning:       curl.Warning,
			Copied:        e.options.Copy && copyError == "",
			CopyError:     copyError,
			OutputFile:    e.options.OutputPath,
			OutputError:   outputError,
		}
		if safeError != nil {
			result.Error = safeError.Error()
		}
		_ = e.encoder.Encode(result)
		return
	}

	fmt.Fprintf(e.options.Writer, "\n[autocurl #%d] %s\n", id,
		render.Summary(event.Method, curl.DisplayURL, event.Status, event.Duration.Round(time.Millisecond).String(), safeError),
	)
	fmt.Fprintf(e.options.Writer, "[autocurl #%d] %s · client %s · upstream %s\n", id,
		kind, fallback(event.Protocol, "unknown"), fallback(event.UpstreamProto, "unknown"))
	if kind == "grpc" && event.GRPCStatus != "" {
		fmt.Fprintf(e.options.Writer, "[autocurl #%d] grpc-status: %s\n", id, event.GRPCStatus)
	}
	if curl.Redacted {
		fmt.Fprintf(e.options.Writer, "[autocurl #%d] sensitive values were redacted; use --show-secrets only for local private debugging\n", id)
	}
	if curl.Warning != "" {
		fmt.Fprintf(e.options.Writer, "[autocurl #%d] warning: %s\n", id, curl.Warning)
	}
	fmt.Fprintf(e.options.Writer, "%s\n", curl.Command)
	if e.options.Copy {
		if copyError == "" {
			fmt.Fprintf(e.options.DiagnosticWriter, "[autocurl #%d] copied cURL to clipboard\n", id)
		} else {
			fmt.Fprintf(e.options.DiagnosticWriter, "[autocurl #%d] warning: could not copy cURL: %s\n", id, copyError)
		}
	}
	if e.options.OutputPath != "" {
		if outputError == "" {
			fmt.Fprintf(e.options.DiagnosticWriter, "[autocurl #%d] wrote cURL to %s\n", id, e.options.OutputPath)
		} else {
			fmt.Fprintf(e.options.DiagnosticWriter, "[autocurl #%d] warning: could not write cURL: %s\n", id, outputError)
		}
	}
}

func (e *emitter) EmitState(recording bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.options.JSON {
		_ = e.encoder.Encode(struct {
			SchemaVersion string `json:"schema_version"`
			Type          string `json:"type"`
			Recording     bool   `json:"recording"`
		}{
			SchemaVersion: "1",
			Type:          "state",
			Recording:     recording,
		})
		return
	}
	state := "paused"
	if recording {
		state = "recording"
	}
	fmt.Fprintf(e.options.Writer, "[autocurl] capture is now %s\n", state)
}

func (e *emitter) EmitDiagnostic(diagnostic capture.Diagnostic) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.options.JSON {
		_ = e.encoder.Encode(struct {
			SchemaVersion   string `json:"schema_version"`
			Type            string `json:"type"`
			Timestamp       string `json:"timestamp"`
			Code            string `json:"code"`
			Severity        string `json:"severity"`
			Host            string `json:"host,omitempty"`
			Summary         string `json:"summary"`
			Detail          string `json:"detail,omitempty"`
			SuggestedAction string `json:"suggested_action,omitempty"`
			BypassTarget    string `json:"bypass_target,omitempty"`
			AutoApplied     bool   `json:"auto_applied,omitempty"`
			RetryRequired   bool   `json:"retry_required,omitempty"`
		}{
			SchemaVersion:   "1",
			Type:            "diagnostic",
			Timestamp:       diagnostic.Timestamp.UTC().Format(time.RFC3339Nano),
			Code:            diagnostic.Code,
			Severity:        diagnostic.Severity,
			Host:            diagnostic.Host,
			Summary:         diagnostic.Summary,
			Detail:          diagnostic.Detail,
			SuggestedAction: diagnostic.SuggestedAction,
			BypassTarget:    diagnostic.BypassTarget,
			AutoApplied:     diagnostic.AutoApplied,
			RetryRequired:   diagnostic.RetryRequired,
		})
		return
	}
	fmt.Fprintf(e.options.DiagnosticWriter, "[autocurl] %s: %s\n",
		fallback(diagnostic.Severity, "info"), diagnostic.Summary)
	if diagnostic.Detail != "" {
		fmt.Fprintf(e.options.DiagnosticWriter, "[autocurl] %s\n", diagnostic.Detail)
	}
	if diagnostic.SuggestedAction != "" {
		fmt.Fprintf(e.options.DiagnosticWriter, "[autocurl] action: %s\n", diagnostic.SuggestedAction)
	}
}

func (e *emitter) shouldEmit(event capture.Event) bool {
	if e.options.Match != "" && !strings.Contains(event.URL, e.options.Match) {
		return false
	}
	if e.options.Method != "" && !strings.EqualFold(event.Method, e.options.Method) {
		return false
	}
	if e.options.All || event.Error != nil || event.Status >= e.options.StatusMin ||
		(event.Kind == "grpc" && event.GRPCStatus != "" && event.GRPCStatus != "0") {
		return true
	}
	return e.options.Slow > 0 && event.Duration >= e.options.Slow
}

func sanitizeEventError(eventError error, rawURL, safeURL string) error {
	if eventError == nil {
		return nil
	}
	message := eventError.Error()
	if rawURL != "" && rawURL != safeURL {
		message = strings.ReplaceAll(message, rawURL, safeURL)
	}
	return errors.New(message)
}

func buildChildEnvironment(
	base []string,
	address, caPath, tempDirectory string,
	bypassTargets []string,
) ([]string, string) {
	proxyURL := "http://" + address
	host, port, _ := strings.Cut(address, ":")
	overrides := map[string]string{
		"ALL_PROXY":                        proxyURL,
		"CURL_CA_BUNDLE":                   caPath,
		"GRPC_DEFAULT_SSL_ROOTS_FILE_PATH": caPath,
		"grpc_proxy":                       proxyURL,
		"HTTPS_PROXY":                      proxyURL,
		"HTTP_PROXY":                       proxyURL,
		"NODE_EXTRA_CA_CERTS":              caPath,
		"NODE_USE_ENV_PROXY":               "1",
		"PIP_CERT":                         caPath,
		"REQUESTS_CA_BUNDLE":               caPath,
		"SSL_CERT_FILE":                    caPath,
		"all_proxy":                        proxyURL,
		"https_proxy":                      proxyURL,
		"http_proxy":                       proxyURL,
	}
	bypass := mergeBypassTargets(base, bypassTargets)
	if bypass != "" {
		overrides["NO_PROXY"] = bypass
		overrides["no_proxy"] = bypass
		overrides["no_grpc_proxy"] = bypass
	}

	note := ""
	if truststorePath, err := createJavaTruststore(caPath, tempDirectory); err == nil {
		javaOptions := environmentValue(base, "JAVA_TOOL_OPTIONS") + " " +
			"-Dhttp.proxyHost=" + host + " " +
			"-Dhttp.proxyPort=" + port + " " +
			"-Dhttps.proxyHost=" + host + " " +
			"-Dhttps.proxyPort=" + port + " "
		if bypass != "" {
			javaOptions += "-Dhttp.nonProxyHosts=" + javaNonProxyHosts(bypass) + " "
		}
		javaOptions = strings.TrimSpace(javaOptions +
			"-Djavax.net.ssl.trustStore=" + truststorePath + " " +
			"-Djavax.net.ssl.trustStoreType=PKCS12 " +
			"-Djavax.net.ssl.trustStorePassword=changeit")
		overrides["JAVA_TOOL_OPTIONS"] = javaOptions
		note = "Java proxy properties and temporary truststore enabled"
	} else {
		note = "Java truststore unavailable: " + err.Error()
	}
	return mergeEnvironment(base, overrides), note
}

func mergeBypassTargets(base, configured []string) string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	add := func(value string) {
		for _, target := range strings.Split(value, ",") {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}
			if _, exists := seen[target]; exists {
				continue
			}
			seen[target] = struct{}{}
			result = append(result, target)
		}
	}
	for _, name := range []string{"NO_PROXY", "no_proxy", "no_grpc_proxy"} {
		add(environmentValue(base, name))
	}
	for _, value := range configured {
		add(value)
	}
	return strings.Join(result, ",")
}

func javaNonProxyHosts(bypass string) string {
	targets := strings.Split(bypass, ",")
	result := make([]string, 0, len(targets))
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if prefix, err := netip.ParsePrefix(target); err == nil {
			patterns := javaCIDRPatterns(prefix)
			if len(patterns) > 0 {
				result = append(result, patterns...)
			}
			continue
		}
		if strings.HasPrefix(target, ".") {
			target = "*" + target
		}
		result = append(result, target)
	}
	return strings.Join(result, "|")
}

func javaCIDRPatterns(prefix netip.Prefix) []string {
	prefix = prefix.Masked()
	if !prefix.Addr().Is4() {
		if prefix.Bits() == prefix.Addr().BitLen() {
			return []string{prefix.Addr().String()}
		}
		return nil
	}
	address := prefix.Addr().As4()
	fullOctets := prefix.Bits() / 8
	partialBits := prefix.Bits() % 8
	fixed := make([]string, 0, 4)
	for position := 0; position < fullOctets; position++ {
		fixed = append(fixed, strconv.Itoa(int(address[position])))
	}
	if partialBits == 0 {
		if fullOctets < len(address) {
			fixed = append(fixed, "*")
		}
		return []string{strings.Join(fixed, ".")}
	}
	count := 1 << (8 - partialBits)
	result := make([]string, 0, count)
	for value := int(address[fullOctets]); value < int(address[fullOctets])+count; value++ {
		parts := append([]string(nil), fixed...)
		parts = append(parts, strconv.Itoa(value))
		if fullOctets+1 < len(address) {
			parts = append(parts, "*")
		}
		result = append(result, strings.Join(parts, "."))
	}
	return result
}

func validateBypassTargets(values []string) error {
	for _, value := range values {
		for _, target := range strings.Split(value, ",") {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}
			if strings.Contains(target, "://") {
				return fmt.Errorf("%q is a URL; use only its host or IP", target)
			}
			if strings.ContainsAny(target, " \t\r\n") {
				return fmt.Errorf("%q contains whitespace", target)
			}
			if _, err := netip.ParsePrefix(target); err == nil {
				continue
			}
			if _, err := netip.ParseAddr(strings.Trim(target, "[]")); err == nil {
				continue
			}
			host := target
			if splitHost, port, err := net.SplitHostPort(target); err == nil {
				host = strings.Trim(splitHost, "[]")
				number, parseErr := strconv.Atoi(port)
				if parseErr != nil || number < 1 || number > 65535 {
					return fmt.Errorf("%q has an invalid port", target)
				}
			}
			host = strings.TrimPrefix(host, "*.")
			host = strings.TrimPrefix(host, ".")
			if host == "" || strings.Contains(host, "/") {
				return fmt.Errorf("%q is not a host, IP, domain suffix, or CIDR", target)
			}
			for _, character := range host {
				if character >= 'a' && character <= 'z' ||
					character >= 'A' && character <= 'Z' ||
					character >= '0' && character <= '9' ||
					strings.ContainsRune("-._*", character) {
					continue
				}
				return fmt.Errorf("%q contains unsupported host characters", target)
			}
		}
	}
	return nil
}

func bypassCompatibilityNotes(bypass string) []string {
	var cidrs []string
	var ipv6CIDRs []string
	for _, target := range strings.Split(bypass, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(target))
		if err != nil {
			continue
		}
		cidrs = append(cidrs, prefix.String())
		if prefix.Addr().Is6() && prefix.Bits() < prefix.Addr().BitLen() {
			ipv6CIDRs = append(ipv6CIDRs, prefix.String())
		}
	}
	notes := make([]string, 0, 2)
	if len(cidrs) > 0 {
		notes = append(notes,
			"CIDR bypass targets "+strings.Join(cidrs, ", ")+
				" are supported by Go and expanded for Java IPv4; Node.js and some Python clients may require exact hosts or IPs.")
	}
	if len(ipv6CIDRs) > 0 {
		notes = append(notes,
			"Java http.nonProxyHosts cannot represent IPv6 CIDR targets "+
				strings.Join(ipv6CIDRs, ", ")+"; configure exact IPv6 hosts for Java.")
	}
	return notes
}

func fallback(value, alternative string) string {
	if value == "" {
		return alternative
	}
	return value
}

func parseCaptureMode(value string) (capture.Mode, error) {
	mode := capture.Mode(strings.ToLower(strings.TrimSpace(value)))
	if mode != capture.ModeSafe && mode != capture.ModeStrict {
		return "", fmt.Errorf(`--mode must be "safe" or "strict"`)
	}
	return mode, nil
}

func createJavaTruststore(caPath, tempDirectory string) (string, error) {
	keytool, err := exec.LookPath("keytool")
	if err != nil {
		return "", fmt.Errorf("keytool not found")
	}
	path := filepath.Join(tempDirectory, "autocurl-truststore.p12")
	command := exec.Command(
		keytool,
		"-importcert",
		"-noprompt",
		"-alias", "autocurl",
		"-file", caPath,
		"-keystore", path,
		"-storetype", "PKCS12",
		"-storepass", "changeit",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("keytool: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return path, nil
}

func mergeEnvironment(base []string, overrides map[string]string) []string {
	merged := make(map[string]string, len(base)+len(overrides))
	for _, item := range base {
		name, value, ok := strings.Cut(item, "=")
		if ok {
			merged[name] = value
		}
	}
	for name, value := range overrides {
		merged[name] = value
	}
	result := make([]string, 0, len(merged))
	for name, value := range merged {
		result = append(result, name+"="+value)
	}
	return result
}

func environmentValue(environment []string, wanted string) string {
	for _, item := range environment {
		name, value, ok := strings.Cut(item, "=")
		if ok && name == wanted {
			return value
		}
	}
	return ""
}

func parseHeaders(values []string) ([]config.Header, error) {
	headers := make([]config.Header, 0, len(values))
	for _, value := range values {
		header, err := config.ParseHeader(value)
		if err != nil {
			return nil, err
		}
		headers = append(headers, header)
	}
	return headers, nil
}

func takeGlobalJSON(arguments []string) (bool, []string) {
	if len(arguments) > 0 && arguments[0] == "--json" {
		return true, arguments[1:]
	}
	return false, arguments
}
