#!/usr/bin/env bash
# M4 T16: skills scan and operation latency on a real window with a fake HOME
# holding ~700 entries (350 dirs + 350 junctions). Writes metrics.json and
# runs the G-PERF gate (kind=skills).
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.perf-m4-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"
mkdir -p "$VAULT/skills/前端/vue" "$APPD/Mile Knowledge Base" "$HOME_DIR"
printf '# vue\n\n' > "$VAULT/skills/前端/vue/SKILL.md"
mkdir -p "$HOME_DIR/.claude/skills" "$HOME_DIR/.local/bin"
printf '@echo off' > "$HOME_DIR/.local/bin/claude.cmd"

VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge Base" <<'PY'
import hashlib, json, os, sys
vault, appdir = sys.argv[1], sys.argv[2]
c = os.path.abspath(vault).replace("/", chr(92)).lower()
vid = hashlib.sha256(c.encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": False,
                  "sidebarCollapsed": False, "agnesControlsCollapsed": False,
                  "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": vault, "displayName": "perf-m4", "lastOpenedAt": "2026-11-20T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

# 350 real skill dirs + 350 junctions to a shared target.
SHARED="$WORK/shared"; mkdir -p "$SHARED"
printf '# shared\n\n' > "$SHARED/SKILL.md"
python -I - "$HOME_DIR/.claude/skills" "$SHARED" <<'PY'
import os, sys
root, shared = sys.argv[1], sys.argv[2]
for i in range(350):
    d = os.path.join(root, "skill-%03d" % i)
    os.makedirs(d, exist_ok=True)
    open(os.path.join(d, "SKILL.md"), "w", encoding="utf-8").write("# s%03d\n\n" % i)
print("real dirs ok")
PY
powershell -NoProfile -Command "\$shared = '$(cygpath -w "$SHARED")'; \$root = '$(cygpath -w "$HOME_DIR/.claude/skills")'; 0..349 | ForEach-Object { New-Item -ItemType Junction -Path (Join-Path \$root ('link-{0:D3}' -f \$_)) -Target \$shared | Out-Null }" >/dev/null
echo "entries: $(find "$HOME_DIR/.claude/skills" -mindepth 1 -maxdepth 1 | wc -l)"

PORT=18541
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  sleep 2
  rm -rf "$WORK"
}
trap cleanup EXIT
if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running"; exit 2
fi

APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" \
  SKILLS_FAKE_HOME="$(cygpath -w "$HOME_DIR")" \
  SKILLS_FAKE_PATH="$(cygpath -w "$HOME_DIR/.local/bin")" \
  SKILLS_FAKE_REGISTRY="$(cygpath -w "$WORK/registry.json")" \
  "$APP" >"$WORK/app.log" 2>&1 &
printf '{"uninstall":[]}' > "$WORK/registry.json"
sleep 7
APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -d '\r')

python -I - "$PORT" "$(cygpath -w "$APPD/Mile Knowledge Base/skills/scan-cache.json")" "$VID" "$WORK/metrics.json" <<'PY'
import json, os, socket, statistics, sys, time

port, cache_path, vid, out_path = int(sys.argv[1]), sys.argv[2], sys.argv[3], sys.argv[4]

def call(method, **params):
    with socket.create_connection(("127.0.0.1", port), timeout=30) as s:
        s.sendall((json.dumps({"id": 1, "method": method, "params": params}) + "\n").encode())
        line = s.makefile("rb").readline()
    data = json.loads(line)
    if "error" in data:
        raise RuntimeError(data["error"])
    return data.get("result", {})

def js(script):
    return call("eval", window=1, script=script).get("value")

def wait(script, tries=240):
    for _ in range(tries):
        try:
            if js(script):
                return
        except Exception:
            pass
        time.sleep(0.5)
    raise SystemExit("wait timeout: " + script[:80])

wait("!!window.__mkbApp && !!window.__mkbApp.getState().settings")
wait("document.querySelector('[data-testid=\"overview-grid\"]') !== null")
try:
    wait("!document.body.innerText.includes('未索引')", tries=120)
except SystemExit:
    pass
time.sleep(1.0)

SCAN = "window.__mkbApi.skills.scan('%s')" % vid
js(SCAN)  # warm-up

def fresh_scan():
    try:
        os.remove(cache_path)
    except OSError:
        pass
    return js(SCAN)

scans = []
for _ in range(20):
    t0 = time.perf_counter()
    fresh_scan()
    scans.append((time.perf_counter() - t0) * 1000)

def p95(values):
    ordered = sorted(values)
    return ordered[max(0, int(round(0.95 * len(ordered))) - 1)]

# Distribute + uninstall against the shared directory (link first).
ops = []
for _ in range(10):
    t0 = time.perf_counter()
    js("window.__mkbApi.skills.distribute('%s','skills/前端/vue',[{targetId:'agents',allowOverwrite:true}],'link')" % vid)
    ops.append((time.perf_counter() - t0) * 1000)
    js("window.__mkbApi.skills.uninstall('agents','vue')")

metrics = {"scanP95Ms": round(p95(scans), 1), "opP95Ms": round(p95(ops), 1)}
json.dump(metrics, open(out_path, "w", encoding="utf-8"))
print("scan_ms:", " ".join("%.0f" % v for v in scans))
print("op_ms:  ", " ".join("%.0f" % v for v in ops))
print("metrics:", json.dumps(metrics))
PY

python -I "$ROOT/scripts/perf-gate.py" --kind skills --input "$WORK/metrics.json"
STATUS=$?
stop_pid=$APP_PID
powershell -NoProfile -Command "Stop-Process -Id $stop_pid -Force" >/dev/null 2>&1
APP_PID=""
exit $STATUS
