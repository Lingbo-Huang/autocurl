# Changelog

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
