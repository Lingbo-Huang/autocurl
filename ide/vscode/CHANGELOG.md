# Changelog

## 0.3.2

- Keep the IDE packages aligned with the 0.3.2 engine release.
- Keep JetBrains tool-window actions visible at the default narrow width.
- Add an IntelliJ IDEA-ready Maven project for the Java HTTP/2 example.
- Clarify that bodyless GET requests correctly show only URL and headers.
- Allow macOS security scanning to finish before rejecting a newly downloaded
  Autocurl engine during its version check.

## 0.3.1

- Allow cold macOS engine initialization to finish before reporting a startup
  timeout in VS Code and Cursor.
- Keep the IDE packages aligned with the 0.3.1 engine release.

## 0.3.0

- Add Safe and Strict Capture modes with actionable TLS and mTLS diagnostics.
- Pause recording without interrupting application traffic.
- Stop the associated Run/Debug session before shutting down the proxy.
- Keep Clear Requests independent from both the application and proxy lifecycle.
- Generate cURLs from the editor, JSON files, clipboard, and common Go, Java,
  Python, Axios, and Fetch request shapes.
- Preserve all existing bypass settings and diagnose missing listen ports,
  startup exits, and HTTP clients that ignore proxy environment variables.

## 0.2.4

- Preserve existing `NO_PROXY`, `no_proxy`, and `no_grpc_proxy` values.
- Add `autocurl.bypassTargets` for mTLS and direct infrastructure calls.
- Keep the IDE packages aligned with the 0.2.4 engine.

## 0.2.3

- Discover the configured Go SDK when VS Code or Cursor is launched from a
  macOS GUI environment with a minimal `PATH`.
- Keep the IDE packages aligned with the corrected 0.2.3 engine.

## 0.2.2

- Require the matching Autocurl engine so macOS Go HTTPS capture gets the
  process-scoped CA trust fix.
- Add automated Visual Studio Marketplace and Open VSX publishing.
- Add release version guards shared with the JetBrains plugin.

## 0.2.1

- Keep the IDE packages aligned with the Autocurl 0.2.1 engine release.
- Document the richer Go HTTP client example used to verify capture.

## 0.2.0

- Start and stop process-scoped capture sessions from the IDE.
- Automatically inject capture settings into VS Code and Cursor debug launches.
- Show captured requests and copy any complete cURL.
- Render selected debugger request JSON without sending network traffic.
- Download and verify the matching Go engine from GitHub Releases.
