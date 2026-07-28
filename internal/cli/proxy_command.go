package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Lingbo-Huang/autocurl/internal/capture"
)

type proxyReadyEvent struct {
	SchemaVersion     string            `json:"schema_version"`
	Type              string            `json:"type"`
	Version           string            `json:"version"`
	ProxyURL          string            `json:"proxy_url"`
	CAFile            string            `json:"ca_file"`
	Environment       map[string]string `json:"environment"`
	AppendEnvironment []string          `json:"append_environment,omitempty"`
	Notes             []string          `json:"notes,omitempty"`
}

type proxyControl struct {
	Command string `json:"command"`
}

func proxyCommand(
	arguments []string,
	globalJSON bool,
	stdin io.Reader,
	stdout, stderr io.Writer,
) int {
	flags := flag.NewFlagSet("autocurl proxy", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var replayHeaderValues stringList
	var liveHeaderValues stringList
	var bypassValues stringList
	all := flags.Bool("all", true, "emit every captured request")
	statusMin := flags.Int("status-min", 400, "minimum HTTP status considered a failure")
	slow := flags.Duration("slow", 2*time.Second, "emit successful requests slower than this duration; 0 disables")
	maxBody := flags.Int64("max-body", 1024*1024, "maximum request-body bytes retained for cURL generation")
	showSecrets := flags.Bool("show-secrets", false, "include credentials and sensitive fields in output (unsafe)")
	jsonOutput := flags.Bool("json", globalJSON, "emit stable JSON Lines on stdout")
	match := flags.String("match", "", "emit only requests whose URL contains this text")
	method := flags.String("method", "", "emit only requests with this HTTP method")
	lifetimeStdin := flags.Bool("lifetime-stdin", false, "stop when stdin closes; intended for IDE integrations")
	flags.Var(&replayHeaderValues, "replay-header", "header added only to generated cURL; repeatable")
	flags.Var(&liveHeaderValues, "live-header", "header injected into live traffic and generated cURL; repeatable")
	flags.Var(&bypassValues, "bypass", "host, IP, domain suffix, or CIDR excluded from capture; repeatable")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), `Usage:
  autocurl [--json] proxy [options]

Starts a process-scoped capture backend for IDE integrations. In JSON mode the
first stdout line is a "ready" event containing only the environment overrides
that the IDE should merge into its debug/run configuration. Later lines are
"request" events containing replayable cURLs.

Options:
`)
		flags.PrintDefaults()
		fmt.Fprint(flags.Output(), `
Examples:
  autocurl --json proxy --lifetime-stdin
  autocurl --json proxy --match '/orders' --replay-header 'x-debug-info: true'
`)
	}
	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "autocurl proxy: unexpected positional arguments")
		flags.Usage()
		return 2
	}
	if *statusMin < 100 || *statusMin > 599 {
		fmt.Fprintln(stderr, "autocurl proxy: --status-min must be between 100 and 599")
		return 2
	}
	if *maxBody < 1 {
		fmt.Fprintln(stderr, "autocurl proxy: --max-body must be greater than zero")
		return 2
	}
	replayHeaders, err := parseHeaders(replayHeaderValues)
	if err != nil {
		fmt.Fprintf(stderr, "autocurl proxy: invalid --replay-header: %v\n", err)
		return 2
	}
	liveHeaders, err := parseHeaders(liveHeaderValues)
	if err != nil {
		fmt.Fprintf(stderr, "autocurl proxy: invalid --live-header: %v\n", err)
		return 2
	}

	eventWriter := stdout
	emitter := newEmitter(emitterOptions{
		Writer:           eventWriter,
		DiagnosticWriter: stderr,
		JSON:             *jsonOutput,
		All:              *all,
		StatusMin:        *statusMin,
		Slow:             *slow,
		ReplayHeaders:    replayHeaders,
		ShowSecrets:      *showSecrets,
		Match:            *match,
		Method:           strings.ToUpper(strings.TrimSpace(*method)),
	})
	session, err := startCaptureSession(capture.Options{
		LiveHeaders: liveHeaders,
		MaxBody:     *maxBody,
		OnEvent:     emitter.Emit,
	})
	if err != nil {
		fmt.Fprintf(stderr, "autocurl proxy: %v\n", err)
		return 1
	}
	defer session.Close()

	environment, notes, environmentErr := session.Environment(nil, bypassValues)
	if environmentErr != nil {
		notes = append(notes, "Environment compatibility warning: "+environmentErr.Error())
	}
	ready := proxyReadyEvent{
		SchemaVersion:     "1",
		Type:              "ready",
		Version:           Version,
		ProxyURL:          session.ProxyURL(),
		CAFile:            session.CAPath,
		Environment:       environmentMap(environment),
		AppendEnvironment: []string{"GOFLAGS", "JAVA_TOOL_OPTIONS"},
		Notes:             notes,
	}
	if *jsonOutput {
		if err := json.NewEncoder(stdout).Encode(ready); err != nil {
			fmt.Fprintf(stderr, "autocurl proxy: write ready event: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintf(stdout, "[autocurl] capture session ready at %s\n", ready.ProxyURL)
		fmt.Fprintln(stdout, "[autocurl] keep this process running; press Ctrl-C to stop")
	}

	signals := make(chan os.Signal, 2)
	signal.Notify(signals, forwardedSignals()...)
	defer signal.Stop(signals)
	if !*lifetimeStdin {
		<-signals
		return 0
	}

	stdinClosed := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			value := strings.TrimSpace(scanner.Text())
			if value == "" {
				continue
			}
			control := proxyControl{Command: value}
			if strings.HasPrefix(value, "{") {
				if err := json.Unmarshal([]byte(value), &control); err != nil {
					fmt.Fprintf(stderr, "autocurl proxy: ignored invalid control message: %v\n", err)
					continue
				}
			}
			switch strings.ToLower(strings.TrimSpace(control.Command)) {
			case "pause":
				session.Proxy.SetRecording(false)
				emitter.EmitState(false)
			case "resume":
				session.Proxy.SetRecording(true)
				emitter.EmitState(true)
			default:
				fmt.Fprintf(stderr, "autocurl proxy: ignored unknown control command %q\n", control.Command)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(stderr, "autocurl proxy: read control input: %v\n", err)
		}
		close(stdinClosed)
	}()
	select {
	case <-signals:
	case <-stdinClosed:
	}
	return 0
}

func environmentMap(environment []string) map[string]string {
	result := make(map[string]string, len(environment))
	for _, item := range environment {
		name, value, ok := strings.Cut(item, "=")
		if ok {
			result[name] = value
		}
	}
	return result
}
