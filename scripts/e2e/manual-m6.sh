#!/usr/bin/env bash
# M6 T11 manual item (B-M6-03; user consent given 2026-11-22): register this
# repository as a real project and scan it read-only, then distribute a test
# skill into a temporary custom directory. A fake HOME keeps the real tool
# directories untouched; a temporary APPDATA keeps the real app data out of it.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
# System temp on purpose: the temporary fake HOME must not sit inside the
# repository, or the project would overlap the built-in tool directories.
WORK=$(mktemp -d)
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"
CUSTOM="$WORK/manual-custom"
mkdir -p "$VAULT/skills/验收/m6-test" "$APPD/Mile Knowledge Base" "$HOME_DIR/.claude/skills" "$HOME_DIR/.local/bin" "$CUSTOM"
printf '# m6-test\n\n验收用测试技能。\n' > "$VAULT/skills/验收/m6-test/SKILL.md"
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
       "recentVaults": [{"id": vid, "root": vault, "displayName": "manual-m6", "lastOpenedAt": "2026-11-22T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

QUEUE="$WORK/queue.txt"
python -I - "$QUEUE" "$(cygpath -w "$CUSTOM")" "$(cygpath -w "$ROOT")" <<'PY'
import sys
open(sys.argv[1], "w", encoding="utf-8").write("\n".join(sys.argv[2:]) + "\n")
PY
printf '{"uninstall":[]}' > "$WORK/registry.json"

snapshot() { python -I - "$1" <<'PY'
import hashlib, os, sys
root = sys.argv[1]
h = hashlib.sha256()
if os.path.isdir(root):
    for base, dirs, files in os.walk(root):
        dirs.sort()
        for name in sorted(files):
            path = os.path.join(base, name)
            try:
                st = os.stat(path, follow_symlinks=False)
                h.update(os.path.relpath(path, root).encode())
                h.update(str((st.st_size, int(st.st_mtime))).encode())
            except OSError:
                h.update(b"?")
print(h.hexdigest())
PY
}
REPO_BEFORE=$(snapshot "$ROOT/.claude/skills")
REAL_BEFORE=$(snapshot "$USERPROFILE/.claude/skills")

PORT=18551
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
  SKILLS_FAKE_PICK_DIR="@$(cygpath -w "$QUEUE")" \
  "$APP" >"$WORK/app.log" 2>&1 &
sleep 7
APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-StartTime -Descending | Select-Object -First 1).Id" 2>/dev/null | tr -d '\r')
if [ -z "$APP_PID" ]; then
  APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -d '\r')
fi

python -I - "$PORT" "$VID" "$(cygpath -w "$APPD/Mile Knowledge Base/skills/scan-cache.json")" "$(cygpath -w "$ROOT")" "$(cygpath -w "$CUSTOM")" "$WORK/result.json" <<'PY'
import json, os, socket, sys, time

port, vid, cache_path, repo, custom, out_path = int(sys.argv[1]), sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5], sys.argv[6]
passed, failed = [], []

def check(name, condition, detail=""):
    (passed if condition else failed).append(name)
    print(("PASS " if condition else "FAIL ") + name + ((" :: " + str(detail)) if not condition else ""))

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

api = "window.__mkbApi.skills"
# 1) Register the temporary custom directory and the real repository.
custom_save = js("(async () => { const p = await %s.pickTargetFolder();"
                 " const r = await %s.saveTarget({kind:'custom', name:'Manual Tool', token:p.token});"
                 " return r.target ? r.target.id : ''; })()" % (api, api))
check("custom registration saved", bool(custom_save), custom_save)
repo_save = js("(async () => { const p = await %s.pickTargetFolder();"
               " const r = await %s.saveTarget({kind:'project', name:'MKB Repo', token:p.token, installDir:'.claude/skills'});"
               " return r.target ? r.target.id : ''; })()" % (api, api))
check("real project registered", bool(repo_save), repo_save)

# 2) Read-only scan of the real repository.
try:
    os.remove(cache_path)
