#!/usr/bin/env bash
# G-PERF for M10 (T09, 技术设计 21.8, AC-MAT-2/3 + AC-MAT-9):
#   effectsSwitchMs <= 100   from setEffects() to the first frame painted with
#                            the new tier (the sidebar panel's alpha is the probe)
#   idleCpuRatio    <= 1.2   CPU seconds of the whole process tree over 300 s
#                            with the window minimized, over the 1.7.0 baseline.
#                            T00 measured 0.625 s; that level was not reproducible
#                            on 2026-11-27 (1.7.0 itself read 0.750 / 0.812 s), so
#                            the user approved re-baselining to 0.812 s (today's
#                            larger sample). The T00 number stays in the spike.
# Temp APPDATA / LOCALAPPDATA / USERPROFILE / HOME and a temp typical vault;
# the app it launches (and the WebView2 children) are the only ones stopped.
#
# Two binaries are involved, because they are measured for different reasons:
#   APP       the e2e build (MYGO_AUTOMATION / automation plugin) for the
#             switch phase, which drives the real store;
#   CPU_APP   a PRODUCTION build (no e2e tags) for the CPU phase, matching the
#             T00 baseline procedure (no automation server, no inspector).
# Default CPU_APP is APP, which is only useful for a smoke run.
# Reproduce the gate with:
#   GOFLAGS="-tags=e2e" MYGO_INSPECTOR=1 bun run build && mkdir -p /tmp/m10-e2e && cp "build/windows-amd64/Mile Knowledge Base.exe" /tmp/m10-e2e/
#   bun run build && mkdir -p /tmp/m10-prod && cp "build/windows-amd64/Mile Knowledge Base.exe" /tmp/m10-prod/
#   APP=/tmp/m10-e2e/"Mile Knowledge Base.exe" CPU_APP=/tmp/m10-prod/"Mile Knowledge Base.exe" bash scripts/e2e/perf-m10.sh
# Both binaries must keep the original file name: a renamed copy gets a
# different Windows process name, and the PID lookup below is by name.
# The CPU part takes 300 s by default; SECONDS_TO_MEASURE and SETTLE_SECONDS
# exist for a smoke run, the gate result in the ledger always uses 300.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="${APP:-$ROOT/build/windows-amd64/Mile Knowledge Base.exe}"
CPU_APP="${CPU_APP:-$APP}"
WORK=$(mktemp -d "$ROOT/.perf-m10-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/appdata"; LOCAL="$WORK/localappdata"; HOME_DIR="$WORK/home"
MEASURE="${SECONDS_TO_MEASURE:-300}"
SETTLE="${SETTLE_SECONDS:-60}"
BASELINE_CPU_SECONDS=0.812
PS="$(command -v pwsh.exe || command -v powershell.exe)"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base" "$LOCAL" "$HOME_DIR"

cleanup() {
  # The ABORT guard above means any instance at this point is ours.
  "$PS" -NoProfile -Command 'Get-Process "Mile Knowledge Base" -ErrorAction SilentlyContinue | Stop-Process -Force' >/dev/null 2>&1
  rm -rf "$WORK"
}
trap cleanup EXIT

if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running; close it first (this script only kills what it starts)"
  exit 2
fi

(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null) || exit 1
VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge Base" <<'PY'
import hashlib, json, os, sys
v, out = sys.argv[1], sys.argv[2]
root = os.path.abspath(v)
vid = hashlib.sha256(root.replace('/', chr(92)).lower().encode()).hexdigest()[:16]
doc = {
    "version": 1, "theme": "light", "closeBehavior": "quit",
    "effects": {"motion": "std", "blur": "std"},
    "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": True, "sidebarCollapsed": False,
               "agnesControlsCollapsed": False, "agnesHistoryOpen": True, "agnesComposerHeight": 96},
    "recentVaults": [{"id": vid, "root": root, "displayName": "perf-m10", "lastOpenedAt": "2026-11-27T09:00:00Z"}],
    "lastVaultId": vid,
}
open(os.path.join(out, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1
PORT=18562
APP_PID=""
export MYGO_AUTOMATION="tcp:127.0.0.1:$PORT"
ev() {
  python -I - "$WORK/req.json" "$1" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "eval", "params": {"window": 1, "script": sys.argv[2]}}))
PY
  timeout 60 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}
ev_json() {
  local out; out=$(ev "$1")
  python -I -X utf8 - "$out" <<'PY'
import json, sys
try:
    doc = json.loads(sys.argv[1])
    value = doc.get("result", {}).get("value")
    sys.stdout.write(value if isinstance(value, str) else json.dumps(value))
except Exception:
    sys.stdout.write("")
PY
}
wait_true() {
  for _ in $(seq 1 90); do
    ev "$1" | grep -q '"value":true' && return 0
    sleep 0.5
  done
  return 1
}
stop_app() {
  if [ -n "${APP_PID:-}" ]; then "$PS" -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  APP_PID=""
  sleep 2
}

# ---------------------------------------------------------------- switch time
echo "== effectsSwitchMs (std -> high -> off -> std, light)"
APPDATA="$(cygpath -w "$APPD")" LOCALAPPDATA="$(cygpath -w "$LOCAL")" USERPROFILE="$(cygpath -w "$HOME_DIR")" HOME="$(cygpath -w "$HOME_DIR")" \
  "$APP" >"$WORK/app.log" 2>&1 &
