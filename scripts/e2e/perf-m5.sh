#!/usr/bin/env bash
# M5 T14: GitHub read and update-check latency on a real window with a local
# fake GitHub. Writes metrics.json and runs the G-PERF gate (kind=m5):
#   50 MB tar.gz -> candidates, p95 <= 3 s;
#   20 update groups checked in one round <= 2 s.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.perf-m5-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"; FIX="$WORK/fixture"
mkdir -p "$VAULT/skills/perf" "$APPD/Mile Knowledge Base" "$HOME_DIR/.local/bin" "$FIX"
printf '@echo off' > "$HOME_DIR/.local/bin/claude.cmd"
printf '{"uninstall":[]}' > "$WORK/registry.json"

VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge Base" <<'PY'
import hashlib, json, os, sys
vault, appdir = sys.argv[1], sys.argv[2]
c = os.path.abspath(vault).replace("/", chr(92)).lower()
vid = hashlib.sha256(c.encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": False,
                  "sidebarCollapsed": False, "agnesControlsCollapsed": False,
                  "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": vault, "displayName": "perf-m5", "lastOpenedAt": "2026-11-21T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

STATE="$FIX/state.json"
python -I - "$(cygpath -w "$FIX")" "$(cygpath -w "$STATE")" "$(cygpath -w "$VAULT")" <<'PY'
import io, json, os, sys, tarfile
fix, state_path, vault = sys.argv[1], sys.argv[2], sys.argv[3]

# 50 MiB of incompressible bytes: the archive stays ~50 MB on disk and the
# unpack step writes the same amount, which is what the target measures.
blob = os.urandom(50 * 1024 * 1024)
big_path = os.path.join(fix, "big.tar.gz")
with tarfile.open(big_path, "w:gz") as archive:
    for rel, data in {
        "skills/big/SKILL.md": b"# big\n\nBig skill.\n",
        "skills/big/assets/blob.bin": blob,
    }.items():
        info = tarfile.TarInfo("repo/" + rel)
        info.size = len(data)
        archive.addfile(info, io.BytesIO(data))

head = "b" * 40
repos = {"acme/big": {"ref": "main", "head": head, "paths": {"": head}, "archives": {head: big_path}}}
records = []
for i in range(20):
    name = "acme/g%02d" % i
    sha = "1" * 40
    repos[name] = {"ref": "main", "head": sha, "paths": {"skills/g%02d" % i: sha}, "archives": {}}
    records.append({
        "skillPath": "skills/perf/g%02d" % i,
        "owner": "acme", "repo": "g%02d" % i, "ref": "main",
        "path": "skills/g%02d" % i,
        "commit": sha, "pathCommit": sha,
        "baseHash": "", "installedAt": "2026-11-21T09:00:00Z",
    })
json.dump({"repos": repos}, open(state_path, "w", encoding="utf-8"))
os.makedirs(os.path.join(vault, ".mile-kb"), exist_ok=True)
json.dump({"version": 1, "records": records},
          open(os.path.join(vault, ".mile-kb", "skill-sources.json"), "w", encoding="utf-8"),
          ensure_ascii=False, indent=2)
print("fixture ok:", round(os.path.getsize(big_path) / 1024 / 1024, 1), "MiB archive")
PY

GH_PORT=18543
if python -I - "$GH_PORT" <<'PY'
import socket, sys
sock = socket.socket()
try:
    rc = sock.connect_ex(("127.0.0.1", int(sys.argv[1])))
finally:
    sock.close()
sys.exit(0 if rc == 0 else 1)
PY
then
  echo "ABORT: 127.0.0.1:$GH_PORT is already listening"; exit 2
fi
python -I "$ROOT/scripts/e2e/fake-github.py" "$(cygpath -w "$STATE")" "$GH_PORT" >"$WORK/github.log" 2>&1 &
GH_PID=$!
GH_PID=$(powershell -NoProfile -Command "(Get-CimInstance Win32_Process | Where-Object { \$_.Name -eq 'python.exe' -and \$_.CommandLine -match 'fake-github' -and \$_.CommandLine -match '$GH_PORT' } | Select-Object -First 1).ProcessId" | tr -d '\r')
python -I - "$GH_PORT" <<'PY' || { echo "ABORT: fake GitHub did not start"; exit 2; }
import socket, sys, time
port = int(sys.argv[1])
for _ in range(100):
    try:
        with socket.create_connection(("127.0.0.1", port), 0.2):
            sys.exit(0)
    except OSError:
        time.sleep(0.1)
sys.exit(1)
PY

PORT=18542
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  powershell -NoProfile -Command "Get-CimInstance Win32_Process | Where-Object { \$_.Name -eq 'python.exe' -and \$_.CommandLine -match 'fake-github' -and \$_.CommandLine -match '18543' } | ForEach-Object { Stop-Process -Id \$_.ProcessId -Force -ErrorAction SilentlyContinue }" >/dev/null 2>&1
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
  SKILLS_FAKE_GITHUB="http://127.0.0.1:$GH_PORT" \
  "$APP" >"$WORK/app.log" 2>&1 &
sleep 7
APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -d '\r')

