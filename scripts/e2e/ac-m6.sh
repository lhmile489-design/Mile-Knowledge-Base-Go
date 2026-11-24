#!/usr/bin/env bash
# AC-M6: registered custom tools and projects on a real window (PRD §15;
# AC-SKM-17, AC-SKM-18 and the M6 rules). Uses the e2e build's fake HOME and
# the folder queue (SKILLS_FAKE_PICK_DIR=@<file>) so the real tool directories
# are never written; the vault, the project and APPDATA are temporary.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.ac-m6-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"
CUSTOM="$WORK/custom-tool"; PROJECT="$WORK/site"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base" "$HOME_DIR" "$CUSTOM/alpha" "$PROJECT/.claude/skills/x" "$PROJECT/.agents/skills/y"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null)
printf '# alpha\n\n' > "$CUSTOM/alpha/SKILL.md"
mkdir -p "$VAULT/skills/前端/vue"
printf '# vue\n\n' > "$VAULT/skills/前端/vue/SKILL.md"
printf '# x\n\n' > "$PROJECT/.claude/skills/x/SKILL.md"
printf '# y\n\n' > "$PROJECT/.agents/skills/y/SKILL.md"

VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge Base" <<'PY'
import hashlib, json, os, sys
vault, appdir = sys.argv[1], sys.argv[2]
c = os.path.abspath(vault).replace("/", chr(92)).lower()
vid = hashlib.sha256(c.encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": False,
                  "sidebarCollapsed": False, "agnesControlsCollapsed": False,
                  "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": vault, "displayName": "ac-m6", "lastOpenedAt": "2026-11-22T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

QUEUE="$WORK/pick-queue.txt"
python -I - "$QUEUE" "$(cygpath -w "$CUSTOM")" "$(cygpath -w "$PROJECT")" "$(cygpath -w "$VAULT/skills")" "$(cygpath -w "$CUSTOM")" <<'PY'
import sys
open(sys.argv[1], "w", encoding="utf-8").write("\n".join(sys.argv[2:]) + "\n")
PY

REGISTRY="$WORK/registry.json"
python -I - "$REGISTRY" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"uninstall": []}))
PY

# Real tool directories must not change (read-only snapshot: names, sizes, mtimes).
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

DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1
PORT=18526
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

