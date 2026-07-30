# Autocurl roadmap

Autocurl is built in the open. The roadmap follows real compatibility reports
and team workflows rather than invented feature counts.

## Now

- Make first-run capture reliable across JetBrains IDEs, VS Code, and Cursor.
- Expand public Go, Java, Python, and Node.js examples.
- Improve HTTP/2, gRPC, WebSocket, mTLS, and certificate diagnostics.
- Publish the IDE plugins to their official marketplaces.
- Keep request processing local, process-scoped, and redacted by default.

## Next

- A public runtime/client compatibility matrix.
- Sanitized export bundles for reproducible bug reports.
- Repository-level capture presets that can be reviewed in code.
- More debugger-object templates for generating cURL before a request is sent.
- Signed, offline-friendly engine and plugin distribution guidance.

## Team pilot

The paid team layer is being validated before it is built. Candidate areas:

- shared redaction and header-preservation policies;
- workspace capture and bypass presets;
- self-hosted plugin/engine distribution;
- policy validation and audit-friendly configuration;
- compatibility support for private gateways and infrastructure.

The local capture engine and individual IDE workflow remain open source under
MIT. If your team has one of the problems above, open a
[team pilot request](https://github.com/Lingbo-Huang/autocurl/issues/new?template=team_pilot.yml).

## Later, only with evidence

- Team policy management UI.
- SSO, RBAC, and signed organization policies.
- Organization-wide compatibility reporting without uploading captured
  requests.
- Enterprise support and certified runtime/client combinations.

Vote with a sanitized
[feature request](https://github.com/Lingbo-Huang/autocurl/issues/new?template=feature_request.yml)
or contribute a minimal public reproduction.
