# Security policy

## Reporting a vulnerability

Please use GitHub private vulnerability reporting for security issues. Do not
open a public issue containing credentials, captured request data, certificate
material, or a working exploit.

## Security model

`autocurl` is a local debugging proxy. It can observe plaintext request data for
the child process it launches.

- The proxy listens on `127.0.0.1` with an ephemeral port.
- A new CA is generated for each run.
- The CA private key remains in process memory.
- Trust configuration is scoped to the child process.
- Temporary CA and truststore files are removed on exit.
- No CA is installed into the system keychain.
- Upstream TLS certificates remain subject to normal verification.

Generated cURLs are redacted by default, but redaction is heuristic. Always
review output before sharing it. `--show-secrets` should be treated as sensitive
debug output.
