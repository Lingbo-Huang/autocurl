package capture

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Lingbo-Huang/autocurl/internal/config"
	"golang.org/x/net/http2"
)

type Event struct {
	Timestamp     time.Time
	Method        string
	URL           string
	Protocol      string
	UpstreamProto string
	Kind          string
	Headers       http.Header
	Body          []byte
	BodyTruncated bool
	Status        int
	GRPCStatus    string
	Duration      time.Duration
	Error         error
}

type Mode string

const (
	ModeSafe   Mode = "safe"
	ModeStrict Mode = "strict"
)

type Diagnostic struct {
	Timestamp       time.Time
	Code            string
	Severity        string
	Host            string
	Summary         string
	Detail          string
	SuggestedAction string
	BypassTarget    string
	AutoApplied     bool
	RetryRequired   bool
}

type TLSProbeResult struct {
	Bypass bool
	Code   string
	Detail string
}

type TLSProbe func(context.Context, string) TLSProbeResult

type Options struct {
	Authority    *Authority
	LiveHeaders  []config.Header
	MaxBody      int64
	Mode         Mode
	OnEvent      func(Event)
	OnDiagnostic func(Diagnostic)
	Transport    http.RoundTripper
	H2CTransport http.RoundTripper
	TLSProbe     TLSProbe
}

type Proxy struct {
	authority      *Authority
	liveHeaders    []config.Header
	maxBody        int64
	mode           Mode
	onEvent        func(Event)
	onDiagnostic   func(Diagnostic)
	transport      http.RoundTripper
	h2cTransport   http.RoundTripper
	tlsProbe       TLSProbe
	server         *http.Server
	listener       net.Listener
	closeOnce      sync.Once
	recording      atomic.Bool
	tlsMu          sync.Mutex
	tlsDecisions   map[string]TLSProbeResult
	diagnosticMu   sync.Mutex
	diagnosticSeen map[string]struct{}
}

func NewProxy(options Options) (*Proxy, error) {
	if options.Authority == nil {
		return nil, fmt.Errorf("authority is required")
	}
	if options.MaxBody <= 0 {
		options.MaxBody = 1024 * 1024
	}
	if options.OnEvent == nil {
		options.OnEvent = func(Event) {}
	}
	if options.Mode == "" {
		options.Mode = ModeStrict
	}
	if options.Mode != ModeSafe && options.Mode != ModeStrict {
		return nil, fmt.Errorf("unsupported capture mode %q", options.Mode)
	}
	if options.OnDiagnostic == nil {
		options.OnDiagnostic = func(Diagnostic) {}
	}
	if options.TLSProbe == nil {
		options.TLSProbe = probeTLSCompatibility
	}
	if options.Transport == nil {
		options.Transport = &http.Transport{
			Proxy:             nil,
			ForceAttemptHTTP2: true,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		}
	}
	if options.H2CTransport == nil {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		options.H2CTransport = &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(
				ctx context.Context,
				network, address string,
				_ *tls.Config,
			) (net.Conn, error) {
				return dialer.DialContext(ctx, network, address)
			},
		}
	}

	proxy := &Proxy{
		authority:      options.Authority,
		liveHeaders:    options.LiveHeaders,
		maxBody:        options.MaxBody,
		mode:           options.Mode,
		onEvent:        options.OnEvent,
		onDiagnostic:   options.OnDiagnostic,
		transport:      options.Transport,
		h2cTransport:   options.H2CTransport,
		tlsProbe:       options.TLSProbe,
		tlsDecisions:   make(map[string]TLSProbeResult),
		diagnosticSeen: make(map[string]struct{}),
	}
	proxy.recording.Store(true)
	proxy.server = &http.Server{
		Handler:           proxy,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return proxy, nil
}

func (p *Proxy) Start() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen for local proxy: %w", err)
	}
	p.listener = listener
	go func() {
		err := p.server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.emit(Event{Timestamp: time.Now(), Error: fmt.Errorf("proxy server: %w", err)})
		}
	}()
	return listener.Addr().String(), nil
}

func (p *Proxy) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		closeErr = p.server.Shutdown(ctx)
		if closer, ok := p.transport.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
		if closer, ok := p.h2cTransport.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
	})
	return closeErr
}

// SetRecording controls event retention without changing proxy forwarding.
// Operational errors without request data are still emitted while paused.
func (p *Proxy) SetRecording(recording bool) {
	p.recording.Store(recording)
}

func (p *Proxy) IsRecording() bool {
	return p.recording.Load()
}

