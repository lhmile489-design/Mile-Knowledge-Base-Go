#!/usr/bin/env bash
# AC-M7: cross-machine restore on two real windows (PRD §15 FR-SKM-SYNC;
# AC-SKM-20–25). Two temporary APPDATA/HOME pairs simulate machine A and B;
# the vault is copied from A to B; GitHub is the local fake server. The real
# tool directories and the real GitHub are never touched.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.ac-m7-XXXXXX")
VAULT_A="$WORK/vault-a"; VAULT_B="$WORK/vault-b"
APPD_A="$WORK/app-a"; APPD_B="$WORK/app-b"
HOME_A="$WORK/home-a"; HOME_B="$WORK/home-b"
CUSTOM_A="$WORK/a-custom"; SITE_A="$WORK/a-site"; SITE_B="$WORK/b-site"
FIX="$WORK/fixture"
mkdir -p "$VAULT_A" "$APPD_A/Mile Knowledge Base" "$HOME_A/.local/bin" \
  "$VAULT_B" "$APPD_B/Mile Knowledge Base" "$HOME_B/.local/bin" "$HOME_B/.claude/skills/vue" \
  "$CUSTOM_A" "$SITE_A/.claude/skills" "$SITE_B/.claude/skills" "$FIX"
printf 'cmd' > "$HOME_A/.local/bin/claude.cmd"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT_A")" >/dev/null)
mkdir -p "$VAULT_A/skills/前端/vue"
printf '# vue\n\n' > "$VAULT_A/skills/前端/vue/SKILL.md"
printf 'cmd' > "$HOME_B/.local/bin/claude.cmd"
printf '# local edit\n' > "$HOME_B/.claude/skills/vue/SKILL.md"

VID_A=$(python -I - "$VAULT_A" "$APPD_A/Mile Knowledge Base" "ac-m7-a" <<'PY'
import hashlib, json, os, sys
vault, appdir, name = sys.argv[1], sys.argv[2], sys.argv[3]
c = os.path.abspath(vault).replace("/", chr(92)).lower()
vid = hashlib.sha256(c.encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": False,
                  "sidebarCollapsed": False, "agnesControlsCollapsed": False,
                  "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": vault, "displayName": name, "lastOpenedAt": "2026-11-22T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

QUEUE_A="$WORK/pick-a.txt"
python -I - "$QUEUE_A" "$(cygpath -w "$CUSTOM_A")" "$(cygpath -w "$SITE_A")" <<'PY'
import sys
open(sys.argv[1], "w", encoding="utf-8").write("\n".join(sys.argv[2:]) + "\n")
PY

REGISTRY="$WORK/registry.json"
python -I - "$REGISTRY" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"uninstall": []}))
PY

# --- fake GitHub (one archive with skills/pdf) ------------------------------
STATE="$FIX/state.json"
python -I - "$(cygpath -w "$FIX")" "$(cygpath -w "$STATE")" <<'PY'
import io, json, os, sys, tarfile
fix, state_path = sys.argv[1], sys.argv[2]
path = os.path.join(fix, "v1.tar.gz")
with tarfile.open(path, "w:gz") as tar:
    for rel, data in {
        "skills/pdf/SKILL.md": b"# pdf\n\nrestored from github\n",
        "skills/docx/SKILL.md": b"# docx v1\n",
    }.items():
        info = tarfile.TarInfo("repo/" + rel)
        info.size = len(data)
        tar.addfile(info, io.BytesIO(data))
head = "1" * 40
state = {"repos": {"anthropics/skills": {"ref": "main", "head": head,
         "paths": {"skills/pdf": head, "skills/docx": head},
         "archives": {head: path}}}}
with open(state_path, "w", encoding="utf-8") as handle:
    json.dump(state, handle)
PY

GH_PORT=18529
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
  echo "ABORT: 127.0.0.1:$GH_PORT is already listening (stale fake GitHub?)"; exit 2
fi
python -I "$ROOT/scripts/e2e/fake-github.py" "$(cygpath -w "$STATE")" "$GH_PORT" >"$WORK/github.log" 2>&1 &
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

