package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Lingbo-Huang/autocurl/internal/config"
	curlrender "github.com/Lingbo-Huang/autocurl/internal/render"
	"golang.org/x/net/http/httpguts"
)

const maxRenderInput = 16 * 1024 * 1024

type renderEnvelope struct {
	Method     string          `json:"method"`
	URL        string          `json:"url"`
	Protocol   string          `json:"protocol,omitempty"`
	Headers    flexibleHeaders `json:"headers,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
	BodyBase64 string          `json:"body_base64,omitempty"`
}

type flexibleHeaders http.Header

func (headers *flexibleHeaders) UnmarshalJSON(data []byte) error {
	var encoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &encoded); err != nil {
		return fmt.Errorf("headers must be a JSON object: %w", err)
	}
	decoded := make(http.Header, len(encoded))
	for name, rawValue := range encoded {
		if !httpguts.ValidHeaderFieldName(name) {
			return fmt.Errorf("invalid header name %q", name)
		}
		name = http.CanonicalHeaderKey(name)
		var single string
		if err := json.Unmarshal(rawValue, &single); err == nil {
			if !httpguts.ValidHeaderFieldValue(single) {
				return fmt.Errorf("header %q contains an invalid value", name)
			}
			decoded.Add(name, single)
			continue
		}
		var multiple []string
		if err := json.Unmarshal(rawValue, &multiple); err != nil {
			return fmt.Errorf("header %q must be a string or string array", name)
		}
		for _, value := range multiple {
			if !httpguts.ValidHeaderFieldValue(value) {
				return fmt.Errorf("header %q contains an invalid value", name)
			}
			decoded.Add(name, value)
		}
	}
	*headers = flexibleHeaders(decoded)
	return nil
}

func renderCommand(arguments []string, globalJSON bool, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("autocurl render", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var headerValues stringList
	requestFile := flags.String("request", "", "JSON request description file; use - for stdin")
	method := flags.String("method", "", "HTTP method; defaults to GET or POST when a body is present")
	requestURL := flags.String("url", "", "absolute request URL")
	protocol := flags.String("protocol", "", "HTTP/1.0, HTTP/1.1, or HTTP/2")
	body := flags.String("body", "", "literal request body")
	bodyFile := flags.String("body-file", "", "request body file; use - for stdin")
	showSecrets := flags.Bool("show-secrets", false, "include credentials and sensitive fields (unsafe)")
	copyCurl := flags.Bool("copy", false, "copy the generated cURL to the clipboard")
	outputPath := flags.String("output", "", "write the generated cURL to this file")
	timings := flags.Bool("timings", false, "include cURL timing output")
	jsonOutput := flags.Bool("json", globalJSON, "emit a stable JSON result")
	flags.Var(&headerValues, "header", "request header in Name: value form; repeatable")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), `Usage:
  autocurl render --request request.json [options]
  autocurl render --method POST --url URL [options]

Build a safe cURL without sending a request. This is useful when a debugger
shows the prepared method, URL, headers, and body before the network call.

Options:
`)
		flags.PrintDefaults()
		fmt.Fprint(flags.Output(), `
JSON request format:
  {
    "method": "POST",
    "url": "https://api.example.com/orders",
    "protocol": "HTTP/2",
    "headers": {"Content-Type": "application/json"},
    "body": {"order_id": "demo-42"}
  }

Examples:
  autocurl render --request request.json --copy
  autocurl render --request - --copy < request.json
  autocurl render --method POST --url https://api.example.com/orders \
    --header 'Content-Type: application/json' --body-file body.json --copy
`)
	}

	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if len(flags.Args()) != 0 {
		return writeRenderError(*jsonOutput, stdout, stderr, "unexpected positional arguments")
	}

	var request curlrender.Request
	var err error
	if *requestFile != "" {
		if *requestURL != "" || *method != "" || *protocol != "" ||
			len(headerValues) > 0 || *body != "" || *bodyFile != "" {
			return writeRenderError(
				*jsonOutput, stdout, stderr,
				"--request cannot be combined with --method, --url, --protocol, --header, --body, or --body-file",
			)
		}
		request, err = requestFromEnvelopeFile(*requestFile, stdin)
	} else {
		request, err = requestFromFlags(
			*method, *requestURL, *protocol, headerValues, *body, *bodyFile, stdin,
		)
	}
	if err != nil {
		return writeRenderError(*jsonOutput, stdout, stderr, err.Error())
	}
	if err := validateRenderRequest(request); err != nil {
		return writeRenderError(*jsonOutput, stdout, stderr, err.Error())
	}
	request.ShowSecrets = *showSecrets
	request.IncludeWriteOut = *timings

	result := curlrender.Curl(request)
	copyError := ""
	if *copyCurl {
		if err := copyToClipboard(result.Command); err != nil {
			copyError = err.Error()
		}
	}
	outputError := ""
	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, []byte(result.Command+"\n"), 0o600); err != nil {
			outputError = err.Error()
		}
	}

	if *jsonOutput {
		response := struct {
			SchemaVersion string `json:"schema_version"`
			Curl          string `json:"curl"`
			Redacted      bool   `json:"redacted"`
			Warning       string `json:"warning,omitempty"`
			Copied        bool   `json:"copied,omitempty"`
			CopyError     string `json:"copy_error,omitempty"`
			OutputFile    string `json:"output_file,omitempty"`
			OutputError   string `json:"output_error,omitempty"`
		}{
			SchemaVersion: "1",
			Curl:          result.Command,
			Redacted:      result.Redacted,
			Warning:       result.Warning,
			Copied:        *copyCurl && copyError == "",
			CopyError:     copyError,
			OutputFile:    *outputPath,
			OutputError:   outputError,
		}
		_ = json.NewEncoder(stdout).Encode(response)
	} else {
		if result.Redacted {
			fmt.Fprintln(stderr, "[autocurl] sensitive values were redacted; use --show-secrets only for local private debugging")
		}
		if result.Warning != "" {
			fmt.Fprintf(stderr, "[autocurl] warning: %s\n", result.Warning)
		}
		fmt.Fprintln(stdout, result.Command)
		if *copyCurl {
			if copyError == "" {
				fmt.Fprintln(stderr, "[autocurl] copied cURL to clipboard")
			} else {
				fmt.Fprintf(stderr, "[autocurl] warning: could not copy cURL: %s\n", copyError)
			}
		}
		if *outputPath != "" {
			if outputError == "" {
				fmt.Fprintf(stderr, "[autocurl] wrote cURL to %s\n", *outputPath)
			} else {
				fmt.Fprintf(stderr, "[autocurl] warning: could not write cURL: %s\n", outputError)
			}
		}
	}
	if copyError != "" || outputError != "" {
		return 1
	}
	return 0
}

func requestFromEnvelopeFile(path string, stdin io.Reader) (curlrender.Request, error) {
	data, err := readRenderInput(path, stdin)
	if err != nil {
		return curlrender.Request{}, err
	}
	var envelope renderEnvelope
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return curlrender.Request{}, fmt.Errorf("decode request JSON: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return curlrender.Request{}, err
	}

	body, isJSON, err := decodeEnvelopeBody(envelope)
	if err != nil {
		return curlrender.Request{}, err
	}
	headers := http.Header(envelope.Headers)
	if headers == nil {
		headers = make(http.Header)
	}
	if isJSON && headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", "application/json")
	}
	method := strings.ToUpper(strings.TrimSpace(envelope.Method))
	if method == "" {
		method = defaultRenderMethod(body)
	}
	return curlrender.Request{
		Method:   method,
		URL:      strings.TrimSpace(envelope.URL),
		Protocol: normalizeProtocol(envelope.Protocol),
		Kind:     renderKind(headers, envelope.URL),
		Headers:  headers,
		Body:     body,
	}, nil
}

func requestFromFlags(
	method, requestURL, protocol string,
	headerValues []string,
	body, bodyFile string,
	stdin io.Reader,
) (curlrender.Request, error) {
	if body != "" && bodyFile != "" {
		return curlrender.Request{}, fmt.Errorf("--body and --body-file are mutually exclusive")
	}
	headers, err := parseHeaders(headerValues)
	if err != nil {
		return curlrender.Request{}, fmt.Errorf("invalid --header: %w", err)
	}
	httpHeaders := make(http.Header)
	config.ApplyHeaders(httpHeaders, headers)

	var bodyData []byte
	switch {
	case bodyFile != "":
		bodyData, err = readRenderInput(bodyFile, stdin)
	case body != "":
		bodyData = []byte(body)
	}
	if err != nil {
		return curlrender.Request{}, err
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = defaultRenderMethod(bodyData)
	}
	return curlrender.Request{
		Method:   method,
		URL:      strings.TrimSpace(requestURL),
		Protocol: normalizeProtocol(protocol),
		Kind:     renderKind(httpHeaders, requestURL),
		Headers:  httpHeaders,
		Body:     bodyData,
	}, nil
}

func decodeEnvelopeBody(envelope renderEnvelope) ([]byte, bool, error) {
	if len(envelope.Body) > 0 && envelope.BodyBase64 != "" {
		return nil, false, fmt.Errorf("body and body_base64 are mutually exclusive")
	}
	if envelope.BodyBase64 != "" {
		body, err := base64.StdEncoding.DecodeString(envelope.BodyBase64)
		if err != nil {
			return nil, false, fmt.Errorf("decode body_base64: %w", err)
		}
		return body, false, nil
	}
	if len(envelope.Body) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Body), []byte("null")) {
		return nil, false, nil
	}
	var text string
	if json.Unmarshal(envelope.Body, &text) == nil {
		return []byte(text), false, nil
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, envelope.Body); err != nil {
		return nil, false, fmt.Errorf("body must be a JSON value: %w", err)
	}
	return compact.Bytes(), true, nil
}

func readRenderInput(path string, stdin io.Reader) ([]byte, error) {
	var reader io.Reader
	var file *os.File
	if path == "-" {
		reader = stdin
	} else {
		var err error
		file, err = os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxRenderInput+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) > maxRenderInput {
		return nil, fmt.Errorf("%s exceeds the 16 MiB render input limit", path)
	}
	return data, nil
}

func validateRenderRequest(request curlrender.Request) error {
	if request.Method == "" || !httpguts.ValidHeaderFieldName(request.Method) {
		return fmt.Errorf("method must be a valid HTTP token")
	}
	parsed, err := url.Parse(request.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("--url must be an absolute URL")
	}
	switch parsed.Scheme {
	case "http", "https", "ws", "wss":
	default:
		return fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}
	switch request.Protocol {
	case "", "HTTP/1.0", "HTTP/1.1", "HTTP/2.0":
		return nil
	default:
		return fmt.Errorf("unsupported protocol %q", request.Protocol)
	}
}

func renderKind(headers http.Header, requestURL string) string {
	contentType := strings.ToLower(headers.Get("Content-Type"))
	if contentType == "application/grpc" || strings.HasPrefix(contentType, "application/grpc+") {
		return "grpc"
	}
	parsed, _ := url.Parse(requestURL)
	if parsed.Scheme == "ws" || parsed.Scheme == "wss" {
		return "websocket"
	}
	return "http"
}

func normalizeProtocol(protocol string) string {
	switch strings.ToUpper(strings.TrimSpace(protocol)) {
	case "", "AUTO":
		return ""
	case "1.0", "HTTP/1.0":
		return "HTTP/1.0"
	case "1.1", "HTTP/1.1":
		return "HTTP/1.1"
	case "2", "2.0", "H2", "HTTP/2", "HTTP/2.0":
		return "HTTP/2.0"
	default:
		return strings.TrimSpace(protocol)
	}
}

func defaultRenderMethod(body []byte) string {
	if len(body) > 0 {
		return http.MethodPost
	}
	return http.MethodGet
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("request JSON contains multiple values")
		}
		return fmt.Errorf("decode request JSON: %w", err)
	}
	return nil
}

func writeRenderError(jsonOutput bool, stdout, stderr io.Writer, message string) int {
	if jsonOutput {
		_ = json.NewEncoder(stdout).Encode(struct {
			SchemaVersion string `json:"schema_version"`
			Error         string `json:"error"`
		}{
			SchemaVersion: "1",
			Error:         message,
		})
	} else {
		fmt.Fprintf(stderr, "autocurl render: %s\n", message)
	}
	return 2
}