func (p *Proxy) emit(event Event) {
	if event.Method == "" && event.URL == "" || p.recording.Load() {
		p.onEvent(event)
	}
}

func (p *Proxy) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodConnect {
		p.handleConnect(writer, request)
		return
	}
	p.handleHTTP(writer, request, "http")
}

func (p *Proxy) handleHTTP(writer http.ResponseWriter, request *http.Request, scheme string) {
	response, event, capturedBody := p.roundTrip(request, scheme)
	if response == nil {
		finalizeEvent(&event, capturedBody, nil)
		p.emit(event)
		http.Error(writer, event.Error.Error(), http.StatusBadGateway)
		return
	}

	if isHTTP1Switch(response) {
		p.handleHTTP1Switch(writer, response, event, capturedBody)
		return
	}
	defer response.Body.Close()

	responseHeaders := response.Header.Clone()
	removeHopByHopHeaders(responseHeaders)
	copyHeaders(writer.Header(), responseHeaders)
	declareTrailers(writer.Header(), response.Trailer)
	writer.WriteHeader(response.StatusCode)
	if event.Kind == "grpc" || event.Kind == "websocket" {
		_ = http.NewResponseController(writer).Flush()
	}

	var destination io.Writer = writer
	if event.Kind == "grpc" {
		destination = flushWriter{writer: writer}
	}
	_, copyErr := io.Copy(destination, response.Body)
	if copyErr != nil && event.Error == nil {
		event.Error = fmt.Errorf("copy upstream response: %w", copyErr)
	}
	copyHeaders(writer.Header(), response.Trailer)
	finalizeEvent(&event, capturedBody, response)
	p.emit(event)
}

func (p *Proxy) handleHTTP1Switch(
	writer http.ResponseWriter,
	response *http.Response,
	event Event,
	capturedBody *limitedBuffer,
) {
	upstream, ok := response.Body.(io.ReadWriteCloser)
	if !ok {
		response.Body.Close()
		event.Error = fmt.Errorf("upstream switched protocols without a writable connection")
		finalizeEvent(&event, capturedBody, response)
		p.emit(event)
		http.Error(writer, event.Error.Error(), http.StatusBadGateway)
		return
	}

	connection, buffered, err := http.NewResponseController(writer).Hijack()
	if err != nil {
		upstream.Close()
		event.Error = fmt.Errorf("hijack upgraded client connection: %w", err)
		finalizeEvent(&event, capturedBody, response)
		p.emit(event)
		return
	}
	defer connection.Close()
	defer upstream.Close()

	if err := writeSwitchingProtocols(buffered, response); err != nil {
		event.Error = fmt.Errorf("write protocol-switch response: %w", err)
		finalizeEvent(&event, capturedBody, response)
		p.emit(event)
		return
	}
	if err := buffered.Flush(); err != nil {
		event.Error = fmt.Errorf("flush protocol-switch response: %w", err)
		finalizeEvent(&event, capturedBody, response)
		p.emit(event)
		return
	}

	finalizeEvent(&event, capturedBody, response)
	p.emit(event)
	relayDuplex(connection, buffered.Reader, upstream)
}

func (p *Proxy) handleConnect(writer http.ResponseWriter, request *http.Request) {
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		http.Error(writer, "proxy does not support connection hijacking", http.StatusInternalServerError)
		return
	}
	rawConnection, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		rawConnection.Close()
		return
	}
	if err := buffered.Flush(); err != nil {
		rawConnection.Close()
		return
	}

	go p.serveConnect(rawConnection, request.Host)
}

func (p *Proxy) serveConnect(rawConnection net.Conn, host string) {
	defer rawConnection.Close()

	reader := bufio.NewReader(rawConnection)
	firstByte, err := reader.Peek(1)
	if err != nil {
		if !errors.Is(err, io.EOF) && !isClosedConnection(err) {
			p.emit(Event{Timestamp: time.Now(), Error: fmt.Errorf("inspect CONNECT stream for %s: %w", host, err)})
		}
		return
	}
	connection := &bufferedConnection{Conn: rawConnection, reader: reader}
	if firstByte[0] == http2.ClientPreface[0] {
		preface, err := reader.Peek(len(http2.ClientPreface))
		if err != nil || !bytes.Equal(preface, []byte(http2.ClientPreface)) {
			p.emit(Event{Timestamp: time.Now(), Error: fmt.Errorf("unsupported cleartext CONNECT protocol for %s", host)})
			return
		}
		p.serveHTTP2(connection, host, "http")
		return
	}
	if p.mode == ModeSafe {
		if decision := p.tlsDecision(host); decision.Bypass {
			p.tunnel(rawConnection, connection, host, decision)
			return
		}
	}
	p.serveTLS(connection, host)
}

