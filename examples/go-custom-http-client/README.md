# Custom Go HTTP client: ignored vs explicit proxy

This example demonstrates why a fully custom HTTP client can make no request
appear in Autocurl even though the request itself succeeds.

Create a standard **Go Application / Go Build** Run Configuration for
`main.go`, then start it with Autocurl.

## 1. Deliberately ignore the injected proxy

Program arguments:

```text
-proxy-mode ignore-env
```

The custom `http.Transport` uses `Proxy: nil`. The request goes directly to the
target, so Autocurl cannot capture it. After the diagnostic delay, the IDE
reports `no_proxy_traffic_observed`.

## 2. Configure the same client explicitly

Program arguments:

```text
-proxy-mode explicit-env-proxy
```

The example reads the process-scoped `HTTPS_PROXY` / `HTTP_PROXY` value that
Autocurl injected and assigns it to this custom transport. Select the captured
POST request to verify the complete JSON body.

This is still scoped to the one Run/Debug process. It does not change the
operating-system proxy or hard-code an Autocurl port.

For production clients, use the proxy API provided by that client library. Do
not copy this example's environment parsing blindly when the library already
has a supported proxy option.
