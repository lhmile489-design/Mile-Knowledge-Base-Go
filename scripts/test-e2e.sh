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
# A previous script can leave its app behind when the machine is slow to start
# it (several scripts resolve the Windows PID a fixed moment after launch and
# the lookup races the start; T09 hit this and every later script aborted on
# purpose). Those instances are ours; a user's installed app is not, so only
# processes whose path is inside this repository are stopped, and the fact is
# printed rather than hidden.
reap_leftovers() {
  local count
  # \$_ must stay literal: inside double quotes bash would expand $_.
  count=$(powershell.exe -NoProfile -Command "@(Get-Process 'Mile Knowledge Base' -ErrorAction SilentlyContinue | Where-Object { \$_.Path -like '*Mile-Knowledge-Base-Go*' }).Count" | tr -cd '0-9')
  if [ -n "${count:-}" ] && [ "$count" != "0" ]; then
    echo "-- reaping $count leftover app instance(s) from this repository"
    powershell.exe -NoProfile -Command 'Get-Process "Mile Knowledge Base" -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "*Mile-Knowledge-Base-Go*" } | Stop-Process -Force' >/dev/null 2>&1
    sleep 3
  fi
}
# Each script gets its own LOCALAPPDATA: WebView2 keeps its user-data profile
# there, and back-to-back instances sharing it fail to initialise while the
# previous instance's WebView2 children are still shutting down (T09 run4: the
# app started but never became ready, every automation call returned empty).
LOCAL_ROOT=".e2e-local"
rm -rf "$LOCAL_ROOT"
mkdir -p "$LOCAL_ROOT"
for script in scripts/e2e/ac-*.sh; do
  echo "== ${script}"
  reap_leftovers
  name=$(basename "$script" .sh)
  rm -rf "$LOCAL_ROOT/$name"
  mkdir -p "$LOCAL_ROOT/$name"
  LOCALAPPDATA="$(cygpath -w "$LOCAL_ROOT/$name")" bash "$script" || status=1
done
reap_leftovers
sleep 3
rm -rf "$LOCAL_ROOT"
exit "$status"
