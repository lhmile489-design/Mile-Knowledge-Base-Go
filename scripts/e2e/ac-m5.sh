#!/usr/bin/env bash
# AC-M5: install sources and updates on a real window (PRD §15; AC-SKM-12–16).
# Uses a fake HOME and a local fake GitHub (SKILLS_FAKE_GITHUB, e2e build only):
# the real GitHub and the real tool directories are never contacted.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.ac-m5-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"; FIX="$WORK/fixture"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base" "$HOME_DIR" "$FIX"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null)

VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge Base" "$HOME_DIR" <<'PY'
import hashlib, json, os, sys
vault, appdir, home = sys.argv[1], sys.argv[2], sys.argv[3]
c = os.path.abspath(vault).replace("/", chr(92)).lower()
vid = hashlib.sha256(c.encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": False,
                  "sidebarCollapsed": False, "agnesControlsCollapsed": False,
                  "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": vault, "displayName": "ac-m5", "lastOpenedAt": "2026-11-21T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

# --- fake GitHub fixture ----------------------------------------------------
STATE="$FIX/state.json"
python -I - "$(cygpath -w "$FIX")" "$(cygpath -w "$STATE")" <<'PY'
import io, json, os, sys, tarfile
fix, state_path = sys.argv[1], sys.argv[2]

def archive(name, files, evil=False):
    path = os.path.join(fix, name)
    with tarfile.open(path, "w:gz") as tar:
        for rel, data in files.items():
            info = tarfile.TarInfo("repo/" + rel)
            info.size = len(data)
            tar.addfile(info, io.BytesIO(data))
        if evil:
            data = b"evil\n"
            info = tarfile.TarInfo("../evil/SKILL.md")
            info.size = len(data)
            tar.addfile(info, io.BytesIO(data))
    return path

v1 = archive("v1.tar.gz", {
    "skills/pdf/SKILL.md": b"# pdf\n\nversion one\n",
    "skills/pdf/reference/old.md": b"old\n",
    "skills/pdf/assets/icon.bin": bytes([255, 1]),
    "skills/docx/SKILL.md": b"# docx v1\n",
})
v2 = archive("v2.tar.gz", {
    "skills/pdf/SKILL.md": b"# pdf\n\nversion two\n",
    "skills/pdf/reference/new.md": b"new\n",
    "skills/pdf/assets/icon.bin": bytes([255, 2]),
    "skills/docx/SKILL.md": b"# docx v1\n",
})
evil = archive("evil.tar.gz", {"SKILL.md": b"# evil\n"}, evil=True)
h1 = "1" * 40
state = {"repos": {
    "anthropics/skills": {"ref": "main", "head": h1,
                          "paths": {"skills/pdf": h1, "skills/docx": h1},
                          "archives": {h1: v1}},
    "acme/evil": {"ref": "main", "head": h1, "paths": {"": h1}, "archives": {h1: evil}},
}}
with open(state_path, "w", encoding="utf-8") as handle:
    json.dump(state, handle)
PY
set_state() { # head archive
  python -I - "$STATE" "$1" "$2" <<'PY'
import json, sys
path, head, archive = sys.argv[1], sys.argv[2], sys.argv[3]
state = json.load(open(path, encoding="utf-8"))
repo = state["repos"]["anthropics/skills"]
repo["head"] = head
repo["paths"] = {"skills/pdf": head, "skills/docx": head}
repo["archives"] = {head: archive} if archive else {}
json.dump(state, open(path, "w", encoding="utf-8"))
PY
}

GH_PORT=18513
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
  echo "ABORT: 127.0.0.1:$GH_PORT is already listening (stale fake GitHub?)"
  exit 2
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

DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1
PORT=18512
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  if [ -n "${GH_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $GH_PID -Force" >/dev/null 2>&1; fi
  powershell -NoProfile -Command "Get-CimInstance Win32_Process | Where-Object { \$_.Name -eq 'python.exe' -and \$_.CommandLine -match 'fake-github' -and \$_.CommandLine -match '18513' } | ForEach-Object { Stop-Process -Id \$_.ProcessId -Force -ErrorAction SilentlyContinue }" >/dev/null 2>&1
  sleep 2
  if [ "${fail:-0}" -ne 0 ]; then
    echo "WORK kept for diagnosis: $WORK"
  else
    rm -rf "$WORK"
  fi
}
trap cleanup EXIT
if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running"; exit 2
fi

w() { mkdir -p "$(dirname "$1")"; printf '%b' "$2" > "$1"; }
w "$HOME_DIR/.local/bin/claude.cmd" "@echo off"
hash_dir() { python -I - "$1" <<'PY'
import hashlib, os, sys
root = sys.argv[1]
h = hashlib.sha256()
if os.path.isdir(root):
    for base, _, files in sorted(os.walk(root)):
        for name in sorted(files):
            path = os.path.join(base, name)
            h.update(os.path.relpath(path, root).replace(os.sep, "/").encode())
            h.update(b"\0")
            h.update(open(path, "rb").read())
            h.update(b"\0")
print(h.hexdigest())
PY
}
stage_count() { python -I - "$1" <<'PY'
import os, sys
root = sys.argv[1]
print(len([n for n in os.listdir(root) if n.startswith("s-")]) if os.path.isdir(root) else 0)
PY
}
source_commit() { python -I - "$VAULT" <<'PY'
import json, os, sys
root = sys.argv[1]
path = os.path.join(root, ".mile-kb", "skill-sources.json")
records = json.load(open(path, encoding="utf-8"))["records"] if os.path.exists(path) else []
print(next((r.get("pathCommit", "") for r in records if r.get("skillPath") == "skills/文档/pdf"), ""))
PY
}
icon_bytes_ok() { python -I - "$1" <<'PY'
import sys
print("yes" if open(sys.argv[1], "rb").read() == bytes([255, 2]) else "no")
PY
}

start_app() { # fake github base
  env APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" \
    SKILLS_FAKE_HOME="$(cygpath -w "$HOME_DIR")" \
    SKILLS_FAKE_PATH="$(cygpath -w "$HOME_DIR/.local/bin")" \
    SKILLS_FAKE_GITHUB="$1" "$APP" >"$WORK/app.log" 2>&1 &
  sleep 7
  APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -d '\r')
}
stop_app() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  APP_PID=""
  sleep 2
}
export MYGO_AUTOMATION="tcp:127.0.0.1:$PORT"
ev() {
  python -I - "$WORK/req.json" "$1" <<'PY'
import json,sys
open(sys.argv[1],"w",encoding="utf-8").write(json.dumps({"id":1,"method":"eval","params":{"window":1,"script":sys.argv[2]}}))
PY
  timeout 90 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}
