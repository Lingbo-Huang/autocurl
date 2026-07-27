# autocurl

[![CI](https://github.com/Lingbo-Huang/autocurl/actions/workflows/ci.yml/badge.svg)](https://github.com/Lingbo-Huang/autocurl/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Turn real outbound HTTP failures into safe, replayable cURL commands without
changing application code.

```console
$ autocurl run --replay-header 'x-debug-info: true' -- python3 app.py

[autocurl] POST https://api.example.com/orders -> 502 (184ms)
[autocurl] http · client HTTP/2.0 · upstream HTTP/2.0
[autocurl] sensitive values were redacted; use --show-secrets only for local private debugging
curl \
  --request 'POST' \
  --url 'https://api.example.com/orders' \
  --http2 \
  --header 'Authorization: [REDACTED]' \
  --header 'Content-Type: application/json' \
  --header 'X-Debug-Info: true' \
  --data-binary '{"items":[{"sku":"A-42","quantity":2}]}' \
  --silent \
  --show-error \
  --fail-with-body
```

`autocurl` wraps one child process with a process-scoped HTTP(S) proxy. It sees
the request that was actually assembled at runtime—including method, URL,
headers, query, and body—and emits a diagnostic cURL for failed, slow, or
explicitly selected calls.

## Why

Outbound requests are often assembled across configuration, middleware,
serialization, authentication, and gateway code. Reconstructing a request by
reading source is slow and usually misses a field.

`autocurl` takes a different approach:

- no SDK or application instrumentation;
- no source-code parsing;
- no permanent system proxy;
- no certificate added to the system keychain;
- safe redaction by default;
- HTTP/1.1, HTTP/2, TLS and h2c gRPC, plus `ws`/`wss` support.

## Install

Requires Go 1.24 or later:

```bash
go install github.com/Lingbo-Huang/autocurl/cmd/autocurl@latest
autocurl doctor
```

Or build from the repository:

```bash
make build
./autocurl doctor
```

## Quick start

Capture failures and requests slower than two seconds:

```bash
autocurl run -- python3 app.py
autocurl run -- go run ./cmd/service
autocurl run -- java App.java
```

Capture every request:

```bash
autocurl run --all -- python3 app.py
```

Try the dependency-free examples:

```bash
autocurl run --all -- python3 examples/python-client/client.py
autocurl run --all -- go run ./examples/go-client
autocurl run --all -- java examples/java-client/AutocurlExample.java
```

Add a gateway debugging header only to the generated replay:

```bash
autocurl run \
  --replay-header 'x-debug-info: true' \
  -- python3 app.py
```

Inject a header into the live request as well:

```bash
autocurl run \
  --live-header 'x-debug-mode: enabled' \
  -- go run ./cmd/service
```

`--replay-header` is the safer default because it does not change application
behavior. Both header flags are repeatable and accept any `Name: value`.

## Commands

```text
autocurl doctor
autocurl run [options] -- <command> [args...]
autocurl version
```

Important `run` options:

| Option | Default | Purpose |
| --- | --- | --- |
| `--all` | off | Emit successful requests too |
| `--status-min` | `400` | Minimum HTTP status treated as a failure |
| `--slow` | `2s` | Emit successful requests slower than this; `0` disables |
| `--replay-header` | none | Add a header only to generated cURLs |
| `--live-header` | none | Add a header to live traffic and generated cURLs |
| `--max-body` | `1048576` | Maximum request-body bytes retained |
| `--show-secrets` | off | Print exact sensitive values; unsafe for logs or sharing |
| `--json` | off | Emit stable JSON Lines for automation |
| `--verbose` | off | Print proxy, CA, and runtime compatibility details |

The child process keeps its original stdout/stderr and exit code. In JSON mode,
child stdout is redirected to stderr so stdout remains valid JSON Lines.

## HTTP/1.0, HTTP/1.1, and HTTP/2

The original prototype was HTTP/1.1-only, not HTTP/1.0-only. It advertised only
`http/1.1` during TLS ALPN and explicitly disabled HTTP/2 upstream.

| Property | HTTP/1.0 | HTTP/1.1 | HTTP/2 |
| --- | --- | --- | --- |
| Wire format | Text | Text | Binary frames |
| Connection reuse | Limited | Persistent by default | Persistent and multiplexed |
| Concurrent requests | Usually separate connections | Pipelining is uncommon | Multiple streams on one connection |
| Header compression | No | No | HPACK |
| Flow control | Connection-level | Connection-level | Connection and stream-level |

`autocurl` now negotiates HTTP/2 with the wrapped client through ALPN and
independently negotiates HTTP/2 with the upstream server. The captured event
records both versions, and the generated command includes `--http1.0`,
`--http1.1`, or `--http2` when known.

## Protocol support

| Protocol | Support | What is captured |
| --- | --- | --- |
| HTTP/1.0 | Compatible | Request is accepted; upstream may be normalized to HTTP/1.1 |
| HTTP/1.1 | Supported | Method, URL, headers, body prefix, status, timing |
| HTTPS over HTTP/1.1 | Supported | Same fields through process-scoped TLS interception |
| HTTP/2 over TLS | Supported | Downstream and upstream protocol, concurrent streams, trailers |
| gRPC over TLS/HTTP/2 | Supported | RPC path, metadata, framed body prefix, `grpc-status`, trailers |
| Cleartext gRPC/h2c through CONNECT | Supported | Same gRPC transport-level fields |
| WebSocket `ws` and `wss` | Supported | HTTP/1.1 handshake and 101 status; bytes are relayed bidirectionally |
| WebSocket over HTTP/2 | Not yet | RFC 8441 Extended CONNECT is a separate protocol path |
| HTTP/3/QUIC | Not yet | QUIC does not use the TCP HTTP proxy path |
| Certificate-pinned TLS | Unsupported | The client intentionally rejects generated certificates |

### gRPC replay boundary

gRPC request bodies contain framed protobuf bytes, not shell-safe JSON.
`autocurl` passes unary and streaming traffic through and reports non-zero
`grpc-status` values as failures. It does not place binary frames inside a shell
argument; the generated cURL contains the request metadata and prints a warning.
Use `grpcurl` with server reflection or the service `.proto` for semantic replay.

### WebSocket replay boundary

For classic `ws` and `wss`, `autocurl` preserves the Upgrade handshake and then
switches to a bidirectional tunnel after `101 Switching Protocols`. It captures
the handshake, not individual WebSocket frames or messages.

## Runtime compatibility

| Runtime/client | Current support | Notes |
| --- | --- | --- |
| Go `net/http` | Verified | Proxy variables are injected; macOS/Windows `go run/build/test` receive a temporary x509 overlay |
| Python stdlib / `requests` | Verified | Proxy and CA environment variables are injected |
| Java 11+ `java.net.http` | Verified | Proxy properties and a temporary PKCS12 truststore are injected; tested with JDK 24 and HTTP/2 |
| Native gRPC clients | Transport-dependent | Standard proxy variables plus `grpc_proxy` and the ephemeral root path are injected |
| Node.js | Best effort | Environment-proxy behavior depends on Node version and HTTP client |
| Custom transports | Client-dependent | A client that ignores proxy settings cannot be forced through an explicit proxy |

Go intentionally ignores `SSL_CERT_FILE` on macOS and Windows. For Go programs
built inside the wrapped process tree, `autocurl` supplies a temporary
`crypto/x509` build overlay. It does not modify the Go installation or
application source. A previously compiled Go binary may need application-level
trust configuration.

## Safe by default

By default, `autocurl` redacts:

- authorization, cookie, API key, token, password, and secret headers;
- sensitive query parameters;
- sensitive keys nested inside JSON bodies;
- sensitive URL-encoded form fields.

Use `--show-secrets` only for private local debugging. Its output can contain
live credentials and should not be pasted into tickets, chat, CI logs, or shell
history.

HTTPS inspection uses an ephemeral CA:

- a new CA is created for each `autocurl run`;
- its private key stays in memory;
- trust is injected only into the child process;
- no certificate is added to the operating-system keychain;
- temporary CA and Java truststore files are deleted on exit;
- upstream certificates are still validated normally.

## JSON contract

`autocurl --json run ...` emits one object per selected request:

```json
{
  "schema_version": "1",
  "timestamp": "2026-07-27T04:30:00Z",
  "method": "POST",
  "url": "https://api.example.com/orders",
  "protocol": "HTTP/2.0",
  "upstream_protocol": "HTTP/2.0",
  "kind": "http",
  "status": 502,
  "duration_ms": 184,
  "curl": "curl ...",
  "redacted": true,
  "body_truncated": false
}
```

For gRPC events, `kind` is `grpc` and `grpc_status` is included when available.
Fields may be added compatibly; `schema_version` changes only for breaking
contract changes.

## Current boundaries

The first public version deliberately does not include:

- permanent request or response-body storage;
- desktop UI, cloud account, or team sharing;
- semantic protobuf decoding without reflection or `.proto` files;
- WebSocket frame/message capture;
- RFC 8441 WebSocket over HTTP/2;
- HTTP/3/QUIC interception;
- exact replay after the request body exceeds `--max-body`.

## Development

```bash
make check
```

The suite includes HTTP and HTTPS interception, HTTP/2 on both sides, TLS and
h2c gRPC framing/trailers, `ws`/`wss` bidirectional tunneling, Java truststore
creation, redaction, body truncation, JSON output, and CLI behavior.

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and reporting guidance.
The implementation is described in [docs/architecture.md](docs/architecture.md).

## License

[MIT](LICENSE)