real_snapshot() { python -I - "$1" <<'PY'
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
REAL_CLAUDE_BEFORE=$(real_snapshot "$USERPROFILE/.claude/skills")
REAL_AGENTS_BEFORE=$(real_snapshot "$USERPROFILE/.agents")

hash_file() { python -I - "$1" <<'PY'
import hashlib, sys
try:
    print(hashlib.sha256(open(sys.argv[1], "rb").read()).hexdigest())
except OSError:
    print("missing")
PY
}

DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1
PORT=18527
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  if [ -n "${GH_PID:-}" ]; then powershell -NoProfile -Command "Get-CimInstance Win32_Process | Where-Object { \$_.Name -eq 'python.exe' -and \$_.CommandLine -match 'fake-github' -and \$_.CommandLine -match '$GH_PORT' } | ForEach-Object { Stop-Process -Id \$_.ProcessId -Force -ErrorAction SilentlyContinue }" >/dev/null 2>&1; fi
  sleep 2
  rm -rf "$WORK"
}
trap cleanup EXIT
if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running"; exit 2
fi

start_app() { # appdata home port queue github-base
  PORT=$3
  export MYGO_AUTOMATION="tcp:127.0.0.1:$PORT"
  env APPDATA="$(cygpath -w "$1")" MYGO_AUTOMATION="tcp:127.0.0.1:$3" \
    SKILLS_FAKE_HOME="$(cygpath -w "$2")" \
    SKILLS_FAKE_PATH="$(cygpath -w "$2/.local/bin")" \
    SKILLS_FAKE_REGISTRY="$(cygpath -w "$REGISTRY")" \
    SKILLS_FAKE_PICK_DIR="@$(cygpath -w "$4")" \
    SKILLS_FAKE_GITHUB="$5" "$APP" >"$WORK/app-$3.log" 2>&1 &
  sleep 7
  APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -d '\r')
}
stop_app() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  APP_PID=""
  sleep 2
}
ev() {
  python -I - "$WORK/req.json" "$1" <<'PY'
import json,sys
open(sys.argv[1],"w",encoding="utf-8").write(json.dumps({"id":1,"method":"eval","params":{"window":1,"script":sys.argv[2]}}))
PY
  timeout 60 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
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
fill() {
  ev "(() => { const el=document.querySelector('[data-testid=\"$1\"]'); if(!el) throw new Error('missing'); const setter=Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set; setter.call(el,$(jsq "$2")); el.dispatchEvent(new Event('input',{bubbles:true})); return true; })()" >/dev/null
}
close_banner() { ev "(() => { const b = document.querySelector('[data-testid=\"skills-banner-close\"]'); if (b) b.click(); return true; })()" >/dev/null; }
open_panel() { ev "window.__mkbApp.getState().openSkillsPanel(); true" >/dev/null; wait_match "panel open" "document.querySelector('[data-testid=\"skills-panel\"]') !== null" '"value":true' 20; }
open_tools() { ev "(() => { const tab=[...document.querySelectorAll('[data-testid=\"skills-panel\"] [role=tab]')].find((t)=>t.textContent.includes('工具技能')); if(tab) tab.click(); return true; })()" >/dev/null; sleep 0.5; }
open_kb() { ev "(() => { const tab=[...document.querySelectorAll('[data-testid=\"skills-panel\"] [role=tab]')].find((t)=>t.textContent.includes('知识库技能')); if(tab) tab.click(); return true; })()" >/dev/null; sleep 0.5; }
try_index() { wait_match "index" "!document.body.innerText.includes('未索引')" '"value":true' 90; }
target_id() {
  ev "(() => { const row=[...document.querySelectorAll('[data-testid^=\"target-row-t-\"]')].find((el)=>el.textContent.includes($(jsq "$1"))); return row ? row.getAttribute('data-testid').slice('target-row-'.length) : ''; })()" | grep -o '"value":"[^"]*"' | cut -d'"' -f4
}
card_distribute() {
  ev "(() => { const card=document.querySelector('[data-testid=\"kb-grid-$1\"] [data-testid=\"kb-card-$2\"]'); const b=card && [...card.querySelectorAll('button')].find((x)=>x.textContent.trim()==='分发'); if(!b) throw new Error('missing distribute'); b.click(); return true; })()" >/dev/null
  wait_match "distribute dialog" "document.querySelector('[data-testid=\"distribute-dialog\"]') !== null" '"value":true' 20
}
select_target() {
  ev "(() => { const el=document.querySelector('[data-testid=\"distribute-target-$1\"]'); if(!el) throw new Error('missing target'); if(el.getAttribute('aria-checked') !== 'true') el.click(); return true; })()" >/dev/null
}
set_mode() { # link | copy → click the mode radio by its label
  local label=$1
  ev "(() => { const b=[...document.querySelectorAll('[data-testid=\"distribute-dialog\"] [role=radio]')].find((x)=>x.textContent.includes($(jsq "$label"))); if(!b) throw new Error('missing mode'); b.click(); return true; })()" >/dev/null
  sleep 0.3
}
submit_distribute() {
  click '[data-testid="distribute-submit"]' >/dev/null
  wait_match "distributed" "document.querySelector('[data-testid=\"skills-banner\"]')?.textContent.includes('已')" '"value":true' 60
  close_banner
}
open_restore() {
  click '[data-testid="restore-banner-open"]'
  wait_match "restore view" "document.querySelector('[data-testid=\"restore-view\"]') !== null" '"value":true' 30
}
manifest_path() { echo "$1/.mile-kb/skill-distributions.json"; }
manifest_report() { python -I - "$(manifest_path "$1")" <<'PY'
import json, sys
path = sys.argv[1]
try:
    doc = json.load(open(path, encoding="utf-8"))
except Exception as exc:
    print("error", exc)
    raise SystemExit(0)
items = doc.get("items", [])
print(json.dumps({
    "version": doc.get("version"),
    "count": len(items),
    "items": [
        {"source": item.get("source"), "name": item.get("name"),
         "target": item.get("target"), "mode": item.get("mode")}
        for item in items
    ],
    "raw": open(path, encoding="utf-8").read(),
}, ensure_ascii=False))
PY
}

# =========================== machine A ======================================
start_app "$APPD_A" "$HOME_A" 18527 "$QUEUE_A" "http://127.0.0.1:$GH_PORT"
wait_match "A overview" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
try_index
open_panel
open_tools
click '[data-testid="manage-targets"]'
wait_match "A targets view" "document.querySelector('[data-testid=\"targets-view\"]') !== null" '"value":true' 20
click '[data-testid="targets-add-custom"]'
wait_match "A custom form" "document.querySelector('[data-testid=\"target-name\"]') !== null" '"value":true' 20
fill target-name "Droid"
click '[data-testid="target-pick"]'
wait_match "A custom picked" "document.querySelector('[data-testid=\"target-picked\"]')?.textContent.includes('a-custom')" '"value":true' 20
click '[data-testid="target-save"]'
wait_match "A custom row" "document.querySelector('[data-testid^=\"target-row-t-\"]') !== null" '"value":true' 30
CUSTOM_TID=$(target_id "Droid")
click '[data-testid="targets-add-project"]'
wait_match "A project form" "document.querySelector('[data-testid=\"target-name\"]') !== null" '"value":true' 20
fill target-name "Site"
click '[data-testid="target-pick"]'
wait_match "A project picked" "document.querySelector('[data-testid=\"target-dirs\"]')?.textContent.includes('.claude/skills')" '"value":true' 20
click '[data-testid="target-install-.claude/skills"]'
click '[data-testid="target-save"]'
wait_match "A project row" "[...document.querySelectorAll('[data-testid^=\"target-row-t-\"]')].some((el)=>el.textContent.includes('Site'))" '"value":true' 30
PROJECT_TID=$(target_id "Site")
click '[data-testid="targets-back"]'
sleep 0.5
[ -n "$CUSTOM_TID" ] && [ -n "$PROJECT_TID" ] && { pass=$((pass+1)); echo "PASS A registrations"; } || { fail=$((fail+1)); echo "FAIL A registrations"; }

# Link vue to Claude Code, copy it to the project and the custom tool.
open_kb
wait_match "A vue card" "document.querySelector('[data-testid=\"kb-card-vue\"]') !== null" '"value":true' 40
card_distribute "前端" "vue"
select_target "claude-code"
check "A link target selected" "document.querySelector('[data-testid=\"distribute-summary\"]').textContent.includes('已选 1')" '"value":true'
click '[data-testid="distribute-submit"]' >/dev/null
wait_match "A link distributed" "document.querySelector('[data-testid=\"skills-banner\"]')?.textContent.includes('已联接')" '"value":true' 60
close_banner
card_distribute "前端" "vue"
select_target "$PROJECT_TID"
select_target "$CUSTOM_TID"
set_mode "拷贝"
submit_distribute

# GitHub skill straight into Claude Code.
click '.skills-install'
wait_match "A install view" "document.querySelector('[data-testid=\"install-github\"]') !== null" '"value":true' 30
click '[data-testid="install-github"]' >/dev/null
fill install-github-input "anthropics/skills"
click '[data-testid="install-github-read"]' >/dev/null
wait_match "A read settled" "document.querySelector('[data-testid=\"install-candidate-skills/pdf\"]') !== null" '"value":true' 80
click '[data-testid="install-candidate-skills/docx"]'
click '[data-testid="install-target-tools"]'
wait_match "A tool targets" "document.querySelector('[data-testid=\"install-tool-claude-code\"]') !== null" '"value":true' 20
click '[data-testid="install-tool-claude-code"]'
wait_match "A claude target on" "document.querySelector('[data-testid=\"install-tool-claude-code\"]')?.getAttribute('aria-checked') === 'true'" '"value":true' 20
click '[data-testid="install-submit"]' >/dev/null
wait_match "A install result" "document.querySelector('[data-testid=\"install-result\"]') !== null" '"value":true' 80
echo "A install result: $(ev "(() => { const el=document.querySelector('[data-testid=\"install-result\"]'); return el ? el.textContent.trim() : ''; })()")"
check "A installed" "document.querySelector('[data-testid=\"install-result\"]').textContent.includes('pdf → Claude Code：已安装')" '"value":true' 

REPORT_A=$(manifest_report "$VAULT_A")
echo "A manifest: $REPORT_A"
echo "$REPORT_A" | grep -q '"count": 4' && { pass=$((pass+1)); echo "PASS AC-SKM-20 four manifest items"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-20 item count"; }
echo "$REPORT_A" | grep -q '"kind": "project"' && echo "$REPORT_A" | grep -q '"subdir": ".claude/skills"' && { pass=$((pass+1)); echo "PASS AC-SKM-20 project target"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-20 project target"; }
echo "$REPORT_A" | grep -q '"source": "github"' && { pass=$((pass+1)); echo "PASS AC-SKM-20 github item"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-20 github item"; }
LEAK=$(python -I - "$(manifest_path "$VAULT_A")" "$(cygpath -w "$WORK")" <<'PY'
import json, os, sys
text = open(sys.argv[1], encoding="utf-8").read()
work = sys.argv[2].replace("/", chr(92)).lower()
problems = []
for needle in [work, os.environ.get("USERNAME", ""), "t-", ":\\\\", "Users"]:
    if needle and needle.lower() in text.lower():
        problems.append(needle)
print(",".join(problems))
PY
)
[ -z "$LEAK" ] && { pass=$((pass+1)); echo "PASS AC-SKM-20 manifest leaks nothing"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-20 manifest leaks :: $LEAK"; }
stop_app

# Copy the vault (manifest included) to machine B.
cp -a "$VAULT_A/." "$VAULT_B/"

# =========================== machine B ======================================
VID_B=$(python -I - "$VAULT_B" "$APPD_B/Mile Knowledge Base" "ac-m7-b" <<'PY'
import hashlib, json, os, sys
vault, appdir, name = sys.argv[1], sys.argv[2], sys.argv[3]
c = os.path.abspath(vault).replace("/", chr(92)).lower()
vid = hashlib.sha256(c.encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": False,
                  "sidebarCollapsed": False, "agnesControlsCollapsed": False,
                  "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": vault, "displayName": name, "lastOpenedAt": "2026-11-22T10:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)
QUEUE_B="$WORK/pick-b.txt"
python -I - "$QUEUE_B" "$(cygpath -w "$SITE_B")" <<'PY'
import sys
open(sys.argv[1], "w", encoding="utf-8").write("\n".join(sys.argv[2:]) + "\n")
PY

start_app "$APPD_B" "$HOME_B" 18528 "$QUEUE_B" "http://127.0.0.1:$GH_PORT"
wait_match "B overview" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
try_index
open_panel
open_tools
click '[data-testid="manage-targets"]'
wait_match "B targets view" "document.querySelector('[data-testid=\"targets-view\"]') !== null" '"value":true' 20
click '[data-testid="targets-add-project"]'
wait_match "B project form" "document.querySelector('[data-testid=\"target-name\"]') !== null" '"value":true' 20
fill target-name "Site"
click '[data-testid="target-pick"]'
wait_match "B project picked" "document.querySelector('[data-testid=\"target-dirs\"]')?.textContent.includes('.claude/skills')" '"value":true' 20
click '[data-testid="target-install-.claude/skills"]'
click '[data-testid="target-save"]'
wait_match "B project row" "[...document.querySelectorAll('[data-testid^=\"target-row-t-\"]')].some((el)=>el.textContent.includes('Site'))" '"value":true' 30
click '[data-testid="targets-back"]'
sleep 0.5
close_banner

# --- AC-SKM-21/22/23: banner, groups and the first restore -----------------
wait_match "AC-SKM-21 banner" "document.querySelector('[data-testid=\"restore-banner\"]')?.textContent.includes('3 项尚未在本机还原')" '"value":true' 40
open_restore
wait_match "AC-SKM-21 groups" "document.querySelector('[data-testid=\"restore-group-create\"]') !== null" '"value":true' 30
check "AC-SKM-21 conflict group" "document.querySelector('[data-testid=\"restore-group-conflict\"]') !== null" '"value":true'
check "AC-SKM-21 skip group" "document.querySelector('[data-testid=\"restore-group-skip\"]')?.textContent.includes('Droid')" '"value":true'
check "AC-SKM-21 selection count" "document.querySelector('[data-testid=\"restore-count\"]').textContent.includes('已选 2 项') && document.querySelector('[data-testid=\"restore-count\"]').textContent.includes('1 项需要联网')" '"value":true'
MANIFEST_B_BEFORE=$(hash_file "$(manifest_path "$VAULT_B")")
check "AC-SKM-22 conflict stays unchecked" "document.querySelector('[data-testid=\"restore-group-conflict\"] [data-testid^=\"restore-check-r-\"]')?.checked === false" '"value":true'
click '[data-testid="restore-run"]'
wait_match "AC-SKM-21 result" "document.querySelector('[data-testid=\"restore-result\"]')?.textContent.includes('已还原 2 项')" '"value":true' 120
check "AC-SKM-23 unmatched listed" "document.querySelector('[data-testid=\"restore-unmatched\"]')?.textContent.includes('Droid')" '"value":true'
[ -f "$SITE_B/.claude/skills/vue/SKILL.md" ] && { pass=$((pass+1)); echo "PASS AC-SKM-21 project copy restored"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-21 project copy missing"; }
[ -f "$HOME_B/.claude/skills/pdf/SKILL.md" ] && { pass=$((pass+1)); echo "PASS AC-SKM-21 github skill restored"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-21 github skill missing"; }
[ "$(cat "$HOME_B/.claude/skills/vue/SKILL.md")" = "# local edit" ] && { pass=$((pass+1)); echo "PASS AC-SKM-22 conflict dir untouched"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-22 conflict dir changed"; }
[ "$(hash_file "$(manifest_path "$VAULT_B")")" = "$MANIFEST_B_BEFORE" ] && { pass=$((pass+1)); echo "PASS restore leaves the manifest alone"; } || { fail=$((fail+1)); echo "FAIL restore rewrote the manifest"; }

# --- AC-SKM-22: check the conflict item and overwrite with a backup ---------
click '[data-testid="restore-done"]'
sleep 0.5
wait_match "AC-SKM-22 banner one left" "document.querySelector('[data-testid=\"restore-banner\"]')?.textContent.includes('1 项尚未在本机还原')" '"value":true' 40
open_restore
CONFLICT_ID=$(ev "(() => { const box=document.querySelector('[data-testid=\"restore-group-conflict\"] [data-testid^=\"restore-check-r-\"]'); return box ? box.getAttribute('data-testid').replace('restore-check-','') : ''; })()" | grep -o '"value":"[^"]*"' | cut -d'"' -f4)
ev "(() => { const box=document.querySelector('[data-testid=\"restore-check-$CONFLICT_ID\"]'); if(!box) throw new Error('missing'); box.click(); return true; })()" >/dev/null
click '[data-testid="restore-run"]'
wait_match "AC-SKM-22 overwrite result" "document.querySelector('[data-testid=\"restore-result\"]')?.textContent.includes('已还原 1 项')" '"value":true' 60
check "AC-SKM-22 backup noted" "document.querySelector('[data-testid=\"restore-result\"]').textContent.includes('可从操作记录恢复')" '"value":true'
grep -q "vue" "$HOME_B/.claude/skills/vue/SKILL.md" && { pass=$((pass+1)); echo "PASS AC-SKM-22 overwrite wrote the vault copy"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-22 overwrite content"; }
click '[data-testid="restore-done"]'
sleep 0.5

# --- AC-SKM-24: corrupt manifest, banner, rebuild ---------------------------
ev "window.__mkbApp.getState().closeSkillsPanel(); true" >/dev/null
python -I - "$(manifest_path "$VAULT_B")" <<'PY'
import sys
path = sys.argv[1]
open(path, "w", encoding="utf-8").write("<<<<<<< HEAD\n{\"version\":1}\n=======\n{\"version\":1}\n>>>>>>> other\n")
PY
open_panel
open_tools
wait_match "AC-SKM-24 unreadable banner" "document.querySelector('[data-testid=\"restore-banner\"]')?.textContent.includes('无法读取')" '"value":true' 40
check "AC-SKM-24 warn tone" "document.querySelector('[data-testid=\"restore-banner\"]').getAttribute('data-tone') === 'warn'" '"value":true'
open_restore
wait_match "AC-SKM-24 restore status" "document.querySelector('[data-testid=\"restore-rebuild\"]') !== null" '"value":true' 20
click '[data-testid="restore-rebuild"]'
wait_match "AC-SKM-24 confirm" "document.querySelector('[data-testid=\"restore-rebuild-dialog\"]') !== null" '"value":true' 20
click '[data-testid="restore-rebuild-confirm"]'
wait_match "AC-SKM-24 rebuilt" "document.querySelector('[data-testid=\"skills-banner\"]')?.textContent.includes('已重建分发清单')" '"value":true' 60
check "AC-SKM-24 corrupt copy kept" "document.querySelector('[data-testid=\"skills-banner\"]').textContent.includes('corrupt-')" '"value":true'
close_banner
if ls "$VAULT_B/.mile-kb/" | grep -q "skill-distributions.json.corrupt-"; then
  pass=$((pass+1)); echo "PASS AC-SKM-24 corrupt copy on disk"
else
  fail=$((fail+1)); echo "FAIL AC-SKM-24 corrupt copy missing"
fi
grep -q '"version": 1' "$(manifest_path "$VAULT_B")" && { pass=$((pass+1)); echo "PASS AC-SKM-24 manifest readable again"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-24 manifest not rebuilt"; }
ev "(() => { const b=document.querySelector('[data-testid=\"restore-back\"]'); if (b) b.click(); return true; })()" >/dev/null
sleep 0.5

# --- AC-SKM-25: the restored GitHub skill keeps its source ------------------
open_tools
wait_match "AC-SKM-25 pdf row" "document.querySelector('[data-testid=\"tool-skill-pdf-claude-code\"]') !== null" '"value":true' 40
check "AC-SKM-25 github source pill" "document.querySelector('[data-testid=\"tool-skill-pdf-claude-code\"]').textContent.includes('GitHub')" '"value":true'
stop_app

# --- real tool directories unchanged ---------------------------------------
REAL_CLAUDE_AFTER=$(real_snapshot "$USERPROFILE/.claude/skills")
REAL_AGENTS_AFTER=$(real_snapshot "$USERPROFILE/.agents")
[ "$REAL_CLAUDE_BEFORE" = "$REAL_CLAUDE_AFTER" ] && { pass=$((pass+1)); echo "PASS real .claude untouched"; } || { fail=$((fail+1)); echo "FAIL real .claude changed"; }
[ "$REAL_AGENTS_BEFORE" = "$REAL_AGENTS_AFTER" ] && { pass=$((pass+1)); echo "PASS real .agents untouched"; } || { fail=$((fail+1)); echo "FAIL real .agents changed"; }

echo "RESULT pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
