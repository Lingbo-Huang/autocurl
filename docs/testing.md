# Test strategy and release evidence

Autocurl combines a network proxy, runtime-specific environment injection, two
IDE adapters, and release packaging. A green unit suite alone is not sufficient,
so releases use four layers of evidence.

## Automated core tests

`go test ./...` covers:

- HTTP/1.0, HTTP/1.1, and HTTP/2 negotiation and translation;
- TLS and h2c gRPC framing, trailers, and `grpc-status`;
- classic `ws` / `wss` handshakes and bidirectional relay;
- Safe mTLS fallback and Strict interception behavior;
- client rejection of the ephemeral CA and retry bypass caching;
- Pause forwarding traffic without recording;
- redaction, body limits, selection, clipboard/output, and JSON Lines;
- existing `NO_PROXY` preservation, CIDR conversion, and diagnostics;
- request JSON adapters for Go, Java, Python, Axios, and Fetch.

`go vet ./...` provides the corresponding static check.

## IDE adapter tests

JetBrains:

```bash
cd ide/jetbrains
./gradlew test buildPlugin verifyPluginStructure verifyPluginProjectConfiguration
```

Tests cover environment merging, GoLand build overlay injection, managed engine
version enforcement, and plugin structure.

VS Code / Cursor:

```bash
cd ide/vscode
npm ci
npm run typecheck
npm run compile
npm run package
```

The compiler checks command registration, session state, debugger environment
injection, diagnostics, and packaging.

## Release workflow

GitHub Actions builds and checks every supported engine archive, the VSIX, and
the JetBrains ZIP. `scripts/check-release-version.sh` prevents tags from being
published when the CLI, both plugins, or their minimum-engine versions differ.
Every downloadable engine archive is covered by `SHA256SUMS`.

## Manual release smoke

Before a feature release:

1. install the packaged JetBrains ZIP into a supported IDE;
2. run `examples/go-http-client` with **Run Selected with Autocurl**;
3. confirm both GET and POST appear and Copy cURL contains the nested body;
4. Pause, trigger traffic, and verify the application still succeeds without a
   new request row;
5. Resume and verify rows return;
6. Stop Session and verify the associated application terminates;
7. run the clipboard JSON generation path;
8. repeat the core flow with the packaged VSIX where a local VS Code-compatible
   host is available.

## Known coverage gaps

The repository does not claim automated semantic protobuf decoding, WebSocket
message capture, HTTP/2 Extended CONNECT, HTTP/3/QUIC interception, or generic
direct access to every IDE debugger variable implementation. These boundaries
are documented instead of hidden.
