#!/usr/bin/env bash
# 旧版性能基线（M3 T12）：同样本、同沙箱方式，只读运行旧版 Electron。
set -u
cd "$(dirname "$0")/../.." || exit 1
ROOT="$(pwd)"
LEGACY="$ROOT/../Mile-Knowledge-Base-master"
KIND="${1:-typical}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

VAULT="$WORK/vault"
(cd "$ROOT" && go run ./cmd/genvault -kind "$KIND" -out "$(cygpath -w "$VAULT")" >/dev/null) || exit 1
APPDATA_DIR="$WORK/appdata"
mkdir -p "$APPDATA_DIR/mile-knowledge-base"
python -I - "$VAULT" "$APPDATA_DIR" <<'PY'
import hashlib, json, os, sys
vault, appdata = sys.argv[1], sys.argv[2]
vault = os.path.realpath(vault)
vid = hashlib.sha256(vault.lower().encode()).hexdigest()[:16]
payload = {
    "theme": "light",
    "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": True,
               "sidebarCollapsed": False, "agnesControlsCollapsed": False,
               "agnesHistoryOpen": True, "agnesComposerHeight": 96},
    "recentVaults": [{"id": vid, "root": vault, "displayName": "perf-vault",
                      "lastOpenedAt": "2026-11-19T00:00:00.000Z"}],
    "lastVaultId": vid,
}
open(os.path.join(appdata, "mile-knowledge-base", "settings.json"), "w", encoding="utf-8", newline="\n").write(
    json.dumps(payload, ensure_ascii=False) + "\n"
)
PY
OUT_JSON="$WORK/result.json"
PERF_APPDATA="$APPDATA_DIR" PERF_OUT="$(cygpath -w "$OUT_JSON")" \
  "$LEGACY/node_modules/electron/dist/electron.exe" scripts/perf-legacy/bench.cjs >/dev/null 2>&1
cat "$OUT_JSON"
echo
python -I -c "import json,sys;d=json.load(open(r'$(cygpath -w "$OUT_JSON")',encoding='utf-8'));print('kind=$KIND cold=%sms searchP95=%sms samples=%s'%(d.get('coldStartMs'),d.get('searchP95Ms'),d.get('samples')))"