func (p *Proxy) tlsDecision(address string) TLSProbeResult {
	p.tlsMu.Lock()
	cached, exists := p.tlsDecisions[address]
	p.tlsMu.Unlock()
	if exists {
		return cached
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	decision := p.tlsProbe(ctx, connectAddress(address))
	if decision.Bypass {
		if decision.Code == "" {
			decision.Code = "tls_interception_unsupported"
		}
		p.tlsMu.Lock()
		p.tlsDecisions[address] = decision
		p.tlsMu.Unlock()
	}
	return decision
}

func (p *Proxy) tunnel(
	downstream net.Conn,
	downstreamReader io.Reader,
	address string,
	decision TLSProbeResult,
) {
	target := connectAddress(address)
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	upstream, err := dialer.Dial("tcp", target)
	if err != nil {
		p.emitDiagnostic(Diagnostic{
			Timestamp:       time.Now(),
			Code:            "upstream_unreachable",
			Severity:        "error",
			Host:            address,
			Summary:         "Autocurl could not reach the upstream service",
			Detail:          err.Error(),
			SuggestedAction: "Check DNS, VPN, service availability, and the destination port.",
			BypassTarget:    bypassTarget(address),
		})
		return
	}
	p.emitDiagnostic(Diagnostic{
		Timestamp:       time.Now(),
		Code:            decision.Code,
		Severity:        "warning",
		Host:            address,
		Summary:         "Safe mode kept this TLS connection end-to-end",
		Detail:          decision.Detail,
		SuggestedAction: "The application keeps working, but this host is not captured. Use Strict Capture only if the destination can be intercepted safely.",
		BypassTarget:    bypassTarget(address),
		AutoApplied:     true,
	})
	relayDuplex(downstream, downstreamReader, upstream)
}

func (p *Proxy) emitDiagnostic(diagnostic Diagnostic) {
	if diagnostic.Timestamp.IsZero() {
		diagnostic.Timestamp = time.Now()
	}
	p.onDiagnostic(diagnostic)
}

func (p *Proxy) serveTLS(rawConnection net.Conn, host string) {
	certificate, err := p.authority.Certificate(host)
	if err != nil {
		p.emit(Event{Timestamp: time.Now(), Error: err})
		return
	}
	tlsConnection := tls.Server(rawConnection, &tls.Config{
		Certificates: []tls.Certificate{*certificate},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2", "http/1.1"},
	})
	if err := tlsConnection.Handshake(); err != nil {
		if looksLikeClientCertificateRequired(err) {
			autoApplied := p.mode == ModeSafe
			if autoApplied {
				p.rememberTLSBypass(host, TLSProbeResult{
					Bypass: true,
					Code:   "client_rejected_ca",
					Detail: "the client rejected the temporary capture certificate: " + err.Error(),
				})
			}
			action := "Add this host to Bypass capture and rerun the application."
			if autoApplied {
				action = "Retry the request. Safe mode will keep the next connection end-to-end; add the host to Bypass capture to preserve this across runs."
			}
			p.emitDiagnostic(Diagnostic{
				Timestamp:       time.Now(),
				Code:            "client_rejected_ca",
				Severity:        "warning",
				Host:            host,
				Summary:         "The client rejected Autocurl's temporary TLS certificate",
				Detail:          err.Error(),
				SuggestedAction: action,
				BypassTarget:    bypassTarget(host),
				AutoApplied:     autoApplied,
				RetryRequired:   true,
			})
		}
		p.emit(Event{Timestamp: time.Now(), Error: fmt.Errorf("TLS handshake for %s: %w", host, err)})
		return
	}

	if tlsConnection.ConnectionState().NegotiatedProtocol == "h2" {
		p.serveHTTP2(tlsConnection, host, "https")
		return
	}

	reader := bufio.NewReader(tlsConnection)
	for {
		request, err := http.ReadRequest(reader)
		if err != nil {
			if !errors.Is(err, io.EOF) && !isClosedConnection(err) {
				p.emit(Event{Timestamp: time.Now(), Error: fmt.Errorf("read HTTPS request for %s: %w", host, err)})
			}
			return
		}

		if request.Host == "" {
			request.Host = host
		}
		response, event, capturedBody := p.roundTrip(request, "https")
		if response == nil {
			finalizeEvent(&event, capturedBody, nil)
			response = &http.Response{
				StatusCode:    http.StatusBadGateway,
				Status:        "502 Bad Gateway",
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        make(http.Header),
				Body:          io.NopCloser(strings.NewReader(event.Error.Error())),
				ContentLength: int64(len(event.Error.Error())),
			}
			response.Header.Set("Content-Type", "text/plain; charset=utf-8")
		}

		if isHTTP1Switch(response) {
			upstream, ok := response.Body.(io.ReadWriteCloser)
			if !ok {
				response.Body.Close()
				event.Error = fmt.Errorf("upstream switched protocols without a writable connection")
				finalizeEvent(&event, capturedBody, response)
				p.emit(event)
				return
			}
			if err := writeSwitchingProtocols(tlsConnection, response); err != nil {
				upstream.Close()
				event.Error = fmt.Errorf("write HTTPS protocol-switch response: %w", err)
				finalizeEvent(&event, capturedBody, response)
				p.emit(event)
				return
			}
			finalizeEvent(&event, capturedBody, response)
			p.emit(event)
			relayDuplex(tlsConnection, reader, upstream)
			upstream.Close()
			return
		}

		downstreamResponse := *response
		downstreamResponse.Proto = request.Proto
		downstreamResponse.ProtoMajor = request.ProtoMajor
		downstreamResponse.ProtoMinor = request.ProtoMinor
		if downstreamResponse.ProtoMajor != 1 {
			downstreamResponse.Proto = "HTTP/1.1"
			downstreamResponse.ProtoMajor = 1
			downstreamResponse.ProtoMinor = 1
		}
		writeErr := downstreamResponse.Write(tlsConnection)
		response.Body.Close()
		if writeErr != nil && event.Error == nil {
			event.Error = fmt.Errorf("write HTTPS response: %w", writeErr)
		}
		finalizeEvent(&event, capturedBody, response)
		p.emit(event)
		if writeErr != nil || request.Close || response.Close {
			return
		}
	}
}

func (p *Proxy) rememberTLSBypass(address string, decision TLSProbeResult) {
	p.tlsMu.Lock()
	defer p.tlsMu.Unlock()
	p.tlsDecisions[address] = decision
}

func probeTLSCompatibility(ctx context.Context, address string) TLSProbeResult {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return TLSProbeResult{
			Code:   "upstream_unreachable",
			Detail: err.Error(),
		}
	}
	defer raw.Close()

	serverName := bypassTarget(address)
	connection := tls.Client(raw, &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: serverName,
		NextProtos: []string{"h2", "http/1.1"},
	})
	if err := connection.HandshakeContext(ctx); err != nil {
		var unknownAuthority x509.UnknownAuthorityError
		var invalidCertificate x509.CertificateInvalidError
		switch {
		case errors.As(err, &unknownAuthority), errors.As(err, &invalidCertificate):
			return TLSProbeResult{
				Bypass: true,
				Code:   "upstream_tls_untrusted",
				Detail: "the Autocurl engine cannot verify the upstream certificate: " + err.Error(),
			}
		case looksLikeClientCertificateRequired(err):
			return TLSProbeResult{
				Bypass: true,
				Code:   "mtls_detected",
				Detail: "the upstream TLS handshake requires or rejects a missing client certificate: " + err.Error(),
			}
		default:
			return TLSProbeResult{
				Bypass: true,
				Code:   "tls_interception_unsupported",
				Detail: "the upstream TLS handshake is not safely interceptable: " + err.Error(),
			}
		}
	}
	return TLSProbeResult{}
}

