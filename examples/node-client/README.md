# Node.js client example

This dependency-free example explicitly honors `HTTP_PROXY` / `HTTPS_PROXY`, so
it works on Node versions older than the built-in environment-proxy support
introduced in Node 22.21 and 24.5.

```bash
autocurl run --all -- node examples/node-client/client.js
```

Use another target with:

```bash
AUTOCURL_DEMO_URL=https://api.example.com/ \
  autocurl run --all -- node examples/node-client/client.js
```

Modern Node can enable built-in environment proxying with
`NODE_USE_ENV_PROXY=1`; Autocurl injects that setting. Third-party HTTP clients
and custom Agents can still ignore it, so run `autocurl doctor` and follow the
client's explicit proxy configuration when no proxy traffic is observed.