sleep 3
# $! is the MSYS pid; resolve the Windows pid, retrying while it starts
APP_PID=""
for _ in $(seq 1 40); do
  APP_PID=$("$PS" -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' -ErrorAction SilentlyContinue | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -cd '0-9')
  [ -n "$APP_PID" ] && break
  sleep 0.5
done
if [ -z "$APP_PID" ]; then echo "app did not start"; tail -5 "$WORK/app.log"; exit 1; fi
if ! wait_true "!!window.__mkbApp && !!window.__mkbApp.getState().settings"; then
  echo "app not ready"; tail -5 "$WORK/app.log"; exit 1
fi
wait_true "document.querySelector('[data-panel=sidebar]') !== null"
sleep 1
SWITCH=$(ev_json "(async () => {
  const frame = () => new Promise((r) => requestAnimationFrame(r));
  const alpha = () => {
    const s = getComputedStyle(document.querySelector('[data-panel=sidebar]')).backgroundColor;
    const m = s.match(/\\/\\s*([0-9.]+)\\)/);
    return m ? +m[1] : 1;
  };
  const target = { low: 0.76, std: 0.58, high: 0.34, off: 1 };
  const app = window.__mkbApp.getState();
  // 20 switches: with 8 samples a p95 is just the maximum, so one system
  // hiccup (a GC, a scheduler slice) failed the gate (T09 diagnosis: 30
  // switches across 3 runs were 6.4-25.3 ms). The 100 ms limit is unchanged.
  const levels = ['high', 'std', 'off', 'low', 'std', 'high', 'off', 'std', 'low', 'high',
                  'off', 'std', 'high', 'low', 'off', 'std', 'high', 'off', 'low', 'std'];
  const times = [];
  for (const level of levels) {
    const t0 = performance.now();
    app.setEffects({ blur: level });
    for (;;) {
      await frame();
      if (document.documentElement.dataset.blur === level && Math.abs(alpha() - target[level]) < 0.01) break;
      if (performance.now() - t0 > 2000) return JSON.stringify({ error: 'timeout', level, alpha: alpha() });
    }
    await frame();
    times.push(Math.round((performance.now() - t0) * 10) / 10);
  }
  return JSON.stringify({ times });
})()")
echo "switch samples: $SWITCH"
sleep 1
echo "stored after switches: $(python -I -X utf8 -c "import json,sys;print(json.dumps(json.load(open(sys.argv[1],encoding='utf-8')).get('effects')))" "$APPD/Mile Knowledge Base/settings.json")"
stop_app
# The switch run leaves the last level behind; AC-MAT-9 is measured at std.
python -I - "$APPD/Mile Knowledge Base/settings.json" <<'PY2'
import json, sys
path = sys.argv[1]
doc = json.load(open(path, encoding="utf-8"))
doc["effects"] = {"motion": "std", "blur": "std"}
with open(path, "w", encoding="utf-8") as fh:
    fh.write(json.dumps(doc, ensure_ascii=False, indent=2))
    fh.write(chr(10))
print("reset to std/std for the CPU run")
PY2

# ---------------------------------------------------------------- idle CPU
echo "== idleCpuRatio (std, ${MEASURE} s minimized, settle ${SETTLE} s, app $CPU_APP)"
env -u MYGO_AUTOMATION \
APPDATA="$(cygpath -w "$APPD")" LOCALAPPDATA="$(cygpath -w "$LOCAL")" USERPROFILE="$(cygpath -w "$HOME_DIR")" HOME="$(cygpath -w "$HOME_DIR")" \
  "$PS" -NoProfile -ExecutionPolicy Bypass -File "$(cygpath -w "$ROOT/scripts/e2e/perf-m10-cpu.ps1")" \
  -AppPath "$(cygpath -w "$CPU_APP")" -AppData "$(cygpath -w "$APPD")" \
  -Seconds "$MEASURE" -SettleSeconds "$SETTLE" > "$WORK/cpu.json" 2>"$WORK/cpu.err"
CPU_STATUS=$?
if [ "$CPU_STATUS" -ne 0 ]; then echo "cpu measurement failed:"; cat "$WORK/cpu.err"; exit 1; fi
cat "$WORK/cpu.json"

python -I - "$WORK/cpu.json" "$WORK/metrics.json" "$SWITCH" "$BASELINE_CPU_SECONDS" <<'PY'
import json, sys
cpu = json.load(open(sys.argv[1], encoding="utf-8"))
switch = json.loads(sys.argv[3])
if "error" in switch:
    raise SystemExit("switch measurement failed: " + json.dumps(switch))
times = sorted(switch["times"])
p95 = times[max(0, -(-95 * len(times) // 100) - 1)]
ratio = round(cpu["cpuSeconds"] / float(sys.argv[4]), 3)
metrics = {
    "effectsSwitchMs": round(p95, 1),
    "idleCpuRatio": ratio,
    "effectsSwitchMaxMs": times[-1],
    "idleCpuSeconds": cpu["cpuSeconds"],
    "idleCpuBaselineSeconds": float(sys.argv[4]),
    "idleCpuProcessCount": cpu["processCount"],
    "idleCpuMinimized": cpu["minimized"],
}
json.dump(metrics, open(sys.argv[2], "w", encoding="utf-8"))
print("switch_ms:", " ".join("%.1f" % v for v in times), " p95:", round(p95, 1))
print("cpu: %.3f s over %.1f s minimized (ratio %.3f against %.3f), processes %d"
      % (cpu["cpuSeconds"], cpu["seconds"], ratio, float(sys.argv[4]), cpu["processCount"]))
PY
echo "metrics: $(cat "$WORK/metrics.json")"
python -I "$ROOT/scripts/perf-gate.py" --kind m10 --input "$(cygpath -w "$WORK/metrics.json")"