python -I - "$PORT" "$VID" "$WORK/metrics.json" <<'PY'
import json, socket, sys, time

port, vid, out_path = int(sys.argv[1]), sys.argv[2], sys.argv[3]

def call(method, **params):
    with socket.create_connection(("127.0.0.1", port), timeout=180) as s:
        s.sendall((json.dumps({"id": 1, "method": method, "params": params}) + "\n").encode())
        line = s.makefile("rb").readline()
    data = json.loads(line)
    if "error" in data:
        raise RuntimeError(data["error"])
    return data.get("result", {})

def js(script, timeout=180):
    result = call("eval", window=1, script=script)
    value = result.get("value") if isinstance(result, dict) else result
    if isinstance(value, str):
        try:
            return json.loads(value)
        except json.JSONDecodeError:
            return value
    return value

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
time.sleep(1.0)

READ = """(async () => {
  const api = window.__mkbApi.skills;
  const t0 = performance.now();
  const res = await api.readGithub(%s, 'acme/big');
  const ms = performance.now() - t0;
  const stage = res.stage;
  if (stage) await api.discardStage(stage.stageId);
  return JSON.stringify({ms, candidates: stage ? stage.candidates.length : -1, failure: res.failure || null});
})()""" % json.dumps(vid)

CHECK = """(async () => {
  const api = window.__mkbApi.skills;
  const t0 = performance.now();
  const first = await api.checkUpdates(%s, true);
  for (;;) {
    const state = await api.checkUpdates(%s, false);
    if (!state.checking) {
      return JSON.stringify({ms: performance.now() - t0, groups: state.groups,
                             unchecked: state.unchecked, checking: first.checking});
    }
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
})()""" % (json.dumps(vid), json.dumps(vid))

reads = []
candidates = None
for _ in range(5):
    value = js(READ)
    reads.append(value["ms"])
    candidates = value["candidates"]
    if value.get("failure"):
        raise SystemExit("read failed: %s" % value["failure"])
    if value["candidates"] < 1:
        raise SystemExit("read returned no candidates")

check = js(CHECK)
if check["unchecked"] != 0 or check["groups"] != 20:
    raise SystemExit("unexpected check result: %s" % check)

def p95(values):
    ordered = sorted(values)
    return ordered[max(0, int(round(0.95 * len(ordered))) - 1)]

metrics = {
    "read50P95Ms": round(p95(reads), 1),
    "check20Ms": round(check["ms"], 1),
    "readCandidates": candidates,
    "checkGroups": check["groups"],
    "checkUnchecked": check["unchecked"],
}
json.dump(metrics, open(out_path, "w", encoding="utf-8"))
print("read50_ms:", " ".join("%.0f" % v for v in reads))
print("check20:", json.dumps(check))
print("metrics:", json.dumps(metrics))
PY

python -I "$ROOT/scripts/perf-gate.py" --kind m5 --input "$WORK/metrics.json"
STATUS=$?
stop_pid=$APP_PID
powershell -NoProfile -Command "Stop-Process -Id $stop_pid -Force" >/dev/null 2>&1
APP_PID=""
exit $STATUS
