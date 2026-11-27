#!/usr/bin/env bash
# M10 T09 diagnosis: per-level switch times (which switch is slow?), repeated.
# Usage: bash spikes/m10-effects/t09/switch-probe.sh <e2e app.exe> [repeats]
set -u
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
APP="$1"; REPEATS="${2:-3}"
WORK=$(mktemp -d "$ROOT/.sw-XXXXXX")
mkdir -p "$WORK/app/Mile Knowledge Base" "$WORK/vault" "$WORK/home"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$WORK/vault")" >/dev/null) || exit 1
python -I - "$WORK/vault" "$WORK/app/Mile Knowledge Base" <<'PY'
import hashlib, json, os, sys
v, out = sys.argv[1], sys.argv[2]
root = os.path.abspath(v)
vid = hashlib.sha256(root.replace('/', chr(92)).lower().encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit", "effects": {"motion": "std", "blur": "std"},
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": True, "sidebarCollapsed": False,
                  "agnesControlsCollapsed": False, "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": root, "displayName": "sw", "lastOpenedAt": "2026-11-27T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(out, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
PY
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$WORK/drive.exe" .) || exit 1
PORT=18564
export MYGO_AUTOMATION="tcp:127.0.0.1:$PORT"
APPDATA="$(cygpath -w "$WORK/app")" HOME="$(cygpath -w "$WORK/home")" "$APP" >"$WORK/app.log" 2>&1 &
sleep 4
PS_PID=""
for _ in $(seq 1 40); do
  PS_PID=$(powershell.exe -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' -ErrorAction SilentlyContinue | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -cd '0-9')
  [ -n "$PS_PID" ] && break
  sleep 0.5
done
if [ -z "$PS_PID" ]; then echo "app did not start"; exit 1; fi
ev() {
  python -I - "$WORK/req.json" "$1" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "eval", "params": {"window": 1, "script": sys.argv[2]}}))
PY
  timeout 60 "$WORK/drive.exe" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}
for _ in $(seq 1 60); do
  ev "!!window.__mkbApp && !!window.__mkbApp.getState().settings" | grep -q '"value":true' && break
  sleep 0.5
done
sleep 1
for r in $(seq 1 "$REPEATS"); do
  OUT=$(ev "(async () => {
    const frame = () => new Promise((res) => requestAnimationFrame(res));
    const alpha = () => {
      const s = getComputedStyle(document.querySelector('[data-panel=sidebar]')).backgroundColor;
      const m = s.match(/\/\s*([0-9.]+)\)/);
      return m ? +m[1] : 1;
    };
    const target = { low: 0.76, std: 0.58, high: 0.34, off: 1 };
    const app = window.__mkbApp.getState();
    const levels = ['high','std','off','std','high','off','low','std','high','off'];
    const times = [];
    for (const level of levels) {
      const t0 = performance.now();
      app.setEffects({ blur: level });
      for (;;) {
        await frame();
        if (document.documentElement.dataset.blur === level && Math.abs(alpha() - target[level]) < 0.01) break;
        if (performance.now() - t0 > 3000) return JSON.stringify({ error: 'timeout', level, alpha: alpha() });
      }
      await frame();
      times.push({ level, ms: Math.round((performance.now() - t0) * 10) / 10 });
    }
    return JSON.stringify(times);
  })()")
  echo "repeat $r: $OUT"
done
powershell.exe -NoProfile -Command "Stop-Process -Id $PS_PID -Force" >/dev/null 2>&1
sleep 2
rm -rf "$WORK"