func looksLikeClientCertificateRequired(err error) bool {
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"certificate required",
		"bad certificate",
		"unknown certificate",
		"certificate unknown",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func connectAddress(address string) string {
	if _, _, err := net.SplitHostPort(address); err == nil {
		return address
	}
	return net.JoinHostPort(strings.Trim(address, "[]"), "443")
}

func bypassTarget(address string) string {
	host, _, err := net.SplitHostPort(connectAddress(address))
	if err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(address, "[]")
}

func (p *Proxy) serveHTTP2(connection net.Conn, host, scheme string) {
	server := &http2.Server{}
	server.ServeConn(connection, &http2.ServeConnOpts{
		Context:    context.Background(),
		BaseConfig: p.server,
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Host == "" {
				request.Host = host
			}
			p.handleHTTP(writer, request, scheme)
		}),
	})
}

func (p *Proxy) roundTrip(incoming *http.Request, scheme string) (*http.Response, Event, *limitedBuffer) {
	request := incoming.Clone(incoming.Context())
	request.RequestURI = ""
	if request.URL.Scheme == "" {
		request.URL.Scheme = scheme
	}
	if request.URL.Host == "" {
		request.URL.Host = request.Host
	}
	request.Header = incoming.Header.Clone()
	switching := isHTTP1UpgradeRequest(incoming)
	removeHopByHopHeaders(request.Header)
	if switching {
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", incoming.Header.Get("Upgrade"))
	}
	config.ApplyHeaders(request.Header, p.liveHeaders)

	capturedBody := &limitedBuffer{limit: p.maxBody}
	if incoming.Body != nil {
		request.Body = &teeReadCloser{
			Reader: io.TeeReader(incoming.Body, capturedBody),
			Closer: incoming.Body,
		}
	}

	started := time.Now()
	transport := p.transport
	if scheme == "http" && incoming.ProtoMajor == 2 {
		transport = p.h2cTransport
	}
	response, err := transport.RoundTrip(request)
	duration := time.Since(started)
	if err != nil {
		p.emitDiagnosticOnce(diagnoseTransportError(request.URL.String(), err))
	}

	status := 0
	if response != nil {
		status = response.StatusCode
	}
	event := Event{
		Timestamp: started,
		Method:    request.Method,
		URL:       request.URL.String(),
		Protocol:  incoming.Proto,
		Kind:      requestKind(incoming),
		Headers:   request.Header.Clone(),
		Status:    status,
		Duration:  duration,
		Error:     err,
	}
	if response != nil {
		event.UpstreamProto = response.Proto
	}
	return response, event, capturedBody
}

