# Autocurl examples

Each example makes an outbound request without importing Autocurl. Start its
normal IDE Run/Debug configuration with Autocurl so the plugin can inject a
process-scoped proxy and temporary CA.

| Language | Entry point | Recommended IDE |
| --- | --- | --- |
| Go | `go-http-client/main.go` | GoLand, VS Code, Cursor |
| Go custom transport | `go-custom-http-client/main.go` | GoLand, VS Code, Cursor |
| Java | [`java-client/AutocurlExample.java`](java-client/AutocurlExample.java) | IntelliJ IDEA, VS Code, Cursor |
| Python | `python-client/client.py` | PyCharm, VS Code, Cursor |
| Node.js | `node-client/client.js` | VS Code, Cursor, WebStorm |

The repository includes `.vscode/launch.json` configurations for all four
examples. Install the recommended language extensions, choose a configuration,
and use **Run and Debug**. Autocurl starts automatically when
`autocurl.autoStartOnDebug` is enabled.

For GoLand, first run `go-http-client/main.go` normally once to create a Go
Build configuration. Then choose **Run > Run Selected with Autocurl**.

For IntelliJ IDEA, open `java-client` as a Maven project, run
`AutocurlExample.main` normally once, then click **Run Selected** in the
Autocurl tool window. The [Java example guide](java-client/README.md) includes
the expected HTTP/2 output and a nested JSON POST body to copy.

To keep the Go demo alive while testing the session controls, add:

```text
-repeat 10 -delay 2s
```

to its program arguments. **Pause Recording** stops adding rows while the
requests continue to work. **Resume Recording** records again. **Stop Session**
stops the associated program before shutting down the capture engine.

The [custom Go client](go-custom-http-client/README.md) intentionally ignores
`HTTP_PROXY` / `HTTPS_PROXY`, then shows how the same client can use Autocurl's
process-scoped proxy explicitly.
