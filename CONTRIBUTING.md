# Contributing

Thanks for helping improve `autocurl`.

## Before opening an issue

Run:

```bash
autocurl doctor
autocurl run --all --verbose -- <small reproducer>
```

Remove credentials, cookies, internal hostnames, customer data, and request
bodies before sharing output. Do not use `--show-secrets` in a public issue.

For protocol compatibility reports, include:

- operating system and architecture;
- `autocurl version`;
- runtime and HTTP client versions;
- client and upstream protocol shown by `autocurl`;
- whether the request is HTTP, gRPC, or WebSocket;
- a minimal sanitized reproducer.

## Development

Requires Go 1.24 or later.

```bash
make check
```

Changes to capture behavior should include an integration test. Changes to JSON
fields or command flags should preserve backward compatibility or document the
breaking change.

## Pull requests

Keep each pull request focused. Explain the user-visible problem, the protocol
or runtime affected, and the validation performed.
