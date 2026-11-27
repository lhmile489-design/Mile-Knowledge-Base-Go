#!/usr/bin/env bash
# M10 T09 diagnosis: idle CPU of one build, same method as the T00 baseline
# (temp APPDATA / LOCALAPPDATA / USERPROFILE / HOME, temp typical vault,
# minimized, settle, sum the process tree). Automation is explicitly removed
# from the environment so an e2e build is measured without its TCP server.
# Usage: bash spikes/m10-effects/t09/cpu-probe.sh <app.exe> <label> [seconds] [settle] [blur]
set -u
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
APP="$1"; LABEL="$2"; SECONDS_TO_MEASURE="${3:-120}"; SETTLE="${4:-30}"; BLUR="${5:-std}"
HERE="$(cd "$(dirname "$0")" && pwd)"
WORK=$(mktemp -d "$HERE/.cpu-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/appdata"; LOCAL="$WORK/localappdata"; HOME_DIR="$WORK/home"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base" "$LOCAL" "$HOME_DIR"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null) || exit 1
python -I - "$VAULT" "$APPD/Mile Knowledge Base" "$BLUR" <<'PY'
import hashlib, json, os, sys
v, out, blur = sys.argv[1], sys.argv[2], sys.argv[3]
root = os.path.abspath(v)
vid = hashlib.sha256(root.replace('/', chr(92)).lower().encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "effects": {"motion": "std", "blur": blur},
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": True, "sidebarCollapsed": False,
                  "agnesControlsCollapsed": False, "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": root, "displayName": "cpu-probe", "lastOpenedAt": "2026-11-27T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(out, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
PY
if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: an instance is already running"; exit 2
fi
PS="$(command -v pwsh.exe || command -v powershell.exe)"
env -u MYGO_AUTOMATION \
  APPDATA="$(cygpath -w "$APPD")" LOCALAPPDATA="$(cygpath -w "$LOCAL")" \
  USERPROFILE="$(cygpath -w "$HOME_DIR")" HOME="$(cygpath -w "$HOME_DIR")" \
  "$PS" -NoProfile -ExecutionPolicy Bypass -File "$(cygpath -w "$ROOT/scripts/e2e/perf-m10-cpu.ps1")" \
  -AppPath "$(cygpath -w "$APP")" -AppData "$(cygpath -w "$APPD")" \
  -Seconds "$SECONDS_TO_MEASURE" -SettleSeconds "$SETTLE" > "$WORK/cpu.json" 2>"$WORK/cpu.err"
STATUS=$?
if [ "$STATUS" -ne 0 ]; then cat "$WORK/cpu.err"; rm -rf "$WORK"; exit "$STATUS"; fi
python -I -X utf8 - "$WORK/cpu.json" "$LABEL" "$SECONDS_TO_MEASURE" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1], encoding="utf-8"))
print(json.dumps({"label": sys.argv[2], "seconds": doc["seconds"], "cpuSeconds": doc["cpuSeconds"],
                  "cpuPercentOne": doc["cpuPercentOne"], "processCount": doc["processCount"],
                  "minimized": doc["minimized"], "window": int(sys.argv[3])}))
PY
rm -rf "$WORK"