func (p *Proxy) emitDiagnosticOnce(diagnostic Diagnostic) {
	key := diagnostic.Code + "\x00" + diagnostic.Host
	p.diagnosticMu.Lock()
	_, exists := p.diagnosticSeen[key]
	if !exists {
		p.diagnosticSeen[key] = struct{}{}
	}
	p.diagnosticMu.Unlock()
	if !exists {
		p.emitDiagnostic(diagnostic)
	}
}

func diagnoseTransportError(rawURL string, err error) Diagnostic {
	host := ""
	if parsed, parseErr := url.Parse(rawURL); parseErr == nil {
		host = parsed.Hostname()
	}
	diagnostic := Diagnostic{
		Timestamp:    time.Now(),
		Severity:     "error",
		Host:         host,
		Detail:       err.Error(),
		BypassTarget: host,
	}
	message := strings.ToLower(err.Error())
	var unknownAuthority x509.UnknownAuthorityError
	var invalidCertificate x509.CertificateInvalidError
	switch {
	case looksLikeClientCertificateRequired(err):
		diagnostic.Code = "mtls_detected"
		diagnostic.Summary = "The upstream TLS service appears to require a client certificate"
		diagnostic.SuggestedAction = "Use Safe mode or add this host to Bypass capture, then retry or restart the application."
	case errors.As(err, &unknownAuthority), errors.As(err, &invalidCertificate):
		diagnostic.Code = "upstream_tls_untrusted"
		diagnostic.Summary = "The Autocurl engine could not verify the upstream certificate"
		diagnostic.SuggestedAction = "Use Safe mode to preserve the application's own trust handling, or install the upstream CA for the Autocurl engine."
	case strings.Contains(message, "connection refused"):
		diagnostic.Code = "upstream_connection_refused"
		diagnostic.Summary = "The destination rejected the connection"
		diagnostic.SuggestedAction = "Confirm the destination process is listening on the reported host and port."
	case strings.Contains(message, "no such host"),
		strings.Contains(message, "server misbehaving"),
		strings.Contains(message, "name or service not known"):
		diagnostic.Code = "dns_failed"
		diagnostic.Summary = "The destination hostname could not be resolved"
		diagnostic.SuggestedAction = "Check DNS, VPN, hosts-file, and service-discovery configuration."
	case errors.Is(err, context.DeadlineExceeded),
		strings.Contains(message, "i/o timeout"),
		strings.Contains(message, "deadline exceeded"):
		diagnostic.Code = "upstream_timeout"
		diagnostic.Summary = "The upstream connection timed out"
		diagnostic.SuggestedAction = "Check VPN, firewall, upstream health, and whether this target must bypass capture."
	default:
		diagnostic.Code = "upstream_request_failed"
		diagnostic.Summary = "The proxied upstream request failed"
		diagnostic.SuggestedAction = "Open the diagnostic detail, verify the destination, and add a bypass only when TLS interception is incompatible."
	}
	return diagnostic
}

