#!/usr/bin/env bash
# G-E2E: build the automation-enabled app, then run every acceptance script.
# Scripts are plain bash driving the real window through mygo automation.
set -u
cd "$(dirname "$0")/.." || exit 1
if [ "${E2E_SKIP_BUILD:-0}" != "1" ]; then
  echo "== build (e2e tags)"
  GOFLAGS="-tags=e2e" MYGO_INSPECTOR=1 bun run build || exit 1
fi
status=0
for script in scripts/e2e/ac-*.sh; do
  echo "== ${script}"
  bash "$script" || status=1
done
exit "$status"