pass=0; fail=0
check() {
  local out; out=$(ev "$2")
  if echo "$out" | grep -q "$3"; then pass=$((pass+1)); echo "PASS $1"; else fail=$((fail+1)); echo "FAIL $1 :: $out"; fi
}
wait_match() {
  local label=$1 script=$2 pattern=$3 tries=${4:-40} i out
  for i in $(seq 1 "$tries"); do
    out=$(ev "$script")
    if echo "$out" | grep -q "$pattern"; then pass=$((pass+1)); echo "PASS $label"; return 0; fi
    sleep 0.5
  done
  fail=$((fail+1)); echo "FAIL $label :: $out"
}
jsq() { python -I -c "import json,sys;print(json.dumps(sys.argv[1]))" "$1"; }
click() { ev "(() => { const el = document.querySelector($(jsq "$1")); if (!el) throw new Error('missing'); el.click(); return true; })()"; }
fill() { # testid value
  ev "(() => { const el=document.querySelector('[data-testid=\"$1\"]'); if(!el) throw new Error('missing'); const setter=Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set; setter.call(el,$(jsq "$2")); el.dispatchEvent(new Event('input',{bubbles:true})); return true; })()" >/dev/null
}
open_panel() { ev "window.__mkbApp.getState().showPage('skills'); true" >/dev/null; wait_match "skills page open" "document.querySelector('[data-testid=\"skills-page\"]') !== null" '"value":true' 20; }
open_install() {
  ev "(() => { const b=document.querySelector('[data-testid=\"kb-install-top\"]'); if(!b) throw new Error('missing install'); b.click(); return true; })()" >/dev/null
  wait_match "install view" "document.querySelector('[data-testid=\"install-github\"]') !== null" '"value":true' 30
}
github_read() { # address wait-label
  local address=$1 label=$2
  # After an install the dialog shows the result step; 继续安装 returns to the source step (M9).
  ev "(() => { const again=document.querySelector('[data-testid=\"install-again\"]'); if (again) again.click(); return true; })()" >/dev/null
  sleep 0.3
  ev "(() => { if (!document.querySelector('[data-testid=\"install-github-form\"]')) { const b=document.querySelector('[data-testid=\"install-github\"]'); if (b) b.click(); } return true; })()" >/dev/null
  sleep 0.3
  fill install-github-input "$address"
  click '[data-testid="install-github-read"]' >/dev/null
  wait_match "$label read settled" "document.querySelector('[data-testid=\"install-error\"]') !== null || document.querySelector('[data-testid=\"install-input-problem\"]') !== null || document.querySelector('[data-testid=\"install-candidate-skills/pdf\"]') !== null" '"value":true' 80
}
refresh_updates() {
  ev "window.__mkbApi.skills.checkUpdates('$VID', true).then((r)=>JSON.stringify(r))" >/dev/null
  wait_match "check round done" "window.__mkbApi.skills.checkUpdates('$VID', false).then((r)=>!r.checking)" '"value":true' 80
  ev "window.__mkbSkills.getState().loadScan('$VID'); true" >/dev/null
  sleep 1
}
try_index() { wait_match "index" "!document.body.innerText.includes('未索引')" '"value":true' 90; }

