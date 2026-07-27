package capture

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Lingbo-Huang/autocurl/internal/config"
	"golang.org/x/net/http2"
)

func TestProxyCapturesHTTPAndInjectsLiveHeader(t *testing.T) {
	requestReceived := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("X-Debug"); got != "enabled" {
			t.Errorf("upstream X-Debug = %q, want enabled", got)
		}
		body, _ := io.ReadAll(request.Body)
		if got := string(body); got != `{"hello":"world"}` {
			t.Errorf("upstream body = %q", got)
		}
		requestReceived <- struct{}{}
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte("gateway failed"))
	}))
	defer upstream.Close()

	authority, _ := NewAuthority()
	events := make(chan Event, 2)
	proxy, err := NewProxy(Options{
		Authority: authority,
		LiveHeaders: []config.Header{
			{Name: "X-Debug", Value: "enabled"},
		},
		OnEvent: func(event Event) { events <- event },
	})
	if err != nil {
		t.Fatalf("NewProxy returned an error: %v", err)
	}
	address, err := proxy.Start()
	if err != nil {
		t.Fatalf("Start returned an error: %v", err)
	}
	defer proxy.Close()

	client := proxyClient(t, address, nil)
	request, _ := http.NewRequest(http.MethodPost, upstream.URL+"/orders", strings.NewReader(`{"hello":"world"}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("proxied request returned an error: %v", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	response.Body.Close()

	select {
	case <-requestReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream did not receive request")
	}
	event := waitForRequestEvent(t, events)
	if event.Status != http.StatusBadGateway {
		t.Fatalf("event status = %d, want %d", event.Status, http.StatusBadGateway)
	}
	if got := string(event.Body); got != `{"hello":"world"}` {
		t.Fatalf("event body = %q", got)
	}
	if got := event.Headers.Get("X-Debug"); got != "enabled" {
		t.Fatalf("captured X-Debug = %q, want enabled", got)
	}
}

func TestProxyCapturesHTTPS(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte("created"))
	}))
	defer upstream.Close()

	authority, _ := NewAuthority()
	events := make(chan Event, 2)
	proxy, err := NewProxy(Options{
		Authority: authority,
		OnEvent:   func(event Event) { events <- event },
		Transport: &http.Transport{
			ForceAttemptHTTP2: false,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // Test-only upstream certificate.
		},
	})
	if err != nil {
		t.Fatalf("NewProxy returned an error: %v", err)
	}
	address, err := proxy.Start()
	if err != nil {
		t.Fatalf("Start returned an error: %v", err)
	}
	defer proxy.Close()

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(authority.PEM()) {
		t.Fatal("could not trust authority PEM")
	}
	client := proxyClient(t, address, roots)
	response, err := client.Get(upstream.URL + "/secure")
	if err != nil {
		t.Fatalf("proxied HTTPS request returned an error: %v", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	response.Body.Close()

	event := waitForRequestEvent(t, events)
	if event.Status != http.StatusCreated {
		t.Fatalf("event status = %d, want %d (error: %v)", event.Status, http.StatusCreated, event.Error)
	}
	if !strings.HasPrefix(event.URL, "https://") {
		t.Fatalf("event URL = %q, want HTTPS", event.URL)
	}
}

func TestProxyAcceptsHTTP10AndRecordsUpstreamVersion(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	authority, _ := NewAuthority()
	events := make(chan Event, 2)
	proxy, _ := NewProxy(Options{
		Authority: authority,
		OnEvent:   func(event Event) { events <- event },
	})
	address, _ := proxy.Start()
	defer proxy.Close()

	connection, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer connection.Close()
	parsedUpstream, _ := url.Parse(upstream.URL)
	_, _ = fmt.Fprintf(connection,
		"GET %s/legacy HTTP/1.0\r\nHost: %s\r\n\r\n",
		upstream.URL, parsedUpstream.Host,
	)
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatalf("read HTTP/1.0 proxy response: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("HTTP/1.0 proxy status = %d, want 204", response.StatusCode)
	}

	event := waitForRequestEvent(t, events)
	if event.Protocol != "HTTP/1.0" || event.UpstreamProto != "HTTP/1.1" {
		t.Fatalf("captured HTTP/1.0 versions = %q -> %q, want HTTP/1.0 -> HTTP/1.1",
			event.Protocol, event.UpstreamProto)
	}
}

func TestProxyNegotiatesHTTP2OnBothSides(t *testing.T) {
	upstreamProtocol := make(chan string, 1)
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamProtocol <- request.Proto
		writer.WriteHeader(http.StatusNoContent)
	}))
	upstream.EnableHTTP2 = true
	upstream.StartTLS()
	defer upstream.Close()

	authority, _ := NewAuthority()
	events := make(chan Event, 2)
	proxy, err := NewProxy(Options{
		Authority: authority,
		OnEvent:   func(event Event) { events <- event },
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // Test-only upstream certificate.
		},
	})
	if err != nil {
		t.Fatalf("NewProxy returned an error: %v", err)
	}
	address, err := proxy.Start()
	if err != nil {
		t.Fatalf("Start returned an error: %v", err)
	}
	defer proxy.Close()

	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(authority.PEM())
	client := proxyClientWithHTTP2(t, address, roots)
	response, err := client.Get(upstream.URL + "/h2")
	if err != nil {
		t.Fatalf("proxied HTTP/2 request returned an error: %v", err)
	}
	response.Body.Close()

	if response.Proto != "HTTP/2.0" {
		t.Fatalf("downstream response protocol = %q, want HTTP/2.0", response.Proto)
	}
	select {
	case protocol := <-upstreamProtocol:
		if protocol != "HTTP/2.0" {
			t.Fatalf("upstream request protocol = %q, want HTTP/2.0", protocol)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upstream did not receive HTTP/2 request")
	}

	event := waitForRequestEvent(t, events)
	if event.Protocol != "HTTP/2.0" || event.UpstreamProto != "HTTP/2.0" {
		t.Fatalf("captured protocols = %q -> %q, want HTTP/2.0 -> HTTP/2.0",
			event.Protocol, event.UpstreamProto)
	}
}

func TestProxyTranslatesHTTP2UpstreamResponseForHTTP1Client(t *testing.T) {
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Proto != "HTTP/2.0" {
			t.Errorf("upstream request protocol = %q, want HTTP/2.0", request.Proto)
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("translated"))
	}))
	upstream.EnableHTTP2 = true
	upstream.StartTLS()
	defer upstream.Close()

	authority, _ := NewAuthority()
	events := make(chan Event, 2)
	proxy, _ := NewProxy(Options{
		Authority: authority,
		OnEvent:   func(event Event) { events <- event },
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // Test-only upstream certificate.
		},
	})
	address, _ := proxy.Start()
	defer proxy.Close()

	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(authority.PEM())
	client := proxyClient(t, address, roots)
	response, err := client.Get(upstream.URL + "/translate")
	if err != nil {
		t.Fatalf("HTTP/1.1 client rejected translated response: %v", err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.Proto != "HTTP/1.1" || string(body) != "translated" {
		t.Fatalf("downstream response = %q body %q", response.Proto, body)
	}

	event := waitForRequestEvent(t, events)
	if event.Protocol != "HTTP/1.1" || event.UpstreamProto != "HTTP/2.0" {
		t.Fatalf("captured translated protocols = %q -> %q",
			event.Protocol, event.UpstreamProto)
	}
}

func TestProxyPassesGRPCFramesAndTrailersOverHTTP2(t *testing.T) {
	requestFrame := []byte{0, 0, 0, 0, 3, 0x0a, 0x01, 'x'}
	responseFrame := []byte{0, 0, 0, 0, 3, 0x0a, 0x01, 'y'}

	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Proto != "HTTP/2.0" {
			t.Errorf("gRPC upstream protocol = %q, want HTTP/2.0", request.Proto)
		}
		if got := request.Header.Get("Te"); !strings.EqualFold(got, "trailers") {
			t.Errorf("gRPC TE header = %q, want trailers", got)
		}
		body, _ := io.ReadAll(request.Body)
		if !bytes.Equal(body, requestFrame) {
			t.Errorf("gRPC request frame = %v, want %v", body, requestFrame)
		}

		writer.Header().Set("Content-Type", "application/grpc")
		writer.Header().Set("Trailer", "Grpc-Status, Grpc-Message")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(responseFrame)
		writer.Header().Set("Grpc-Status", "7")
		writer.Header().Set("Grpc-Message", "permission denied")
	}))
	upstream.EnableHTTP2 = true
	upstream.StartTLS()
	defer upstream.Close()

	authority, _ := NewAuthority()
	events := make(chan Event, 2)
	proxy, _ := NewProxy(Options{
		Authority: authority,
		OnEvent:   func(event Event) { events <- event },
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // Test-only upstream certificate.
		},
	})
	address, _ := proxy.Start()
	defer proxy.Close()

	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(authority.PEM())
	client := proxyClientWithHTTP2(t, address, roots)
	request, _ := http.NewRequest(http.MethodPost, upstream.URL+"/demo.Service/Call", bytes.NewReader(requestFrame))
	request.Header.Set("Content-Type", "application/grpc")
	request.Header.Set("Te", "trailers")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("proxied gRPC request returned an error: %v", err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()

	if !bytes.Equal(body, responseFrame) {
		t.Fatalf("gRPC response frame = %v, want %v", body, responseFrame)
	}
	if got := response.Trailer.Get("Grpc-Status"); got != "7" {
		t.Fatalf("downstream grpc-status = %q, want 7", got)
	}
	event := waitForRequestEvent(t, events)
	if event.Kind != "grpc" || event.GRPCStatus != "7" {
		t.Fatalf("captured gRPC event = kind %q, status %q", event.Kind, event.GRPCStatus)
	}
	if !bytes.Equal(event.Body, requestFrame) {
		t.Fatalf("captured gRPC frame = %v, want %v", event.Body, requestFrame)
	}
}

func TestProxyPassesCleartextGRPCOverH2CConnect(t *testing.T) {
	requestFrame := []byte{0, 0, 0, 0, 1, 'x'}
	upstreamListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for h2c upstream: %v", err)
	}
	defer upstreamListener.Close()

	upstreamServer := &http2.Server{}
	upstreamHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Proto != "HTTP/2.0" {
			t.Errorf("h2c upstream protocol = %q, want HTTP/2.0", request.Proto)
		}
		body, _ := io.ReadAll(request.Body)
		if !bytes.Equal(body, requestFrame) {
			t.Errorf("h2c gRPC frame = %v, want %v", body, requestFrame)
		}
		writer.Header().Set("Content-Type", "application/grpc")
		writer.Header().Set("Trailer", "Grpc-Status")
		writer.WriteHeader(http.StatusOK)
		writer.Header().Set("Grpc-Status", "0")
	})
	go func() {
		for {
			connection, acceptErr := upstreamListener.Accept()
			if acceptErr != nil {
				return
			}
			go upstreamServer.ServeConn(connection, &http2.ServeConnOpts{Handler: upstreamHandler})
		}
	}()

	authority, _ := NewAuthority()
	events := make(chan Event, 2)
	proxy, _ := NewProxy(Options{
		Authority: authority,
		OnEvent:   func(event Event) { events <- event },
	})
	proxyAddress, _ := proxy.Start()
	defer proxy.Close()

	clientTransport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(
			ctx context.Context,
			network, address string,
			_ *tls.Config,
		) (net.Conn, error) {
			dialer := &net.Dialer{}
			connection, dialErr := dialer.DialContext(ctx, network, proxyAddress)
			if dialErr != nil {
				return nil, dialErr
			}
			_, _ = fmt.Fprintf(connection,
				"CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n",
				address, address,
			)
			reader := bufio.NewReader(connection)
			status, readErr := reader.ReadString('\n')
			if readErr != nil {
				connection.Close()
				return nil, readErr
			}
			if !strings.Contains(status, "200 Connection Established") {
				connection.Close()
				return nil, fmt.Errorf("unexpected CONNECT status: %s", strings.TrimSpace(status))
			}
			for {
				line, headerErr := reader.ReadString('\n')
				if headerErr != nil {
					connection.Close()
					return nil, headerErr
				}
				if line == "\r\n" {
					break
				}
			}
			return &bufferedConnection{Conn: connection, reader: reader}, nil
		},
	}
	defer clientTransport.CloseIdleConnections()
	client := &http.Client{Transport: clientTransport, Timeout: 5 * time.Second}
	request, _ := http.NewRequest(
		http.MethodPost,
		"http://"+upstreamListener.Addr().String()+"/demo.Service/Call",
		bytes.NewReader(requestFrame),
	)
	request.Header.Set("Content-Type", "application/grpc")
	request.Header.Set("Te", "trailers")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("proxied h2c gRPC request returned an error: %v", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	response.Body.Close()

	if got := response.Trailer.Get("Grpc-Status"); got != "0" {
		t.Fatalf("h2c downstream grpc-status = %q, want 0", got)
	}
	event := waitForRequestEvent(t, events)
	if event.Kind != "grpc" || event.Protocol != "HTTP/2.0" ||
		event.UpstreamProto != "HTTP/2.0" || event.GRPCStatus != "0" {
		t.Fatalf("captured h2c gRPC event = kind %q, %q -> %q, grpc-status %q",
			event.Kind, event.Protocol, event.UpstreamProto, event.GRPCStatus)
	}
}

func TestProxyRelaysHTTP1WebSocketUpgrade(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !isHTTP1UpgradeRequest(request) {
			t.Errorf("upstream request was not a WebSocket upgrade: %#v", request.Header)
			http.Error(writer, "upgrade required", http.StatusUpgradeRequired)
			return
		}
		connection, buffered, err := http.NewResponseController(writer).Hijack()
		if err != nil {
			t.Errorf("hijack upstream: %v", err)
			return
		}
		defer connection.Close()
		_, _ = buffered.WriteString(
			"HTTP/1.1 101 Switching Protocols\r\n" +
				"Connection: Upgrade\r\n" +
				"Upgrade: websocket\r\n\r\n",
		)
		_ = buffered.Flush()
		payload := make([]byte, 4)
		_, _ = io.ReadFull(buffered, payload)
		_, _ = connection.Write(payload)
	}))
	defer upstream.Close()

	authority, _ := NewAuthority()
	events := make(chan Event, 2)
	proxy, _ := NewProxy(Options{
		Authority: authority,
		OnEvent:   func(event Event) { events <- event },
	})
	address, _ := proxy.Start()
	defer proxy.Close()

	connection, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer connection.Close()
	parsedUpstream, _ := url.Parse(upstream.URL)
	_, _ = fmt.Fprintf(connection,
		"GET %s/socket HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n",
		upstream.URL, parsedUpstream.Host,
	)
	reader := bufio.NewReader(connection)
	status, _ := reader.ReadString('\n')
	if !strings.Contains(status, "101 Switching Protocols") {
		t.Fatalf("upgrade status = %q", strings.TrimSpace(status))
	}
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			t.Fatalf("read upgrade headers: %v", readErr)
		}
		if line == "\r\n" {
			break
		}
	}
	_, _ = connection.Write([]byte("ping"))
	echo := make([]byte, 4)
	if _, err := io.ReadFull(reader, echo); err != nil {
		t.Fatalf("read WebSocket tunnel echo: %v", err)
	}
	if string(echo) != "ping" {
		t.Fatalf("WebSocket tunnel echo = %q, want ping", echo)
	}

	event := waitForRequestEvent(t, events)
	if event.Kind != "websocket" || event.Status != http.StatusSwitchingProtocols {
		t.Fatalf("captured WebSocket event = kind %q, status %d", event.Kind, event.Status)
	}
}

func TestProxyRelaysSecureWebSocketUpgrade(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !isHTTP1UpgradeRequest(request) {
			t.Errorf("upstream request was not a secure WebSocket upgrade: %#v", request.Header)
			http.Error(writer, "upgrade required", http.StatusUpgradeRequired)
			return
		}
		connection, buffered, err := http.NewResponseController(writer).Hijack()
		if err != nil {
			t.Errorf("hijack secure upstream: %v", err)
			return
		}
		defer connection.Close()
		_, _ = buffered.WriteString(
			"HTTP/1.1 101 Switching Protocols\r\n" +
				"Connection: Upgrade\r\n" +
				"Upgrade: websocket\r\n\r\n",
		)
		_ = buffered.Flush()
		payload := make([]byte, 4)
		_, _ = io.ReadFull(buffered, payload)
		_, _ = connection.Write(payload)
	}))
	defer upstream.Close()

	authority, _ := NewAuthority()
	events := make(chan Event, 2)
	proxy, _ := NewProxy(Options{
		Authority: authority,
		OnEvent:   func(event Event) { events <- event },
		Transport: &http.Transport{
			ForceAttemptHTTP2: false,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // Test-only upstream certificate.
		},
	})
	address, _ := proxy.Start()
	defer proxy.Close()

	connection, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer connection.Close()
	parsedUpstream, _ := url.Parse(upstream.URL)
	_, _ = fmt.Fprintf(connection,
		"CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n",
		parsedUpstream.Host, parsedUpstream.Host,
	)
	reader := bufio.NewReader(connection)
	status, _ := reader.ReadString('\n')
	if !strings.Contains(status, "200 Connection Established") {
		t.Fatalf("CONNECT status = %q", strings.TrimSpace(status))
	}
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			t.Fatalf("read CONNECT headers: %v", readErr)
		}
		if line == "\r\n" {
			break
		}
	}

	host, _, _ := net.SplitHostPort(parsedUpstream.Host)
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(authority.PEM())
	secureConnection := tls.Client(connection, &tls.Config{
		RootCAs:    roots,
		ServerName: host,
		NextProtos: []string{"http/1.1"},
	})
	if err := secureConnection.Handshake(); err != nil {
		t.Fatalf("TLS handshake with proxy: %v", err)
	}
	_, _ = fmt.Fprintf(secureConnection,
		"GET /socket HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n",
		parsedUpstream.Host,
	)
	secureReader := bufio.NewReader(secureConnection)
	status, _ = secureReader.ReadString('\n')
	if !strings.Contains(status, "101 Switching Protocols") {
		t.Fatalf("secure upgrade status = %q", strings.TrimSpace(status))
	}
	for {
		line, readErr := secureReader.ReadString('\n')
		if readErr != nil {
			t.Fatalf("read secure upgrade headers: %v", readErr)
		}
		if line == "\r\n" {
			break
		}
	}
	_, _ = secureConnection.Write([]byte("ping"))
	echo := make([]byte, 4)
	if _, err := io.ReadFull(secureReader, echo); err != nil {
		t.Fatalf("read secure WebSocket tunnel echo: %v", err)
	}
	if string(echo) != "ping" {
		t.Fatalf("secure WebSocket tunnel echo = %q, want ping", echo)
	}

	event := waitForRequestEvent(t, events)
	if event.Kind != "websocket" || event.Status != http.StatusSwitchingProtocols ||
		event.Protocol != "HTTP/1.1" {
		t.Fatalf("captured secure WebSocket event = kind %q, status %d, protocol %q",
			event.Kind, event.Status, event.Protocol)
	}
}

func TestProxyMarksTruncatedRequestBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writer.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	authority, _ := NewAuthority()
	events := make(chan Event, 1)
	proxy, _ := NewProxy(Options{
		Authority: authority,
		MaxBody:   4,
		OnEvent:   func(event Event) { events <- event },
	})
	address, _ := proxy.Start()
	defer proxy.Close()

	client := proxyClient(t, address, nil)
	response, err := client.Post(upstream.URL, "text/plain", strings.NewReader("123456789"))
	if err != nil {
		t.Fatalf("proxied request returned an error: %v", err)
	}
	response.Body.Close()

	event := waitForRequestEvent(t, events)
	if !event.BodyTruncated {
		t.Fatal("expected body to be marked as truncated")
	}
	if got := string(event.Body); got != "1234" {
		t.Fatalf("captured body = %q, want 1234", got)
	}
}

func proxyClientWithHTTP2(t *testing.T, address string, roots *x509.CertPool) *http.Client {
	t.Helper()
	proxyURL, err := url.Parse("http://" + address)
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy:             http.ProxyURL(proxyURL),
			ForceAttemptHTTP2: true,
			TLSClientConfig: &tls.Config{
				RootCAs: roots,
			},
		},
	}
}

func proxyClient(t *testing.T, address string, roots *x509.CertPool) *http.Client {
	t.Helper()
	proxyURL, err := url.Parse("http://" + address)
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	return &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			Proxy:             http.ProxyURL(proxyURL),
			ForceAttemptHTTP2: false,
			TLSClientConfig: &tls.Config{
				RootCAs: roots,
			},
		},
	}
}

func waitForRequestEvent(t *testing.T, events <-chan Event) Event {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case event := <-events:
			if event.Method != "" {
				return event
			}
			if event.Error != nil {
				t.Logf("proxy diagnostic event: %v", event.Error)
			}
		case <-timeout:
			t.Fatal("timed out waiting for captured request")
		}
	}
}
