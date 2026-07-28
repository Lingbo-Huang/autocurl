# Autocurl for VS Code and Cursor

Capture complete outbound HTTP, HTTP/2, gRPC, and WebSocket handshake requests
from an IDE debug launch, then copy a replayable cURL from the Explorer view.

1. Install this VSIX.
2. Start debugging normally. Capture starts automatically by default.
3. Open **Explorer → Autocurl Requests**.
4. Click a request to inspect it, or use its copy button.

Use **Pause Recording** to hide new requests while traffic keeps flowing.
**Stop Session** stops the associated debug process before closing the proxy.
**Clear Requests** only empties the list.

The extension does not change the operating-system proxy or permanently install
a CA. It starts the open-source Go engine as a child process and injects an
ephemeral, process-scoped proxy environment into the debug configuration.

For mTLS, certificate-pinned, or direct infrastructure calls, set
`autocurl.bypassTargets` to the required hosts, IPs, domain suffixes, or CIDRs.
Existing `NO_PROXY`, `no_proxy`, and `no_grpc_proxy` values are preserved.

Safe mode is the default and keeps detected incompatible TLS destinations
end-to-end. Strict mode forces interception for diagnosis. Configure expected
service ports to distinguish startup failures from upstream failures.

At a breakpoint before send, copy a request object as JSON and run
**Autocurl: Generate cURL from Request JSON**. The command accepts an editor
selection, a JSON file, or clipboard content and never sends the request.

Sensitive headers, cookies, query parameters, and JSON fields are redacted by
default. Use `autocurl.showSecrets` only for private local debugging.

If automatic engine download is blocked by a corporate network, install the
`autocurl` binary once and set `autocurl.binaryPath`.

Repository and documentation:
https://github.com/Lingbo-Huang/autocurl
