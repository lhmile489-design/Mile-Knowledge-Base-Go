#!/usr/bin/env bash
# M5 T14 B-M5-02: manual REAL GitHub smoke. This script contacts github.com
# with the anonymous quota and must only run after the user agreed to it.
# It reads one public repository, installs one skill into a temporary vault
# (fake HOME, e2e build), prints the evidence and deletes everything.
#
# Usage: bash scripts/e2e/manual-github-smoke.sh [owner/repo]
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
REPO="${1:-anthropics/skills}"
WORK=$(mktemp -d "$ROOT/.smoke-github-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"
mkdir -p "$VAULT/skills" "$APPD/Mile Knowledge Base" "$HOME_DIR/.local/bin"
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
       "recentVaults": [{"id": vid, "root": vault, "displayName": "github-smoke", "lastOpenedAt": "2026-11-21T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

PORT=18544
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

# No SKILLS_FAKE_GITHUB: this script talks to the real GitHub on purpose.
APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" \
  SKILLS_FAKE_HOME="$(cygpath -w "$HOME_DIR")" \
  SKILLS_FAKE_PATH="$(cygpath -w "$HOME_DIR/.local/bin")" \
  SKILLS_FAKE_REGISTRY="$(cygpath -w "$WORK/registry.json")" \
  "$APP" >"$WORK/app.log" 2>&1 &
sleep 7
APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -d '\r')

python -I - "$PORT" "$VID" "$REPO" <<'PY'
import json, socket, sys, time

port, vid, repo = int(sys.argv[1]), sys.argv[2], sys.argv[3]

def call(method, **params):
    with socket.create_connection(("127.0.0.1", port), timeout=180) as s:
        s.sendall((json.dumps({"id": 1, "method": method, "params": params}) + "\n").encode())
        line = s.makefile("rb").readline()
    data = json.loads(line)
    if "error" in data:
        raise RuntimeError(data["error"])
    return data.get("result", {})

def js(script):
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
  const res = await api.readGithub(%s, %s);
  const stage = res.stage;
  return JSON.stringify({
    ms: performance.now() - t0,
    stageId: stage ? stage.stageId : "",
    label: stage ? stage.label : "",
    origin: stage ? stage.origin : null,
    candidates: stage ? stage.candidates.map((c) => ({id: c.id, name: c.name, state: c.state})) : [],
  });
})()""" % (json.dumps(vid), json.dumps(repo))

read = js(READ)
print("read:", json.dumps(read, ensure_ascii=False))
if not read.get("stageId"):
    raise SystemExit("no stage from the real GitHub")
pick = next((c for c in read["candidates"] if c["state"] == "ok"), None)
if pick is None:
    raise SystemExit("no installable candidate in %s" % repo)

INSTALL = """(async () => {
  const api = window.__mkbApi.skills;
  const value = await api.install(%s, %s, [%s], {kind: 'kb', category: '文档'});
  return JSON.stringify(value);
})()""" % (json.dumps(vid), json.dumps(read["stageId"]), json.dumps(pick["id"]))
installed = js(INSTALL)
print("install:", json.dumps(installed, ensure_ascii=False))
if not any(item.get("result") == "copied" for item in installed.get("items", [])):
    raise SystemExit("install did not copy the skill")
print("skill:", pick["name"])
PY

STATUS=$?
echo "--- installed files ---"
find "$VAULT/skills" -mindepth 2 -maxdepth 3 -type f | head -10
echo "--- source record ---"
if [ -f "$VAULT/.mile-kb/skill-sources.json" ]; then
  head -40 "$VAULT/.mile-kb/skill-sources.json"
else
  echo "missing"
fi
stop_pid=$APP_PID
powershell -NoProfile -Command "Stop-Process -Id $stop_pid -Force" >/dev/null 2>&1
APP_PID=""
exit $STATUS
