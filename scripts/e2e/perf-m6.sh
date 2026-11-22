#!/usr/bin/env bash
# M6 T11: scan and registration latency on a real window. Phase 1 measures the
# M4 baseline (700 entries, no registrations); phase 2 adds 10 projects with
# 5 subdirectories and 20 skills each (1000 entries) through targets.json and
# measures the scan plus save/remove latency. Writes metrics.json and runs the
# G-PERF gate (kind=m6).
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.perf-m6-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"
mkdir -p "$VAULT/skills/前端/vue" "$APPD/Mile Knowledge Base" "$HOME_DIR/.claude/skills" "$HOME_DIR/.local/bin"
printf '# vue\n\n' > "$VAULT/skills/前端/vue/SKILL.md"
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
       "recentVaults": [{"id": vid, "root": vault, "displayName": "perf-m6", "lastOpenedAt": "2026-11-22T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

# M4 baseline: 350 dirs + 350 junctions (same shape as perf-m4).
SHARED="$WORK/shared"; mkdir -p "$SHARED"
printf '# shared\n\n' > "$SHARED/SKILL.md"
python -I - "$HOME_DIR/.claude/skills" <<'PY'
import os, sys
root = sys.argv[1]
for i in range(350):
    d = os.path.join(root, "skill-%03d" % i)
    os.makedirs(d, exist_ok=True)
    open(os.path.join(d, "SKILL.md"), "w", encoding="utf-8").write("# s%03d\n\n" % i)
PY
powershell -NoProfile -Command "\$shared = '$(cygpath -w "$SHARED")'; \$root = '$(cygpath -w "$HOME_DIR/.claude/skills")'; 0..349 | ForEach-Object { New-Item -ItemType Junction -Path (Join-Path \$root ('link-{0:D3}' -f \$_)) -Target \$shared | Out-Null }" >/dev/null

# Ten projects: five conventional subdirectories with 20 skills each.
python -I - "$WORK" <<'PY'
import json, os, sys
work = sys.argv[1]
rels = [".agents/skills", ".claude/skills", ".kiro/skills", ".cline/skills", ".warp/skills"]
targets = []
for p in range(10):
    root = os.path.join(work, "projects", "proj-%02d" % p)
    for rel in rels:
        for s in range(20):
            d = os.path.join(root, rel.replace("/", os.sep), "skill-%02d" % s)
            os.makedirs(d, exist_ok=True)
            open(os.path.join(d, "SKILL.md"), "w", encoding="utf-8").write("# p%ds%d\n\n" % (p, s))
    targets.append({
        "id": "t-%012x" % (p + 1), "kind": "project", "name": "proj-%02d" % p,
        "path": root, "installRel": ".claude/skills",
        "createdAt": "2026-11-22T09:%02d:00Z" % p,
    })
json.dump({"version": 1, "targets": targets}, open(os.path.join(work, "targets.json"), "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print("projects ok")
PY
TARGETS_FILE="$APPD/Mile Knowledge Base/skills/targets.json"
mkdir -p "$(dirname "$TARGETS_FILE")"

PORT=18546
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  sleep 2
  if [ "${PERF_KEEP:-0}" != "1" ]; then
    rm -rf "$WORK"
  fi
}
trap cleanup EXIT
if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running"; exit 2
fi
printf '{"uninstall":[]}' > "$WORK/registry.json"

start_app() {
  APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" \
    SKILLS_FAKE_HOME="$(cygpath -w "$HOME_DIR")" \
    SKILLS_FAKE_PATH="$(cygpath -w "$HOME_DIR/.local/bin")" \
    SKILLS_FAKE_REGISTRY="$(cygpath -w "$WORK/registry.json")" \
    SKILLS_FAKE_PICK_DIR="$(cygpath -w "$WORK/extra")" \
    "$APP" >"$WORK/app.log" 2>&1 &
  sleep 7
  APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -d '\r')
}
stop_app() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  APP_PID=""
  sleep 2
}
mkdir -p "$WORK/extra"

# Phase 1: M4 baseline without registrations.
start_app
python -I - "$PORT" "$(cygpath -w "$APPD/Mile Knowledge Base/skills/scan-cache.json")" "$VID" "$WORK/metrics-base.json" <<'PY'
import json, os, socket, sys, time

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

wait("!!window.__mkbApi && !!window.__mkbApp.getState().settings")
wait("document.querySelector('[data-testid=\"overview-grid\"]') !== null")
try:
    wait("!document.body.innerText.includes('未索引')", tries=120)
except SystemExit:
    pass
time.sleep(1.0)

SCAN = "window.__mkbApi.skills.scan('%s')" % vid
js(SCAN)

def fresh_scan():
    try:
        os.remove(cache_path)
    except OSError:
        pass
    return js(SCAN)

scans = []
for _ in range(10):
    t0 = time.perf_counter()
    fresh_scan()
    scans.append((time.perf_counter() - t0) * 1000)

def p95(values):
    ordered = sorted(values)
    return ordered[max(0, int(round(0.95 * len(ordered))) - 1)]

metrics = {"scanBaseP95Ms": round(p95(scans), 1)}
json.dump(metrics, open(out_path, "w", encoding="utf-8"))
print("base_ms:", " ".join("%.0f" % v for v in scans))
print("metrics-base:", json.dumps(metrics))
PY
stop_app
cp "$WORK/metrics-base.json" "$WORK/metrics.json"

# Phase 2: the same baseline plus 10 registered projects.
cp "$WORK/targets.json" "$TARGETS_FILE"
start_app
python -I - "$PORT" "$(cygpath -w "$APPD/Mile Knowledge Base/skills/scan-cache.json")" "$VID" "$WORK/metrics-projects.json" <<'PY'
import json, os, socket, sys, time

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

wait("!!window.__mkbApi && !!window.__mkbApp.getState().settings")
wait("document.querySelector('[data-testid=\"overview-grid\"]') !== null")
try:
    wait("!document.body.innerText.includes('未索引')", tries=120)
except SystemExit:
    pass
time.sleep(1.0)

SCAN = "window.__mkbApi.skills.scan('%s')" % vid
js(SCAN)

def fresh_scan():
    try:
        os.remove(cache_path)
    except OSError:
        pass
    return js(SCAN)

scans = []
for _ in range(10):
    t0 = time.perf_counter()
    fresh_scan()
    scans.append((time.perf_counter() - t0) * 1000)

# Save + remove one custom registration per round (M99/M100).
ROUND = """(async () => {
  const api = window.__mkbApi.skills;
  const pick = await api.pickTargetFolder();
  if (pick.cancelled || !pick.token) throw new Error('no pick');
  const t0 = performance.now();
  const saved = await api.saveTarget({ kind: 'custom', name: 'Perf Tool', token: pick.token });
  const saveMs = performance.now() - t0;
  const t1 = performance.now();
  await api.removeTarget(saved.target.id);
  const removeMs = performance.now() - t1;
  return [saveMs, removeMs];
})()"""
saves, removes = [], []
for _ in range(10):
    value = js(ROUND)
    saves.append(value[0])
    removes.append(value[1])

def p95(values):
    ordered = sorted(values)
    return ordered[max(0, int(round(0.95 * len(ordered))) - 1)]

metrics = {
    "scanProjectP95Ms": round(p95(scans), 1),
    "targetSaveP95Ms": round(p95(saves), 1),
    "targetRemoveP95Ms": round(p95(removes), 1),
}
json.dump(metrics, open(out_path, "w", encoding="utf-8"))
print("project_ms:", " ".join("%.0f" % v for v in scans))
print("save_ms:   ", " ".join("%.1f" % v for v in saves))
print("remove_ms: ", " ".join("%.1f" % v for v in removes))
print("metrics-projects:", json.dumps(metrics))
PY

python -I - "$WORK/metrics.json" "$WORK/metrics-projects.json" <<'PY'
import json, sys
merged = json.load(open(sys.argv[1], encoding="utf-8"))
merged.update(json.load(open(sys.argv[2], encoding="utf-8")))
json.dump(merged, open(sys.argv[1], "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print("metrics:", json.dumps(merged, ensure_ascii=False))
PY

python -I "$ROOT/scripts/perf-gate.py" --kind m6 --input "$WORK/metrics.json"
STATUS=$?
stop_pid=$APP_PID
powershell -NoProfile -Command "Stop-Process -Id $stop_pid -Force" >/dev/null 2>&1
APP_PID=""
exit $STATUS
