const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const sessionSource = fs.readFileSync(
  path.join(__dirname, "..", "src", "session.ts"),
  "utf8",
);
const binarySource = fs.readFileSync(
  path.join(__dirname, "..", "src", "binary.ts"),
  "utf8",
);
const jetBrainsEngineSource = fs.readFileSync(
  path.join(
    __dirname,
    "..",
    "..",
    "jetbrains",
    "src",
    "main",
    "java",
    "com",
    "github",
    "lingbohuang",
    "autocurl",
    "EngineManager.java",
  ),
  "utf8",
);

test("engine ready timeout covers cold trust initialization", () => {
  const timeoutMatch = sessionSource.match(
    /export const ENGINE_READY_TIMEOUT_MS = ([\d_]+);/,
  );
  assert.ok(timeoutMatch, "session.ts must define ENGINE_READY_TIMEOUT_MS");

  const timeout = Number(timeoutMatch[1].replaceAll("_", ""));
  assert.ok(
    timeout >= 60_000,
    `engine ready timeout must be at least 60 seconds, got ${timeout} ms`,
  );
  assert.match(
    sessionSource,
    /}, ENGINE_READY_TIMEOUT_MS\);/,
    "the engine startup timer must use ENGINE_READY_TIMEOUT_MS",
  );
});

test("binary version checks tolerate cold macOS security scanning", () => {
  const timeoutMatch = binarySource.match(
    /export const BINARY_VERSION_TIMEOUT_MS = ([\d_]+);/,
  );
  assert.ok(timeoutMatch, "binary.ts must define BINARY_VERSION_TIMEOUT_MS");

  const timeout = Number(timeoutMatch[1].replaceAll("_", ""));
  assert.ok(
    timeout >= 30_000,
    `binary version timeout must be at least 30 seconds, got ${timeout} ms`,
  );
  assert.match(
    binarySource,
    /timeout: BINARY_VERSION_TIMEOUT_MS,/,
    "the binary version check must use BINARY_VERSION_TIMEOUT_MS",
  );
});

test("JetBrains binary version checks use the same cold-start allowance", () => {
  const timeoutMatch = jetBrainsEngineSource.match(
    /VERSION_CHECK_TIMEOUT_SECONDS = ([\d_]+);/,
  );
  assert.ok(
    timeoutMatch,
    "EngineManager.java must define VERSION_CHECK_TIMEOUT_SECONDS",
  );

  const timeout = Number(timeoutMatch[1].replaceAll("_", ""));
  assert.ok(
    timeout >= 30,
    `JetBrains binary version timeout must be at least 30 seconds, got ${timeout} seconds`,
  );
  assert.match(
    jetBrainsEngineSource,
    /process\.waitFor\(VERSION_CHECK_TIMEOUT_SECONDS, TimeUnit\.SECONDS\)/,
    "the JetBrains binary version check must use VERSION_CHECK_TIMEOUT_SECONDS",
  );
});