start_app() {
  env APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" \
    SKILLS_FAKE_HOME="$(cygpath -w "$HOME_DIR")" \
    SKILLS_FAKE_PATH="$(cygpath -w "$HOME_DIR/.local/bin")" \
    SKILLS_FAKE_REGISTRY="$(cygpath -w "$REGISTRY")" \
    SKILLS_FAKE_PICK_DIR="@$(cygpath -w "$QUEUE")" "$APP" >"$WORK/app.log" 2>&1 &
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
fill() { # testid value
  ev "(() => { const el=document.querySelector('[data-testid=\"$1\"]'); if(!el) throw new Error('missing'); const setter=Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set; setter.call(el,$(jsq "$2")); el.dispatchEvent(new Event('input',{bubbles:true})); return true; })()" >/dev/null
}
close_banner() { ev "(() => { const b = document.querySelector('[data-testid=\"skills-banner-close\"]'); if (b) b.click(); return true; })()" >/dev/null; }
open_panel() { ev "window.__mkbApp.getState().showPage('skills'); true" >/dev/null; wait_match "skills page open" "document.querySelector('[data-testid=\"skills-page\"]') !== null" '"value":true' 20; }
open_tools() { ev "(() => { const b=document.querySelector('[data-testid=\"skills-nav\"] button[title=\"全部工具\"]'); if(b) b.click(); return true; })()" >/dev/null; sleep 0.5; }
open_kb() { ev "(() => { const b=document.querySelector('[data-testid=\"skills-nav\"] button[title=\"全部技能\"]'); if(b) b.click(); return true; })()" >/dev/null; sleep 0.5; }
open_targets() {
  click '[data-testid="skills-nav"] button[title="管理目录…"]'
  wait_match "targets view" "document.querySelector('[data-testid=\"targets-view\"]') !== null" '"value":true' 20
}
target_id() { # name fragment → t-xxx
  ev "(() => { const row=[...document.querySelectorAll('[data-testid^=\"target-row-t-\"]')].find((el)=>el.textContent.includes($(jsq "$1"))); return row ? row.getAttribute('data-testid').slice('target-row-'.length) : ''; })()" | grep -o '"value":"[^"]*"' | cut -d'"' -f4
}
card_distribute() { # category name
  ev "(() => { const card=document.querySelector('[data-testid=\"kb-grid-$1\"] [data-testid=\"kb-card-$2\"]'); const b=card && [...card.querySelectorAll('button')].find((x)=>x.textContent.trim()==='分发'); if(!b) throw new Error('missing distribute'); b.click(); return true; })()" >/dev/null
  wait_match "distribute dialog" "document.querySelector('[data-testid=\"distribute-dialog\"]') !== null" '"value":true' 20
}
select_target() { # toolId
  ev "(() => { const el=document.querySelector('[data-testid=\"distribute-target-$1\"]'); if(!el) throw new Error('missing target'); if(el.getAttribute('aria-checked') !== 'true') el.click(); return true; })()" >/dev/null
}

start_app
wait_match "overview" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
wait_match "index" "!document.body.innerText.includes('未索引')" '"value":true' 90
open_panel
open_tools
open_targets

# --- register a custom tool and a project -----------------------------------
click '[data-testid="targets-add-custom"]'
wait_match "custom form" "document.querySelector('[data-testid=\"target-name\"]') !== null" '"value":true' 20
fill target-name "Droid"
click '[data-testid="target-pick"]'
wait_match "custom picked" "document.querySelector('[data-testid=\"target-picked\"]')?.textContent.includes('custom-tool')" '"value":true' 20
click '[data-testid="target-save"]'
wait_match "custom row" "document.querySelector('[data-testid^=\"target-row-t-\"]') !== null" '"value":true' 30
CUSTOM_TID=$(target_id "Droid")
[ -n "$CUSTOM_TID" ] && { pass=$((pass+1)); echo "PASS custom id $CUSTOM_TID"; } || { fail=$((fail+1)); echo "FAIL custom id"; }
check "custom path masked" "document.querySelector('[data-testid=\"target-row-$CUSTOM_TID\"] .targets-path').textContent.includes('…')" '"value":true'

click '[data-testid="targets-add-project"]'
wait_match "project form" "document.querySelector('[data-testid=\"target-name\"]') !== null" '"value":true' 20
fill target-name "Site"
click '[data-testid="target-pick"]'
wait_match "project dirs" "document.querySelector('[data-testid=\"target-dirs\"]')?.textContent.includes('.claude/skills')" '"value":true' 20
click '[data-testid="target-install-.claude/skills"]'
click '[data-testid="target-save"]'
wait_match "project row" "[...document.querySelectorAll('[data-testid^=\"target-row-t-\"]')].some((el)=>el.textContent.includes('Site'))" '"value":true' 30
PROJECT_TID=$(target_id "Site")
[ -n "$PROJECT_TID" ] && { pass=$((pass+1)); echo "PASS project id $PROJECT_TID"; } || { fail=$((fail+1)); echo "FAIL project id"; }
check "targets count line" "document.querySelector('[data-testid=\"targets-count\"]').textContent.includes('自定义 1') && document.querySelector('[data-testid=\"targets-count\"]').textContent.includes('项目 1')" '"value":true'
close_banner
check "project path masked" "document.querySelector('[data-testid=\"target-row-$PROJECT_TID\"] .targets-path').textContent.includes('…')" '"value":true'

# --- rejections: inside the vault, then overlapping the custom tool ---------
click '[data-testid="targets-add-custom"]'
fill target-name "Bad Vault"
click '[data-testid="target-pick"]'
wait_match "bad picked" "document.querySelector('[data-testid=\"target-picked\"]')?.textContent.includes('vault')" '"value":true' 20
click '[data-testid="target-save"]'
wait_match "vault rejected" "document.querySelector('[data-testid=\"targets-error\"]')?.textContent.includes('知识库')" '"value":true' 30
check "vault form kept open" "document.querySelector('[data-testid=\"target-name\"]') !== null" '"value":true'
click '[data-testid="targets-cancel"]'
wait_match "back to list" "document.querySelector('[data-testid=\"targets-add-custom\"]') !== null" '"value":true' 20
click '[data-testid="targets-add-custom"]'
fill target-name "Twin"
click '[data-testid="target-pick"]'
wait_match "twin picked" "document.querySelector('[data-testid=\"target-picked\"]')?.textContent.includes('custom-tool')" '"value":true' 20
click '[data-testid="target-save"]'
wait_match "overlap rejected" "document.querySelector('[data-testid=\"targets-error\"]')?.textContent.includes('重叠')" '"value":true' 30
click '[data-testid="targets-cancel"]'
wait_match "list again" "document.querySelector('[data-testid=\"targets-add-custom\"]') !== null" '"value":true' 20
click '[data-testid="targets-back"]'
sleep 0.5

# --- AC-SKM-18: both groups show up in the tool page ------------------------
open_tools
wait_match "custom chip" "document.querySelector('[data-testid=\"tool-chip-$CUSTOM_TID\"]') !== null" '"value":true' 40
wait_match "project chip" "document.querySelector('[data-testid=\"tool-chip-$PROJECT_TID\"]') !== null" '"value":true' 40
click "[data-testid=\"tool-chip-$PROJECT_TID\"]"
wait_match "project row x" "document.querySelector('[data-testid=\"tool-skill-x-$PROJECT_TID\"]') !== null" '"value":true' 30
wait_match "project row y" "document.querySelector('[data-testid=\"tool-skill-y-$PROJECT_TID\"]') !== null" '"value":true' 30
check "project row attribution" "document.querySelector('[data-testid=\"tool-skill-x-$PROJECT_TID\"] .tool-row-icon')?.getAttribute('title') === 'Claude Code · .claude/skills'" '"value":true'
check "project row path masked" "document.querySelector('[data-testid=\"tool-skill-x-$PROJECT_TID\"] .tool-row-path').textContent.includes('…')" '"value":true'

# --- AC-SKM-17: distribute to both, then remove the custom registration -----
KB_BEFORE=$(hash_dir "$VAULT/skills/前端/vue")
open_kb
wait_match "kb vue card" "document.querySelector('[data-testid=\"kb-card-vue\"]') !== null" '"value":true' 40
card_distribute "前端" "vue"
select_target "$CUSTOM_TID"
select_target "$PROJECT_TID"
check "two targets selected" "document.querySelector('[data-testid=\"distribute-summary\"]').textContent.includes('已选 2')" '"value":true'
click '[data-testid="distribute-submit"]'
wait_match "distributed" "document.querySelector('[data-testid=\"skills-banner\"]')?.textContent.includes('已联接')" '"value":true' 40
close_banner
[ -e "$CUSTOM/vue" ] && { pass=$((pass+1)); echo "PASS custom link written"; } || { fail=$((fail+1)); echo "FAIL custom link written"; }
[ -e "$PROJECT/.claude/skills/vue" ] && { pass=$((pass+1)); echo "PASS project link written"; } || { fail=$((fail+1)); echo "FAIL project link written"; }
printf '\nM6 验收更新。\n' >> "$VAULT/skills/前端/vue/SKILL.md"
grep -q "M6 验收更新" "$CUSTOM/vue/SKILL.md" && { pass=$((pass+1)); echo "PASS read through custom link"; } || { fail=$((fail+1)); echo "FAIL read through custom link"; }
CUSTOM_BEFORE=$(hash_dir "$CUSTOM")

open_tools
click "[data-testid=\"tool-chip-$CUSTOM_TID\"]"
open_targets_from_tools() { click '[data-testid="skills-nav"] button[title="管理目录…"]'; wait_match "targets view 2" "document.querySelector('[data-testid=\"targets-view\"]') !== null" '"value":true' 20; }
open_targets_from_tools
click "[data-testid=\"target-remove-$CUSTOM_TID\"]"
wait_match "remove dialog" "document.querySelector('[data-testid=\"target-remove-dialog\"]') !== null" '"value":true' 20
check "remove dialog keeps files" "document.querySelector('[data-testid=\"target-remove-dialog\"]').textContent.includes('目录里的文件不会删除')" '"value":true'
click '[data-testid="target-remove-confirm"]'
wait_match "removed banner" "document.querySelector('[data-testid=\"skills-banner\"]')?.textContent.includes('删除 1 条分发记录')" '"value":true' 30
close_banner
[ "$(hash_dir "$CUSTOM")" = "$CUSTOM_BEFORE" ] && { pass=$((pass+1)); echo "PASS registration removal kept files"; } || { fail=$((fail+1)); echo "FAIL registration removal changed files"; }
[ "$(hash_dir "$VAULT/skills/前端/vue")" != "$KB_BEFORE" ] && { pass=$((pass+1)); echo "PASS kb edit visible"; } || { fail=$((fail+1)); echo "FAIL kb edit"; }
click '[data-testid="targets-back"]'
sleep 0.5
wait_match "custom chip gone" "[...document.querySelectorAll('[data-testid^=\"tool-chip-t-\"]')].every((el)=>!el.textContent.includes('Droid'))" '"value":true' 40
check "project chip remains" "document.querySelector('[data-testid=\"tool-chip-$PROJECT_TID\"]') !== null" '"value":true'

# --- settings entry opens the manager directly ------------------------------
ev "window.__mkbApp.getState().showPage('work'); true" >/dev/null
ev "window.__mkbApp.getState().openTargetsManager(); true" >/dev/null
wait_match "manager via settings entry" "document.querySelector('[data-testid=\"targets-view\"]') !== null" '"value":true' 20
ev "window.__mkbApp.getState().showPage('work'); true" >/dev/null
stop_app

# --- real tool directories unchanged ----------------------------------------
REAL_CLAUDE_AFTER=$(real_snapshot "$USERPROFILE/.claude/skills")
REAL_AGENTS_AFTER=$(real_snapshot "$USERPROFILE/.agents")
[ "$REAL_CLAUDE_BEFORE" = "$REAL_CLAUDE_AFTER" ] && { pass=$((pass+1)); echo "PASS real .claude untouched"; } || { fail=$((fail+1)); echo "FAIL real .claude changed"; }
[ "$REAL_AGENTS_BEFORE" = "$REAL_AGENTS_AFTER" ] && { pass=$((pass+1)); echo "PASS real .agents untouched"; } || { fail=$((fail+1)); echo "FAIL real .agents changed"; }

echo "RESULT pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
