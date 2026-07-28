# autocurl

[![CI](https://github.com/Lingbo-Huang/autocurl/actions/workflows/ci.yml/badge.svg)](https://github.com/Lingbo-Huang/autocurl/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[简体中文](README.zh-CN.md)

Turn real outbound HTTP failures into safe, replayable cURL commands without
changing application code.

## IDE plugins: use it without typing commands

The same Go capture engine now powers two thin IDE plugins:

- **VS Code and Cursor:** install `autocurl-0.2.4.vsix` from
  [GitHub Releases](https://github.com/Lingbo-Huang/autocurl/releases). Start
  debugging normally. Capture starts automatically, and requests appear under
  **Explorer → Autocurl Requests**. Click a request to inspect it or click its
  action to copy the complete cURL.
- **IntelliJ IDEA, GoLand, PyCharm, WebStorm, and other JetBrains IDEs:**
  install `autocurl-jetbrains-0.2.4.zip` with
  **Settings → Plugins → ⚙ → Install Plugin from Disk**. Select an existing
  Run/Debug Configuration, then choose
  **Run → Run Selected with Autocurl** or
  **Run → Debug Selected with Autocurl**. Requests appear in the **Autocurl**
  tool window.

GoLand users can run [`examples/go-http-client`](examples/go-http-client/README.md)
immediately. It sends a GET with query parameters and a POST with a nested JSON
body.

Both plugins download the matching engine from GitHub Releases on first use,
verify it against `SHA256SUMS`, and store it in IDE-managed storage. If a
corporate network blocks GitHub downloads, install the binary once and set
`autocurl.binaryPath` in VS Code/Cursor or
**Settings → Tools → Autocurl** in JetBrains IDEs.

The version shown on the JetBrains plugin page is the IDE adapter version, not
the cached Go engine version. Updating only the engine does not change that
number. Fixes spanning both layers require installing the new plugin and
restarting the IDE; the plugin then verifies and updates its managed engine.

The plugins create only a process-scoped proxy session. They do not change the
operating-system proxy, add a permanent CA, or modify the original JetBrains
Run Configuration.

If a service uses mTLS, certificate pinning, or infrastructure calls that must
remain direct, add the corresponding host, IP, domain suffix, or CIDR under
**Settings → Tools → Autocurl → Bypass capture**. The engine also preserves
existing `NO_PROXY`, `no_proxy`, and `no_grpc_proxy` values. Bypassed traffic is
not captured; all other eligible traffic still is.

If execution is paused before the network call, select a JSON object containing
`method`, `url`, `headers`, and `body`, then run
**Autocurl: Render Selected Request JSON** in VS Code/Cursor or
**Render Selected Request JSON as cURL** in a JetBrains IDE. The cURL is copied
without sending a request.

See the [IDE plugin guide](docs/ide-plugins.md) for installation, settings,
architecture, and packaging. Maintainers can use the
[marketplace publication runbook](docs/marketplace-publishing.zh-CN.md) for
one-time account setup and automated tagged releases.

## One-minute workflow: capture and copy one request

Start the application underneath `autocurl`:

```bash
autocurl run --all --match '/orders' --method POST --copy -- java -jar app.jar
```

Keep that terminal running. Trigger the application behavior that makes the
outbound request—for example, call your service from Apifox, a browser, a test,
or another service. When a matching request is sent, the terminal prints:

```console
[autocurl #1] POST https://api.example.com/orders -> 200 (86ms)
[autocurl #1] http · client HTTP/2.0 · upstream HTTP/2.0
curl \
  --request 'POST' \
  --url 'https://api.example.com/orders' \
  --http2 \
  --header 'Content-Type: application/json' \
  --data-binary '{"order_id":"demo-42","items":[{"sku":"A-42","quantity":2}]}' \
  --silent \
  --show-error \
  --fail-with-body
[autocurl #1] copied cURL to clipboard
```

Paste the complete command with <kbd>Cmd</kbd>+<kbd>V</kbd> on macOS or
<kbd>Ctrl</kbd>+<kbd>V</kbd> elsewhere.

`--match` filters on a URL substring. `--method` narrows requests further. If
several requests match, every numbered block is printed and the clipboard holds
the latest one.

Use `--all` while learning the tool. Without it, `autocurl` intentionally emits
only:

- network/proxy errors;
- HTTP statuses at or above 400;
- successful requests slower than two seconds;
- non-zero gRPC statuses.

If a successful fast request produced no output, add `--all`.

Save the latest selected cURL instead of copying it:

```bash
autocurl run --all --match '/orders' --output ./order-request.sh -- java -jar app.jar
```

The output file is overwritten for each match and always contains the latest
selected command.

## Why

Outbound requests are often assembled across configuration, middleware,
serialization, authentication, and gateway code. Reconstructing a request by
reading source is slow and usually misses a field.

`autocurl` observes the request that was actually sent:

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

Or download a prebuilt archive from
[GitHub Releases](https://github.com/Lingbo-Huang/autocurl/releases).

Build from the repository:

```bash
make build
./autocurl doctor
```

## Runtime examples

```bash
autocurl run --all --match '/orders' --copy -- python3 app.py
autocurl run --all --match '/orders' --copy -- go run ./cmd/service
autocurl run --all --match '/orders' --copy -- java -jar app.jar
autocurl run --all --bypass 10.4.44.94 --bypass 10.61.98.0/24 -- go run ./cmd/service
```

Dependency-free smoke examples:

```bash
autocurl run --all -- python3 examples/python-client/client.py
autocurl run --all -- go run ./examples/go-client
autocurl run --all -- java examples/java-client/AutocurlExample.java
```

Add a gateway debug header only to the generated replay:

```bash
autocurl run \
  --all \
  --match '/orders' \
  --replay-header 'x-debug-info: true' \
  --copy \
  -- python3 app.py
```

Inject a header into live traffic as well:

```bash
autocurl run \
  --all \
  --live-header 'x-debug-mode: enabled' \
  -- go run ./cmd/service
```

`--replay-header` is safer because it does not change application behavior.
Both header flags are repeatable and accept any `Name: value`.

## Debugger workflows

### The request will eventually be sent

Start the debug server underneath `autocurl`, then attach the IDE. This keeps
the zero-code-change capture path.

Java with IntelliJ Remote JVM Debug:

```bash
autocurl run --all --match '/orders' --copy -- \
  java -agentlib:jdwp=transport=dt_socket,server=y,suspend=y,address=127.0.0.1:5005 \
  -jar app.jar
```

Python with `debugpy`:

```bash
autocurl run --all --match '/orders' --copy -- \
  python3 -m debugpy --listen 127.0.0.1:5678 --wait-for-client app.py
```

Go with Delve:

```bash
autocurl run --all --match '/orders' --copy -- \
  dlv debug --headless --listen=127.0.0.1:2345 --api-version=2 ./cmd/service
```

Attach the IDE to the local debug port, continue execution, and trigger the
outbound request normally.

### The debugger is paused before send

A proxy cannot capture bytes that have not entered the network. If the
debugger shows the method, URL, headers, and body, export them as JSON:

```json
{
  "method": "POST",
  "url": "https://api.example.com/orders",
  "protocol": "HTTP/2",
  "headers": {
    "Content-Type": "application/json",
    "Authorization": "Bearer local-debug-token"
  },
  "body": {
    "order_id": "demo-42",
    "items": [{"sku": "A-42", "quantity": 2}]
  }
}
```

Generate and copy a cURL without sending a request:

```bash
autocurl render --request request.json --copy
```

Or assemble values copied separately from the debugger:

```bash
autocurl render \
  --method POST \
  --url https://api.example.com/orders \
  --protocol HTTP/2 \
  --header 'Content-Type: application/json' \
  --body-file body.json \
  --copy
```

On macOS, a body copied in the debugger can be piped directly:

```bash
pbpaste | autocurl render \
  --method POST \
  --url https://api.example.com/orders \
  --header 'Content-Type: application/json' \
  --body-file - \
  --copy
```

`render` performs no network request. It uses the same redaction and cURL
renderer as live capture. The complete method, URL, headers, and body must be
available somewhere; it cannot infer fields that exist only inside application
state.

An already-running process cannot retroactively receive proxy/trust environment
variables. Restart it beneath `autocurl run`, attach the debugger, or use
`render` with values visible at the breakpoint. See
[docs/debugging.md](docs/debugging.md).

## Commands

```text
autocurl doctor
autocurl proxy [options]
autocurl render [options]
autocurl run [options] -- <command> [args...]
autocurl version
```

`proxy` is the machine-facing JSON Lines session used by the IDE plugins. The
first event is `ready`; later events are captured `request` objects. Most users
should start it through an IDE plugin rather than manually.

Important `run` options:

| Option | Default | Purpose |
| --- | --- | --- |
| `--all` | off | Emit successful requests too |
| `--match` | none | Emit only URLs containing this text |
| `--method` | none | Emit only this HTTP method |
| `--copy` | off | Copy each emitted cURL; the latest match remains |
| `--output` | none | Write the latest emitted cURL to a file |
| `--status-min` | `400` | Minimum HTTP status treated as a failure |
| `--slow` | `2s` | Emit successful requests slower than this; `0` disables |
| `--replay-header` | none | Add a header only to generated cURLs |
| `--live-header` | none | Add a header to live traffic and generated cURLs |
| `--bypass` | existing `NO_PROXY` values | Exclude a host, IP, domain suffix, or CIDR; repeatable |
| `--max-body` | `1048576` | Maximum request-body bytes retained |
| `--show-secrets` | off | Print exact sensitive values; unsafe for sharing |
| `--json` | off | Emit stable JSON Lines for automation |
| `--verbose` | off | Print proxy, CA, and runtime compatibility details |

`autocurl render --help` documents JSON, stdin, body-file, clipboard, and output
options. Clipboard support uses `pbcopy` on macOS, `clip` on Windows, and the
first available of `wl-copy`, `xclip`, or `xsel` on Linux.

The child process keeps its original stdout/stderr and exit code. In JSON mode,
child stdout moves to stderr so stdout remains valid JSON Lines.

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
independently negotiates HTTP/2 with the upstream server. Events record both
versions. Generated commands include `--http1.0`, `--http1.1`, or `--http2`
when known.

## Protocol support

| Protocol | Support | What is captured |
| --- | --- | --- |
| HTTP/1.0 | Compatible | Request is accepted; upstream may normalize to HTTP/1.1 |
| HTTP/1.1 | Supported | Method, URL, headers, body prefix, status, timing |
| HTTPS over HTTP/1.1 | Supported | Same fields through process-scoped TLS interception |
| HTTP/2 over TLS | Supported | Downstream/upstream protocol, concurrent streams, trailers |
| gRPC over TLS/HTTP/2 | Supported | RPC path, metadata, framed body prefix, `grpc-status`, trailers |
| Cleartext gRPC/h2c through CONNECT | Supported | Same gRPC transport-level fields |
| WebSocket `ws` and `wss` | Supported | HTTP/1.1 handshake and 101 status; bidirectional relay |
| WebSocket over HTTP/2 | Not yet | RFC 8441 Extended CONNECT is a separate path |
| HTTP/3/QUIC | Not yet | QUIC does not use this TCP proxy path |
| Certificate-pinned TLS | Unsupported | The client intentionally rejects generated certificates |
| Mutual TLS (mTLS) | Bypass required | A transparent MITM cannot reuse the client's private key; configure `--bypass` / IDE bypass targets |

### gRPC replay boundary

gRPC bodies contain framed protobuf bytes, not shell-safe JSON. `autocurl`
passes unary and streaming traffic through and reports non-zero `grpc-status`
values as failures. It does not place binary frames inside shell arguments. Use
`grpcurl` with reflection or the service `.proto` for semantic replay.

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
| Native gRPC clients | Transport-dependent | Standard proxy variables plus `grpc_proxy` and ephemeral root path are injected |
| Node.js | Best effort | Environment-proxy behavior depends on Node version and HTTP client |
| Custom transports | Client-dependent | Clients that ignore proxy settings cannot be forced through an explicit proxy |

Go intentionally ignores `SSL_CERT_FILE` on macOS and Windows. Go programs
built inside the wrapped process tree receive a temporary `crypto/x509` build
overlay. It does not modify the Go installation or application source. A
previously compiled Go binary may need application-level trust configuration.

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
  "type": "request",
  "id": 1,
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
  "body_truncated": false,
  "copied": true
}
```

For gRPC events, `kind` is `grpc` and `grpc_status` appears when available.
`autocurl --json render ...` returns one object containing `curl`, `redacted`,
optional `warning`, and copy/output status. Fields may be added compatibly;
`schema_version` changes only for breaking changes.

`autocurl --json proxy --lifetime-stdin` first emits:

```json
{
  "schema_version": "1",
  "type": "ready",
  "version": "0.2.4",
  "proxy_url": "http://127.0.0.1:54321",
  "ca_file": "/tmp/autocurl-.../autocurl-ca.pem",
  "environment": {
    "HTTP_PROXY": "http://127.0.0.1:54321",
    "SSL_CERT_FILE": "/tmp/autocurl-.../autocurl-ca.pem"
  },
  "append_environment": ["GOFLAGS", "JAVA_TOOL_OPTIONS"]
}
```

The environment object contains only generated overrides, never the parent
process environment. Closing plugin stdin or sending a termination signal
removes the ephemeral session files.

## Current boundaries

The current version does not include:

- permanent request or response-body storage;
- standalone desktop UI, cloud account, or team sharing;
- semantic protobuf decoding without reflection or `.proto` files;
- WebSocket frame/message capture;
- RFC 8441 WebSocket over HTTP/2;
- HTTP/3/QUIC interception;
- exact replay after the request body exceeds `--max-body`;
- retroactive attachment to an already-running process.

## How it is tested

The automated suite covers:

- HTTP/1.0, HTTP/1.1, and HTTP/2 negotiation and translation;
- TLS and h2c gRPC framing, trailers, and `grpc-status`;
- `ws`/`wss` handshakes and bidirectional byte relay;
- Java truststore creation;
- selection, numbering, clipboard/output behavior, and offline `render`;
- redaction, body truncation, JSON output, and CLI behavior.

Release smoke tests make real HTTPS calls through Python, Go, and Java. The Java
test verifies HTTP/2 from JDK to `autocurl` and from `autocurl` to the upstream.
Local integration tests verify protocol translation and WebSocket relay without
external dependencies.

## Development

```bash
make check
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidance,
[docs/architecture.md](docs/architecture.md) for internals, and
[docs/debugging.md](docs/debugging.md) for debugger workflows. IDE plugin
development and packaging are documented in
[docs/ide-plugins.md](docs/ide-plugins.md).

## License

[MIT](LICENSE)
