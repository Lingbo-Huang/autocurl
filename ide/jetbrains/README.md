# Autocurl for JetBrains IDEs

Works with IntelliJ IDEA, GoLand, PyCharm, WebStorm, and other IntelliJ Platform
IDEs from build 251 (2025.1).

1. Install the plugin ZIP from **Settings → Plugins → ⚙ → Install Plugin from Disk**.
2. Select an existing Run/Debug Configuration.
3. Use **Run → Run Selected with Autocurl** or **Debug Selected with Autocurl**.
4. Open the **Autocurl** tool window, select a request, and click **Copy cURL**.

When execution is paused before the request is sent, select a request JSON
object in the editor and use **Render Selected Request JSON as cURL**. It
generates and copies a cURL without sending network traffic.

The plugin creates a temporary copy of the selected configuration and injects
only process-scoped proxy and trust variables. It does not persist changes to
the original configuration or modify the operating-system proxy.

Services that depend on mTLS, certificate pinning, or infrastructure that must
remain direct should add those hosts/IPs under **Settings → Tools → Autocurl →
Bypass capture**. Existing `NO_PROXY`, `no_proxy`, and `no_grpc_proxy` values
are preserved. Bypassed calls are not captured; other outbound traffic remains
visible.

The matching open-source engine is downloaded from GitHub Releases and checked
against `SHA256SUMS`. If a corporate network blocks downloads, install the
engine once and set its path under **Settings → Tools → Autocurl**.

Repository: https://github.com/Lingbo-Huang/autocurl
