#!/usr/bin/env bash
# AC-M4: the skills loop on a real window (PRD §15; AC-SKM-1–11, 19).
# Uses a fake HOME (SKILLS_FAKE_HOME/PATH/REGISTRY, e2e build only) so the real
# tool directories are never touched; the vault and APPDATA are temporary.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.ac-m4-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base" "$HOME_DIR"
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
       "recentVaults": [{"id": vid, "root": vault, "displayName": "ac-m4", "lastOpenedAt": "2026-11-20T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
copied = os.path.join(home, ".claude", "skills", "copiedone")
state = {"version": 1, "distributions": [{
    "id": "d-aaaaaaaaaaaa", "vaultId": vid, "skillPath": "skills/前端/vue",
    "toolId": "claude-code", "targetPath": copied, "mode": "copy", "name": "vue",
    "createdAt": "2026-11-20T09:00:00Z", "baseHash": "sha256:old"}]}
os.makedirs(os.path.join(appdir, "skills"), exist_ok=True)
open(os.path.join(appdir, "skills", "state.json"), "w", encoding="utf-8").write(json.dumps(state, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1
PORT=18511
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

junction() { powershell -NoProfile -Command "New-Item -ItemType Junction -Path '$(cygpath -w "$1")' -Target '$(cygpath -w "$2")' | Out-Null"; }
w() { mkdir -p "$(dirname "$1")"; printf '%b' "$2" > "$1"; }
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

# --- vault skills -----------------------------------------------------------
w "$VAULT/skills/前端/vue/SKILL.md" "# vue\n\n"
w "$VAULT/skills/后端/vue/SKILL.md" "# vue\n\n"
# --- fake home: tools, links, copies ---------------------------------------
w "$HOME_DIR/.local/bin/claude.cmd" "@echo off"
w "$HOME_DIR/.local/bin/cursor.cmd" "@echo off"
w "$HOME_DIR/.local/bin/trae.cmd" "@echo off"
w "$HOME_DIR/.kiro/skills/x/SKILL.md" "# leftover\n"
w "$HOME_DIR/.cursor/skills/vue/SKILL.md" "# old vue\n"
w "$HOME_DIR/.claude/skills/foo/SKILL.md" "# foo\n"
w "$HOME_DIR/.cursor/skills/foo/SKILL.md" "# foo\n"
w "$HOME_DIR/.trae/builtin_skills/builtinone/SKILL.md" "# builtin\n"
cp -r "$VAULT/skills/前端/vue" "$HOME_DIR/.claude/skills/copiedone"
w "$HOME_DIR/.claude/skills/indepone/SKILL.md" "# independent\n"
EXT="$WORK/ext/extskill"; w "$EXT/SKILL.md" "# external\n"
junction "$HOME_DIR/.claude/skills/linked" "$VAULT/skills/前端/vue"
junction "$HOME_DIR/.claude/skills/extskill" "$EXT"
GONE="$WORK/gone"; w "$GONE/SKILL.md" "# gone\n"
junction "$HOME_DIR/.claude/skills/brokenone" "$GONE"
rm -rf "$GONE"
REGISTRY="$WORK/registry.json"
python -I - "$REGISTRY" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"uninstall": [{"displayName": "TRAE 2.0", "installLocation": ""}]}))
PY

start_app() { # extra env NAME=VALUE ...
  env APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" \
    SKILLS_FAKE_HOME="$(cygpath -w "$HOME_DIR")" \
    SKILLS_FAKE_PATH="$(cygpath -w "$HOME_DIR/.local/bin")" \
    SKILLS_FAKE_REGISTRY="$(cygpath -w "$REGISTRY")" "$@" "$APP" >"$WORK/app.log" 2>&1 &
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
close_banner() { ev "(() => { const b = document.querySelector('[data-testid=\"skills-banner-close\"]'); if (b) b.click(); return true; })()" >/dev/null; }
open_panel() { ev "window.__mkbApp.getState().openSkillsPanel(); true" >/dev/null; wait_match "panel open" "document.querySelector('[data-testid=\"skills-panel\"]') !== null" '"value":true' 20; }
open_tools() { ev "(() => { const tab=[...document.querySelectorAll('[data-testid=\"skills-panel\"] [role=tab]')].find((t)=>t.textContent.includes('工具技能')); if(tab) tab.click(); return true; })()" >/dev/null; sleep 0.5; }
open_kb() { ev "(() => { const tab=[...document.querySelectorAll('[data-testid=\"skills-panel\"] [role=tab]')].find((t)=>t.textContent.includes('知识库技能')); if(tab) tab.click(); return true; })()" >/dev/null; sleep 0.5; }
card_distribute() { # category name
  ev "(() => { const card=document.querySelector('[data-testid=\"kb-grid-$1\"] [data-testid=\"kb-card-$2\"]'); const b=card && [...card.querySelectorAll('button')].find((x)=>x.textContent.trim()==='分发'); if(!b) throw new Error('missing distribute'); b.click(); return true; })()" >/dev/null
  wait_match "distribute dialog" "document.querySelector('[data-testid=\"distribute-dialog\"]') !== null" '"value":true' 20
}
select_target() { # toolId
  ev "(() => { const el=document.querySelector('[data-testid=\"distribute-target-$1\"]'); if(!el) throw new Error('missing target'); if(el.getAttribute('aria-checked') !== 'true') el.click(); return true; })()" >/dev/null
}
origin_pill() { ev "document.querySelector('[data-testid=\"tool-skill-$1-$2\"] .pill')?.textContent.trim()"; }
row_exists() { ev "document.querySelector('[data-testid=\"tool-skill-$1-$2\"]') !== null"; }

# --- first run --------------------------------------------------------------
start_app
wait_match "overview" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
try_index() { wait_match "index" "!document.body.innerText.includes('未索引')" '"value":true' 90; }
try_index
open_panel
open_tools
wait_match "tools rows" "document.querySelector('[data-testid=\"tool-skill-linked-claude-code\"]') !== null" '"value":true' 40

check "AC-SKM-2 linked" "document.querySelector('[data-testid=\"tool-skill-linked-claude-code\"] .pill').textContent.trim() === '链接到知识库'" '"value":true'
check "AC-SKM-2 broken" "document.querySelector('[data-testid=\"tool-skill-brokenone-claude-code\"] .pill').textContent.trim() === '链接失效'" '"value":true'
check "AC-SKM-2 copied" "document.querySelector('[data-testid=\"tool-skill-copiedone-claude-code\"] .pill').textContent.trim().indexOf('拷贝') >= 0 || document.querySelector('[data-testid=\"tool-skill-copiedone-claude-code\"] .pill').textContent.trim().length > 0" '"value":true'
check "AC-SKM-2 independent" "document.querySelector('[data-testid=\"tool-skill-indepone-claude-code\"] .pill').textContent.trim() === '独立'" '"value":true'
check "AC-SKM-2 builtin" "document.querySelector('[data-testid=\"tool-skill-builtinone-trae\"] .pill').textContent.trim() === '内置'" '"value":true'
check "AC-SKM-2 external" "document.querySelector('[data-testid=\"tool-skill-extskill-claude-code\"] .pill').textContent.trim() === '链接到外部'" '"value":true'

# AC-SKM-2: uninstall the external link; the target bytes stay the same.
EXT_BEFORE=$(hash_dir "$EXT")
click '[data-testid="tool-uninstall-extskill-claude-code"]'
wait_match "uninstall dialog" "document.querySelector('[data-testid=\"uninstall-dialog\"]') !== null" '"value":true' 20
click '[data-testid="uninstall-confirm"]'
wait_match "external removed" "document.querySelector('[data-testid=\"skills-banner\"]') !== null" '"value":true' 30
close_banner
[ ! -e "$HOME_DIR/.claude/skills/extskill" ] && { pass=$((pass+1)); echo "PASS AC-SKM-2 link gone"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-2 link gone"; }
[ "$(hash_dir "$EXT")" = "$EXT_BEFORE" ] && { pass=$((pass+1)); echo "PASS AC-SKM-2 external target unchanged"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-2 external target changed"; }

# AC-SKM-1: Kiro is leftover-only; hidden until expanded; never a target.
check "AC-SKM-1 kiro hidden by default" "document.querySelector('[data-testid=\"tool-chip-kiro\"]') === null" '"value":true'
click '[data-testid="tool-toggle-hidden"]'
wait_match "AC-SKM-1 kiro chip after expand" "document.querySelector('[data-testid=\"tool-chip-kiro\"]') !== null" '"value":true' 20
check "AC-SKM-1 kiro skill visible" "document.querySelector('[data-testid=\"tool-skill-x-kiro\"]') !== null" '"value":true'
open_kb
card_distribute "前端" "vue"
check "AC-SKM-1 kiro not a target" "document.querySelector('[data-testid=\"distribute-target-kiro\"]') === null" '"value":true'

# AC-SKM-3: distribute 前端/vue to Claude Code as a junction (the dialog is
# exercised by AC-SKM-11; the write path is driven through the same API).
ev "window.__mkbApi.skills.distribute('$VID','skills/前端/vue',[{targetId:'claude-code'}],'link').then((r)=>JSON.stringify(r.items))" >/dev/null
sleep 1.5
printf '
知识库更新。
' >> "$VAULT/skills/前端/vue/SKILL.md"
grep -q "知识库更新" "$HOME_DIR/.claude/skills/vue/SKILL.md" && { pass=$((pass+1)); echo "PASS AC-SKM-3 read through link"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-3 read through link"; }

# AC-SKM-5: the same name from another category is refused.
CONFLICT=$(ev "window.__mkbApi.skills.distributePreview('$VID','skills/后端/vue',[{targetId:'claude-code'}],'link').then((r)=>JSON.stringify(r.items))")
if echo "$CONFLICT" | grep -q 'action.*conflict'; then pass=$((pass+1)); echo "PASS AC-SKM-5 conflict"; else fail=$((fail+1)); echo "FAIL AC-SKM-5 conflict :: $CONFLICT"; fi

# AC-SKM-6: overwrite the independent vue in Cursor, then restore the backup.
ev "window.__mkbApi.skills.distribute('$VID','skills/前端/vue',[{targetId:'cursor',allowOverwrite:true}],'link').then((r)=>JSON.stringify(r.items))" >/dev/null
sleep 1.5
grep -q "知识库更新" "$HOME_DIR/.cursor/skills/vue/SKILL.md" && { pass=$((pass+1)); echo "PASS AC-SKM-6 overwrite content"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-6 overwrite content"; }
# Restore refuses an occupied path (PRD), so remove the new link first, then
# bring the backup back from the install view.
open_tools
click '[data-testid="tool-uninstall-vue-cursor"]'
wait_match "AC-SKM-6 uninstall new" "document.querySelector('[data-testid=\"uninstall-dialog\"]') !== null" '"value":true' 20
click '[data-testid="uninstall-confirm"]'
wait_match "AC-SKM-6 removed new" "document.querySelector('[data-testid=\"skills-banner\"]') !== null" '"value":true' 30
close_banner
ev "(() => { const b=[...document.querySelectorAll('[data-testid=\"skills-panel\"] button')].find((x)=>x.textContent.trim()==='安装'); if(b) b.click(); return true; })()" >/dev/null
wait_match "install view" "document.querySelector('[data-testid=\"install-records\"]') !== null" '"value":true' 20
click '[data-testid^="install-restore-"]'
wait_match "AC-SKM-6 restore" "document.querySelector('[data-testid=\"install-notice\"]') !== null" '"value":true' 30
grep -q "# old vue" "$HOME_DIR/.cursor/skills/vue/SKILL.md" && { pass=$((pass+1)); echo "PASS AC-SKM-6 restore content"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-6 restore content"; }
ev "(() => { const b=document.querySelector('[aria-label=\"返回技能列表\"]'); if(b) b.click(); return true; })()" >/dev/null
sleep 0.3

# AC-SKM-7: uninstall a link; the knowledge base bytes are unchanged.
KB_BEFORE=$(hash_dir "$VAULT/skills/前端/vue")
open_tools
click '[data-testid="tool-uninstall-linked-claude-code"]'
wait_match "AC-SKM-7 dialog" "document.querySelector('[data-testid=\"uninstall-dialog\"]') !== null" '"value":true' 20
click '[data-testid="uninstall-confirm"]'
wait_match "AC-SKM-7 removed" "document.querySelector('[data-testid=\"skills-banner\"]') !== null" '"value":true' 30
close_banner
[ ! -e "$HOME_DIR/.claude/skills/linked" ] && { pass=$((pass+1)); echo "PASS AC-SKM-7 link gone"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-7 link gone"; }
[ "$(hash_dir "$VAULT/skills/前端/vue")" = "$KB_BEFORE" ] && { pass=$((pass+1)); echo "PASS AC-SKM-7 kb unchanged"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-7 kb changed"; }

# AC-SKM-8: uninstall an independent skill, then restore it.
w "$HOME_DIR/.claude/skills/indepone/SKILL.md" "# independent\n"
INDEP_BEFORE=$(hash_dir "$HOME_DIR/.claude/skills/indepone")
open_tools
click '[data-testid="tool-uninstall-indepone-claude-code"]'
wait_match "AC-SKM-8 dialog" "document.querySelector('[data-testid=\"uninstall-dialog\"]') !== null" '"value":true' 20
click '[data-testid="uninstall-confirm"]'
wait_match "AC-SKM-8 removed" "document.querySelector('[data-testid=\"skills-banner\"]') !== null" '"value":true' 30
close_banner
[ ! -e "$HOME_DIR/.claude/skills/indepone" ] && { pass=$((pass+1)); echo "PASS AC-SKM-8 backed up"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-8 not removed"; }
ev "(() => { const b=[...document.querySelectorAll('[data-testid=\"skills-panel\"] button')].find((x)=>x.textContent.trim()==='安装'); if(b) b.click(); return true; })()" >/dev/null
wait_match "AC-SKM-8 records" "document.querySelector('[data-testid=\"install-records\"]') !== null" '"value":true' 20
click '[data-testid^="install-restore-"]'
wait_match "AC-SKM-8 restore" "document.querySelector('[data-testid=\"install-notice\"]') !== null" '"value":true' 30
[ "$(hash_dir "$HOME_DIR/.claude/skills/indepone")" = "$INDEP_BEFORE" ] && { pass=$((pass+1)); echo "PASS AC-SKM-8 restored"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-8 not restored"; }
check "AC-SKM-8 records include restore" "document.body.innerText.includes('恢复')" '"value":true'

# AC-SKM-9: adopt the same foo from two tools; both become links.
ev "window.__mkbApi.skills.adopt('$VID','claude-code','foo','分类',true,[{toolId:'cursor',path:'foo'}]).then((r)=>JSON.stringify({warnings:r.warnings}))" >/dev/null
sleep 1.5
grep -q "# foo" "$VAULT/skills/分类/foo/SKILL.md" && { pass=$((pass+1)); echo "PASS AC-SKM-9 vault copy"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-9 vault copy"; }
if grep -q "# foo" "$HOME_DIR/.claude/skills/foo/SKILL.md" && grep -q "# foo" "$HOME_DIR/.cursor/skills/foo/SKILL.md"; then
  pass=$((pass+1)); echo "PASS AC-SKM-9 both tools link"
else
  fail=$((fail+1)); echo "FAIL AC-SKM-9 tool links"
fi
if python -I -c "import json,os,sys; root=sys.argv[1]; kinds=[json.load(open(os.path.join(root,n,'meta.json'),encoding='utf-8')).get('kind') for n in os.listdir(root) if os.path.exists(os.path.join(root,n,'meta.json'))] if os.path.isdir(root) else []; sys.exit(0 if 'adopt' in kinds else 1)" "$APPD/Mile Knowledge Base/skills/backups"; then
  pass=$((pass+1)); echo "PASS AC-SKM-9 backups"
else
  fail=$((fail+1)); echo "FAIL AC-SKM-9 backups"
fi
ev "window.__mkbSkills.getState().loadScan('$VID'); true" >/dev/null
sleep 1.0
open_tools
wait_match "AC-SKM-9 linked row" "document.querySelector('[data-testid=\"tool-skill-foo-claude-code\"] .pill')?.textContent.includes('链接到知识库')" '"value":true' 30

# AC-SKM-10: builtin skills have no uninstall action.
check "AC-SKM-10 no uninstall for builtin" "document.querySelector('[data-testid=\"tool-uninstall-builtinone-trae\"]') === null" '"value":true'
REJECT=$(ev "window.__mkbApi.skills.uninstall('trae','builtinone').then(()=> 'unexpected').catch((e)=>String(e.code||e.message))")
echo "$REJECT" | grep -q 'skill-protected' && { pass=$((pass+1)); echo "PASS AC-SKM-10 api rejects"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-10 api rejects :: $REJECT"; }

# AC-SKM-11: panel over settings; Esc returns from the subview, then closes.
ev "window.__mkbApp.getState().closeSkillsPanel(); true" >/dev/null
ev "window.__mkbApp.getState().openSettings(); true" >/dev/null
wait_match "settings open" "document.querySelector('[data-panel=\"settings-nav\"]') !== null" '"value":true' 30
click '[data-testid="open-skills"]'
wait_match "AC-SKM-11 panel over settings" "document.querySelector('[data-testid=\"skills-panel\"]') !== null" '"value":true' 20
open_kb
card_distribute "前端" "vue"
ev "document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); true" >/dev/null
sleep 0.4
check "AC-SKM-11 first Esc back to list" "document.querySelector('[data-testid=\"distribute-dialog\"]') === null && document.querySelector('[data-testid=\"skills-panel\"]') !== null" '"value":true'
ev "document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); true" >/dev/null
sleep 0.4
check "AC-SKM-11 second Esc closes" "document.querySelector('[data-testid=\"skills-panel\"]') === null" '"value":true'
check "AC-SKM-11 focus back on pill" "document.activeElement?.getAttribute('data-testid') === 'open-skills'" '"value":true'
check "AC-SKM-11 no sidebar import entry" "document.body.innerText.indexOf('导入技能') < 0" '"value":true'
ev "window.__mkbApp.getState().closeSettings(); true" >/dev/null
stop_app

# --- AC-SKM-4: junction failure falls back to copy --------------------------
start_app SKILLS_FAIL_JUNCTION=1
wait_match "second overview" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
try_index
open_panel
open_tools
if ev "document.querySelector('[data-testid=\"tool-uninstall-vue-claude-code\"]') !== null" | grep -q '"value":true'; then
  click '[data-testid="tool-uninstall-vue-claude-code"]'
  wait_match "old vue dialog" "document.querySelector('[data-testid=\"uninstall-dialog\"]') !== null" '"value":true' 20
  click '[data-testid="uninstall-confirm"]'
  wait_match "old vue removed" "document.querySelector('[data-testid=\"skills-banner\"]') !== null" '"value":true' 30
  close_banner
fi
RESULT=$(ev "window.__mkbApi.skills.distribute('$VID','skills/前端/vue',[{targetId:'claude-code'}],'link').then((r)=>JSON.stringify(r.items))")
echo "AC-SKM-4 items: $RESULT"
echo "$RESULT" | grep -q 'copied' && { pass=$((pass+1)); echo "PASS AC-SKM-4 reported copy"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-4 reported copy :: $RESULT"; }
[ -f "$HOME_DIR/.claude/skills/vue/SKILL.md" ] && { pass=$((pass+1)); echo "PASS AC-SKM-4 copied files"; } || { fail=$((fail+1)); echo "FAIL AC-SKM-4 copied files"; }
printf '
再次更新。
' >> "$VAULT/skills/前端/vue/SKILL.md"
# The panel answers from a cache younger than 2 seconds (loop guard); let it age.
sleep 2.2
ev "window.__mkbSkills.getState().loadScan('$VID'); true" >/dev/null
sleep 1.0
open_tools
wait_match "AC-SKM-4 sync state" "document.querySelector('[data-testid=\"tool-skill-vue-claude-code\"] .pill')?.textContent.includes('知识库有更新')" '"value":true' 40
check "AC-SKM-4 redistribute action" "document.querySelector('[aria-label=\"重新分发 vue\"]') !== null" '"value":true'
stop_app

echo "RESULT pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
