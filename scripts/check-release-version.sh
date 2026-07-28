#!/usr/bin/env bash

set -euo pipefail

tag="${1:-}"
if [[ ! "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "Expected a release tag such as v0.3.0, got: ${tag:-<empty>}" >&2
  exit 1
fi

expected="${tag#v}"
engine="$(go run ./cmd/autocurl version)"
vscode="$(node -p "require('./ide/vscode/package.json').version")"
jetbrains="$(
  awk -F= '$1 == "pluginVersion" { print $2 }' ide/jetbrains/gradle.properties
)"
vscode_engine="$(
  sed -nE 's/^const minimumVersion = "([^"]+)";/\1/p' ide/vscode/src/binary.ts
)"
jetbrains_engine="$(
  sed -nE 's/.*MINIMUM_VERSION = "([^"]+)";/\1/p' \
    ide/jetbrains/src/main/java/com/github/lingbohuang/autocurl/EngineManager.java
)"

failed=0
for component in engine vscode jetbrains vscode_engine jetbrains_engine; do
  actual="${!component}"
  if [[ "${actual}" != "${expected}" ]]; then
    echo "${component} version is ${actual}; release tag requires ${expected}" >&2
    failed=1
  fi
done

if [[ "${failed}" -ne 0 ]]; then
  exit 1
fi

echo "Release versions match ${tag}: engine=${engine}, vscode=${vscode}, jetbrains=${jetbrains}"
