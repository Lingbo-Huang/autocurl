# Autocurl for JetBrains IDEs

Works with IntelliJ IDEA, GoLand, PyCharm, WebStorm, and other IntelliJ Platform
IDEs from build 251 (2025.1).

1. Install the plugin ZIP from **Settings → Plugins → ⚙ → Install Plugin from Disk**.
2. Select an existing Run/Debug Configuration.
3. Use **Run → Run Selected with Autocurl** or **Debug Selected with Autocurl**.
4. Open the **Autocurl** tool window, select a request, and click **Copy cURL**.

When execution is paused before the request is sent, copy a request object as
JSON from Variables/Watches and use **Generate cURL from Request JSON**. It
reads an editor selection, a JSON document, or the clipboard and generates a
cURL without sending network traffic.

The plugin creates a temporary copy of the selected configuration and injects
only process-scoped proxy and trust variables. It does not persist changes to
the original configuration or modify the operating-system proxy.

Services that depend on mTLS, certificate pinning, or infrastructure that must
remain direct should add those hosts/IPs under **Settings → Tools → Autocurl →
Bypass capture**. Existing `NO_PROXY`, `no_proxy`, and `no_grpc_proxy` values
are preserved. Bypassed calls are not captured; other outbound traffic remains
visible.

Safe mode is the default and keeps detected incompatible TLS destinations
end-to-end. Strict mode forces interception for diagnosis. Configure expected
local listen ports to distinguish application startup failures from upstream
connection failures.

Use **Pause Recording** to suppress new rows while proxy traffic keeps flowing.
**Stop Session** stops the associated Run/Debug process before closing the
proxy. **Clear** only empties the request list.

The matching open-source engine is downloaded from GitHub Releases and checked
against `SHA256SUMS`. If a corporate network blocks downloads, install the
engine once and set its path under **Settings → Tools → Autocurl**.

Repository: https://github.com/Lingbo-Huang/autocurl