type limitedBuffer struct {
	data      []byte
	limit     int64
	truncated bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	originalLength := len(data)
	remaining := b.limit - int64(len(b.data))
	if remaining <= 0 {
		b.truncated = true
		return originalLength, nil
	}
	if int64(len(data)) > remaining {
		b.data = append(b.data, data[:remaining]...)
		b.truncated = true
		return originalLength, nil
	}
	b.data = append(b.data, data...)
	return originalLength, nil
}

func (b *limitedBuffer) Bytes() []byte {
	return b.data
}

type teeReadCloser struct {
	io.Reader
	io.Closer
}

func copyHeaders(destination, source http.Header) {
	for name, values := range source {
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func removeHopByHopHeaders(headers http.Header) {
	teTrailers := headerHasToken(headers, "Te", "trailers")
	connectionHeaders := headers.Values("Connection")
	for _, value := range connectionHeaders {
		for name := range strings.SplitSeq(value, ",") {
			headers.Del(strings.TrimSpace(name))
		}
	}
	for _, name := range []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Proxy-Connection",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	} {
		headers.Del(name)
	}
	if teTrailers {
		headers.Set("Te", "trailers")
	}
}

func declareTrailers(headers, trailers http.Header) {
	for name := range trailers {
		headers.Add("Trailer", name)
	}
}

func finalizeEvent(event *Event, capturedBody *limitedBuffer, response *http.Response) {
	event.Body = append([]byte(nil), capturedBody.Bytes()...)
	event.BodyTruncated = capturedBody.truncated
	if response == nil {
		return
	}
	event.UpstreamProto = response.Proto
	if event.Kind == "grpc" {
		event.GRPCStatus = response.Trailer.Get("Grpc-Status")
		if event.GRPCStatus == "" {
			event.GRPCStatus = response.Header.Get("Grpc-Status")
		}
	}
}

func requestKind(request *http.Request) string {
	if isHTTP1UpgradeRequest(request) ||
		(request.Method == http.MethodConnect &&
			strings.EqualFold(request.Header.Get(":protocol"), "websocket")) {
		return "websocket"
	}
	contentType := strings.ToLower(request.Header.Get("Content-Type"))
	if contentType == "application/grpc" || strings.HasPrefix(contentType, "application/grpc+") {
		return "grpc"
	}
	return "http"
}

func isHTTP1UpgradeRequest(request *http.Request) bool {
	return request.ProtoMajor == 1 &&
		request.Header.Get("Upgrade") != "" &&
		headerHasToken(request.Header, "Connection", "upgrade")
}

func isHTTP1Switch(response *http.Response) bool {
	return response.StatusCode == http.StatusSwitchingProtocols &&
		response.Header.Get("Upgrade") != "" &&
		headerHasToken(response.Header, "Connection", "upgrade")
}

func headerHasToken(headers http.Header, name, token string) bool {
	for _, value := range headers.Values(name) {
		for part := range strings.SplitSeq(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

func writeSwitchingProtocols(writer io.Writer, response *http.Response) error {
	status := response.Status
	if status == "" {
		status = fmt.Sprintf("%d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}
	if _, err := fmt.Fprintf(writer, "HTTP/1.1 %s\r\n", status); err != nil {
		return err
	}
	if err := response.Header.Write(writer); err != nil {
		return err
	}
	_, err := io.WriteString(writer, "\r\n")
	return err
}

func relayDuplex(downstream net.Conn, downstreamReader io.Reader, upstream io.ReadWriteCloser) {
	done := make(chan struct{}, 2)
	copyStream := func(destination io.Writer, source io.Reader) {
		_, _ = io.Copy(destination, source)
		done <- struct{}{}
	}
	go copyStream(upstream, downstreamReader)
	go copyStream(downstream, upstream)
	<-done
	_ = upstream.Close()
	_ = downstream.Close()
	<-done
}

type flushWriter struct {
	writer http.ResponseWriter
}

type bufferedConnection struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConnection) Read(data []byte) (int, error) {
	return c.reader.Read(data)
}

func (f flushWriter) Write(data []byte) (int, error) {
	count, err := f.writer.Write(data)
	if flushErr := http.NewResponseController(f.writer).Flush(); err == nil && flushErr != nil {
		err = flushErr
	}
	return count, err
}

func isClosedConnection(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "use of closed network connection") ||
		strings.Contains(message, "connection reset by peer")
}
