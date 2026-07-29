const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const sessionSource = fs.readFileSync(
  path.join(__dirname, "..", "src", "session.ts"),
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