except OSError:
    pass
scan = js("%s.scan('%s')" % (api, vid))["scan"]
group = next((g for g in scan["tools"] if g["toolId"] == repo_save), None)
check("project group present", group is not None)
skills = group["skills"] if group else []
check("project skills found", len(skills) >= 9, len(skills))
known = next((s for s in skills if s["name"] == "agnes-engine"), None)
check("known skill scanned", known is not None)
if known:
    check("source tool attributed", known.get("sourceToolId") == "claude-code", known.get("sourceToolId"))
    check("subdir recorded", known.get("subdir") == ".claude/skills", known.get("subdir"))
    check("path masked", repo.lower() not in known["path"].lower(), known["path"])

# 3) Distribute the test skill into the temporary custom directory.
op = js("(async () => { const r = await %s.distribute('%s','skills/验收/m6-test',"
        " [{targetId:'%s'}], 'link'); return r.items[0].result; })()" % (api, vid, custom_save))
check("link distributed", op == "linked", op)

# 4) Remove the custom registration; its files stay.
removed = js("(async () => { const r = await %s.removeTarget('%s');"
             " return JSON.stringify({removed:r.removed, d:r.distributionsRemoved}); })()" % (api, custom_save))
payload = json.loads(removed)
check("registration removed", payload.get("removed") is True, payload)
check("distribution record cleaned", payload.get("d") == 1, payload)

json.dump({"passed": passed, "failed": failed}, open(out_path, "w", encoding="utf-8"))
print("api checks: pass=%d fail=%d" % (len(passed), len(failed)))
PY
API_STATUS=$?

python -I - "$CUSTOM" "$VAULT" "$REPO_BEFORE" "$ROOT/.claude/skills" "$REAL_BEFORE" "$USERPROFILE/.claude/skills" <<'PY'
import hashlib, os, sys

custom, vault, repo_before, repo_root, real_before, real_root = sys.argv[1:7]
failed = 0

def check(name, condition, detail=""):
    global failed
    if condition:
        print("PASS " + name)
    else:
        failed += 1
        print("FAIL " + name + (" :: " + str(detail) if detail else ""))

def hash_tree(root):
    h = hashlib.sha256()
    if os.path.isdir(root):
        for base, _, files in sorted(os.walk(root)):
            for name in sorted(files):
                path = os.path.join(base, name)
                h.update(os.path.relpath(path, root).replace(os.sep, "/").encode())
                h.update(b"\0")
                h.update(open(path, "rb").read())
                h.update(b"\0")
    return h.hexdigest()

def snapshot(root):
    h = hashlib.sha256()
    if os.path.isdir(root):
        for base, dirs, files in os.walk(root):
            dirs.sort()
            for name in sorted(files):
                path = os.path.join(base, name)
                try:
                    st = os.stat(path, follow_symlinks=False)
                    h.update(os.path.relpath(path, root).encode())
                    h.update(str((st.st_size, int(st.st_mtime))).encode())
                except OSError:
                    h.update(b"?")
    return h.hexdigest()

link = os.path.join(custom, "m6-test")
check("link exists after removal", os.path.exists(link))
read_through = ""
try:
    read_through = open(os.path.join(link, "SKILL.md"), encoding="utf-8").read()
except OSError:
    pass
check("reads through the link", "验收用测试技能" in read_through)
check("repository untouched", snapshot(repo_root) == repo_before)
check("real tool dir untouched", snapshot(real_root) == real_before)
raise SystemExit(0 if failed == 0 else 1)
PY
FILE_STATUS=$?

STATUS=0
[ "$API_STATUS" -eq 0 ] || STATUS=1
[ "$FILE_STATUS" -eq 0 ] || STATUS=1
python -I - "$WORK/result.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
print("RESULT api pass=%d fail=%d" % (len(data["passed"]), len(data["failed"])))
sys.exit(0 if not data["failed"] else 1)
PY
[ "$?" -eq 0 ] || STATUS=1

powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1
APP_PID=""
exit "$STATUS"
