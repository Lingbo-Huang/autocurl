# Debugging guide

There are two debugging situations. They need different paths because a proxy
can observe network traffic but cannot inspect arbitrary in-memory objects
inside another process.

## 1. The request will be sent

Launch the debug target beneath `autocurl`, then attach the IDE to the language
debug server.

### Java / IntelliJ IDEA

```bash
autocurl run --all --match '/orders' --copy -- \
  java -agentlib:jdwp=transport=dt_socket,server=y,suspend=y,address=127.0.0.1:5005 \
  -jar app.jar
```

Create a Remote JVM Debug configuration for `127.0.0.1:5005`. After attaching,
continue execution and trigger the request. `autocurl` prints and copies the
request after the HTTP client sends it.

### Python / debugpy

Install `debugpy` in the application's environment, then run:

```bash
autocurl run --all --match '/orders' --copy -- \
  python3 -m debugpy --listen 127.0.0.1:5678 --wait-for-client app.py
```

Attach VS Code or another DAP client to port 5678.

### Go / Delve

```bash
autocurl run --all --match '/orders' --copy -- \
  dlv debug --headless --listen=127.0.0.1:2345 --api-version=2 ./cmd/service
```

Attach the Go IDE to port 2345.

## 2. The debugger is paused before send

At this point no network traffic exists. Export the prepared request values and
pass them to `autocurl render`.

```bash
autocurl render --request request.json --copy
```

Input schema:

```json
{
  "method": "POST",
  "url": "https://api.example.com/orders",
  "protocol": "HTTP/2",
  "headers": {
    "Content-Type": "application/json",
    "X-Request-Id": ["request-42"]
  },
  "body": {
    "order_id": "demo-42"
  }
}
```

Header values may be strings or string arrays. A JSON object/array body is
compacted and receives `Content-Type: application/json` when no content type is
provided. A string body is used literally. Binary bodies may use
`body_base64`, but cannot always be represented safely as a shell argument.

The request can also be assembled from separate debugger values:

```bash
autocurl render \
  --method POST \
  --url https://api.example.com/orders \
  --protocol HTTP/2 \
  --header 'Content-Type: application/json' \
  --header 'X-Request-Id: request-42' \
  --body-file ./body.json \
  --copy
```

Use `--show-secrets` only when exact local credentials are required. Default
output is redacted.

## Already-running processes

`autocurl` cannot retroactively change the proxy or trust environment of a
process that is already running. Restart it beneath `autocurl run`, attach the
debugger as shown above, or use `render` with values visible at the breakpoint.