# --- AC-SKM-12: a multi-skill repository installs only the checked skills ----
start_app "http://127.0.0.1:$GH_PORT"
wait_match "overview" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
try_index
open_panel
open_install
click '[data-testid="install-github"]' >/dev/null
fill install-github-input "anthropics/skills"
click '[data-testid="install-github-read"]' >/dev/null
wait_match "AC-SKM-12 read settled" "document.querySelector('[data-testid=\"install-error\"]') !== null || document.querySelector('[data-testid=\"install-input-problem\"]') !== null || document.querySelector('[data-testid=\"install-candidate-skills/pdf\"]') !== null" '"value":true' 80
echo "AC-SKM-12 read state: $(ev "(() => { const e=document.querySelector('[data-testid=\"install-error\"]'); const p=document.querySelector('[data-testid=\"install-input-problem\"]'); return e ? e.textContent.trim() : (p ? p.textContent.trim() : 'candidates'); })()")"
check "AC-SKM-12 candidates" "document.querySelector('[data-testid=\"install-candidate-skills/pdf\"]') !== null && document.querySelector('[data-testid=\"install-candidate-skills/docx\"]') !== null" '"value":true'
check "AC-SKM-12 both preselected" "document.querySelector('[data-testid=\"install-candidate-skills/docx\"]').checked" '"value":true'
click '[data-testid="install-candidate-skills/docx"]'
fill install-category "文档"
click '[data-testid="install-submit"]' >/dev/null
wait_match "AC-SKM-12 installed" "document.querySelector('[data-testid=\"install-result\"]') !== null" '"value":true' 80
check "AC-SKM-12 result line" "document.querySelector('[data-testid=\"install-result\"]').textContent.includes('pdf → 知识库：已安装')" '"value":true'
[ -f "$VAULT/skills/文档/pdf/SKILL.md" ] && { pass=$((pass+1)); echo "PASS AC-SKM-12 selected skill written"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-12 selected skill missing"; }
[ ! -e "$VAULT/skills/文档/docx" ] && { pass=$((pass+1)); echo "PASS AC-SKM-12 unchecked skill skipped"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-12 unchecked skill installed"; }
[ "$(source_commit)" = "1111111111111111111111111111111111111111" ] && { pass=$((pass+1)); echo "PASS AC-SKM-12 source record"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-12 source record :: $(source_commit)"; }

# --- AC-SKM-13: an unsafe archive is rejected without residue ----------------
KB_BEFORE=$(hash_dir "$VAULT/skills")
github_read "acme/evil" "AC-SKM-13"
wait_match "AC-SKM-13 archive error" "document.querySelector('[data-testid=\"install-error\"]') !== null" '"value":true' 20
check "AC-SKM-13 fault text" "document.querySelector('[data-testid=\"install-error\"]').textContent.includes('不安全的路径')" '"value":true'
check "AC-SKM-13 offending entry" "document.querySelector('[data-testid=\"install-error\"]').textContent.includes('../evil/SKILL.md')" '"value":true'
[ "$(hash_dir "$VAULT/skills")" = "$KB_BEFORE" ] && { pass=$((pass+1)); echo "PASS AC-SKM-13 vault unchanged"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-13 vault changed"; }
[ "$(stage_count "$APPD/Mile Knowledge Base/skills/tmp")" = "0" ] && { pass=$((pass+1)); echo "PASS AC-SKM-13 no stage residue"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-13 stage residue"; }

# --- AC-SKM-14: no network means no writes -----------------------------------
stop_app
start_app "http://127.0.0.1:1"
wait_match "offline overview" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
try_index
KB_BEFORE=$(hash_dir "$VAULT/skills")
REC_BEFORE=$(source_commit)
open_panel
open_install
click '[data-testid="install-github"]' >/dev/null
fill install-github-input "anthropics/skills"
click '[data-testid="install-github-read"]' >/dev/null
wait_match "AC-SKM-14 network error" "document.querySelector('[data-testid=\"install-error\"]')?.textContent.includes('连不上 GitHub')" '"value":true' 80
[ "$(hash_dir "$VAULT/skills")" = "$KB_BEFORE" ] && { pass=$((pass+1)); echo "PASS AC-SKM-14 vault unchanged"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-14 vault changed"; }
[ "$(source_commit)" = "$REC_BEFORE" ] && { pass=$((pass+1)); echo "PASS AC-SKM-14 source record unchanged"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-14 source record changed"; }
[ "$(stage_count "$APPD/Mile Knowledge Base/skills/tmp")" = "0" ] && { pass=$((pass+1)); echo "PASS AC-SKM-14 no stage residue"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-14 stage residue"; }
stop_app

# --- AC-SKM-15: update through the plan card, recoverable --------------------
set_state "2222222222222222222222222222222222222222" "$(cygpath -w "$FIX/v2.tar.gz")"
start_app "http://127.0.0.1:$GH_PORT"
wait_match "update overview" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
try_index
open_panel
wait_match "scan settles" "!document.querySelector('[data-testid=\"skills-status\"]')?.textContent.includes('后台扫描中')" '"value":true' 60
refresh_updates
wait_match "AC-SKM-15 update pill" "[...document.querySelectorAll('[data-testid=\"kb-card-pdf\"] .pill')].some((p)=>p.textContent.includes('GitHub 有更新'))" '"value":true' 40
click '[data-testid="kb-update-pdf"]' >/dev/null
wait_match "AC-SKM-15 preview" "document.querySelector('[data-testid=\"update-files\"]') !== null" '"value":true' 80
check "AC-SKM-15 text row" "document.querySelector('[data-testid=\"update-files\"]').textContent.includes('SKILL.md')" '"value":true'
check "AC-SKM-15 binary row" "document.querySelector('[data-testid=\"update-files\"]').textContent.includes('assets/icon.bin') && document.querySelector('[data-testid=\"update-files\"]').textContent.includes('二进制')" '"value":true'
check "AC-SKM-15 removed row" "document.querySelector('[data-testid=\"update-files\"]').textContent.includes('old.md')" '"value":true'
click '[data-testid="update-propose"]' >/dev/null
wait_match "AC-SKM-15 plan card" "document.querySelector('.plan-card') !== null" '"value":true' 30
check "AC-SKM-15 plan importFile" "document.querySelector('.plan-card').textContent.includes('导入文件')" '"value":true'
check "AC-SKM-15 plan trash" "document.querySelector('.plan-card').textContent.includes('移到回收站')" '"value":true'
ev "(() => { const b=[...document.querySelectorAll('.plan-card button')].find((x)=>x.textContent.trim()==='执行'); if(!b) throw new Error('missing execute'); b.click(); return true; })()" >/dev/null
wait_match "AC-SKM-15 executed" "(window.__mkbMutation.getState().report||{}).status||''" '"value":"completed"' 80
wait_match "AC-SKM-15 latest pill" "[...document.querySelectorAll('[data-testid=\"kb-card-pdf\"] .pill')].some((p)=>p.textContent.includes('最新'))" '"value":true' 80
grep -q "version two" "$VAULT/skills/文档/pdf/SKILL.md" && { pass=$((pass+1)); echo "PASS AC-SKM-15 text updated"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-15 text not updated"; }
[ -f "$VAULT/skills/文档/pdf/reference/new.md" ] && { pass=$((pass+1)); echo "PASS AC-SKM-15 file added"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-15 file missing"; }
[ ! -e "$VAULT/skills/文档/pdf/reference/old.md" ] && { pass=$((pass+1)); echo "PASS AC-SKM-15 file removed"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-15 file still there"; }
[ "$(icon_bytes_ok "$VAULT/skills/文档/pdf/assets/icon.bin")" = "yes" ] && { pass=$((pass+1)); echo "PASS AC-SKM-15 binary updated"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-15 binary wrong"; }
[ "$(source_commit)" = "2222222222222222222222222222222222222222" ] && { pass=$((pass+1)); echo "PASS AC-SKM-15 record moved"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-15 record :: $(source_commit)"; }
check "AC-SKM-15 recoverable" "window.__mkbApi.mutation.rewindPrepare('$VID', window.__mkbMutation.getState().report.transactionId).then((r)=>String(r.entries.every((e)=>e.restorable)))" '"value":"true"'
check "AC-SKM-15 transaction has entries" "window.__mkbApi.mutation.rewindPrepare('$VID', window.__mkbMutation.getState().report.transactionId).then((r)=>String(r.entries.length))" '"value":"[1-9]'

# --- AC-SKM-16: both sides changed, no one-click update ----------------------
STAGES_BEFORE=$(stage_count "$APPD/Mile Knowledge Base/skills/tmp")
printf '\nlocal edit\n' >> "$VAULT/skills/文档/pdf/SKILL.md"
set_state "3333333333333333333333333333333333333333" ""
refresh_updates
wait_match "AC-SKM-16 both changed" "[...document.querySelectorAll('[data-testid=\"kb-card-pdf\"] .pill')].some((p)=>p.textContent.includes('两边都改了'))" '"value":true' 40
check "AC-SKM-16 no update button" "document.querySelector('[data-testid=\"kb-update-pdf\"]') === null" '"value":true'
check "AC-SKM-16 api refuses" "window.__mkbApi.skills.updatePreview('$VID',{kind:'kb',skillPath:'skills/文档/pdf'}).then(()=> 'unexpected').catch((e)=>String(e.code||e.message))" 'skill-conflict'
[ "$(source_commit)" = "2222222222222222222222222222222222222222" ] && { pass=$((pass+1)); echo "PASS AC-SKM-16 record unchanged"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-16 record moved"; }
[ "$(stage_count "$APPD/Mile Knowledge Base/skills/tmp")" = "$STAGES_BEFORE" ] && { pass=$((pass+1)); echo "PASS AC-SKM-16 no new stage"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-16 new stage left"; }

stop_app
echo "RESULT pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
