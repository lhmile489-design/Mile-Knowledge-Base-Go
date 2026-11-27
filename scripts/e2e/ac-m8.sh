#!/usr/bin/env bash
# AC-M8: the standalone skills page on a real window (PRD §17.6, AC-PAGE-1–6).
# Uses a temporary APPDATA, vault and fake HOME; the fake provider streams a
# long answer for the continuity check. Real pointer clicks are used where the
# assertion is about focus.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.ac-m8-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; APPD2="$WORK/app-novault"; HOME_DIR="$WORK/home"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base" "$APPD2/Mile Knowledge Base" "$HOME_DIR/.claude/skills/demo"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null)
printf '# demo\n\n独立技能。\n' > "$HOME_DIR/.claude/skills/demo/SKILL.md"
# 1.6.1: enough cards that the knowledge-base view must scroll.
python -I - "$VAULT" <<'PY'
import os, sys
for i in range(40):
    d = os.path.join(sys.argv[1], "skills", "zz-scroll", "scroll-%02d" % i)
    os.makedirs(d, exist_ok=True)
    open(os.path.join(d, "SKILL.md"), "w", encoding="utf-8").write(
        "---\nname: scroll-%02d\ndescription: 用于滚动与列表布局回归的技能 %02d，描述写得长一些，确认列表模式里只占一行并截断显示。\n---\n\n# scroll-%02d\n" % (i, i, i))
PY
REGISTRY="$WORK/registry.json"
printf '{"uninstall": []}\n' > "$REGISTRY"

VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge Base" <<'PY'
import hashlib, json, os, sys
vault, out = sys.argv[1], sys.argv[2]
c = os.path.abspath(vault).replace("/", chr(92)).lower()
vid = hashlib.sha256(c.encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": False,
                  "sidebarCollapsed": False, "agnesControlsCollapsed": False,
                  "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": vault, "displayName": "ac-m8", "lastOpenedAt": "2026-11-24T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(out, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

PORT_AI=18522
python -I - "$VAULT" "$PORT_AI" <<'PY'
import json, os, sys
vault, port = sys.argv[1], sys.argv[2]
path = os.path.join(vault, ".mile-kb", "settings", "providers.json")
os.makedirs(os.path.dirname(path), exist_ok=True)
doc = {"version": 1, "providers": [{"name": "fake", "kind": "openai", "baseUrl": "http://127.0.0.1:" + port,
                                    "models": ["fake-model"], "defaultModel": "fake-model", "apiKeyEnv": ""}]}
open(path, "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
PY
python -I "$ROOT/scripts/e2e/fake-openai.py" --port "$PORT_AI" >"$WORK/fake-ai.log" 2>&1 &
AI_PID=$!

DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1

PORT=18521
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  if [ -n "${AI_PID:-}" ]; then kill "$AI_PID" >/dev/null 2>&1; fi
  sleep 1
  rm -rf "$WORK"
}
trap cleanup EXIT
if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running"; exit 2
fi
for _ in $(seq 1 50); do
  code=$(curl.exe -s -o NUL -w "%{http_code}" "http://127.0.0.1:$PORT_AI/health")
  [ "$code" = "200" ] && break
  sleep 0.2
done

start_app() { # appdata [extra env ...]
  local appdata="$1"; shift
  env APPDATA="$(cygpath -w "$appdata")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" \
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

eval_value() { # script -> the value, as text
  python -I - "$WORK/req.json" "$1" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "eval", "params": {"window": 1, "script": sys.argv[2]}}))
PY
  timeout 60 "$DRIVE" -f "$WORK/req.json" 2>/dev/null | python -I -c "import json,sys; line=sys.stdin.readline(); print(json.loads(line)['result'].get('value',''))"
}
ev() {
  python -I - "$WORK/req.json" "$1" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "eval", "params": {"window": 1, "script": sys.argv[2]}}))
PY
  timeout 60 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}
jsq() { python -I -c "import json,sys;print(json.dumps(sys.argv[1]))" "$1"; }
click() { ev "(() => { const el=document.querySelector($(jsq "$1")); if(!el) throw new Error('missing'); el.click(); return true; })()"; }

foreground_app() {
  [ -n "${APP_PID:-}" ] || return 0
  cat > "$WORK/focus.ps1" <<'PS'
param([int]$ProcId)
Add-Type -Namespace Win -Name Fg -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
'@
$p = Get-Process -Id $ProcId -ErrorAction SilentlyContinue
if ($p -and $p.MainWindowHandle -ne 0) { [Win.Fg]::SetForegroundWindow($p.MainWindowHandle) | Out-Null }
try { (New-Object -ComObject WScript.Shell).AppActivate($ProcId) | Out-Null } catch {}
PS
  powershell -NoProfile -ExecutionPolicy Bypass -File "$(cygpath -w "$WORK/focus.ps1")" -ProcId "$APP_PID" >/dev/null 2>&1
  sleep 0.4
}

resize_window() { # width height
  cat > "$WORK/resize.ps1" <<'PS'
param([int]$ProcId, [int]$Width, [int]$Height)
Add-Type -Namespace Win -Name Win -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr hWnd, IntPtr after, int x, int y, int cx, int cy, uint flags);
'@
$p = Get-Process -Id $ProcId -ErrorAction SilentlyContinue
if ($p -and $p.MainWindowHandle -ne 0) { [Win.Win]::SetWindowPos($p.MainWindowHandle, [IntPtr]::Zero, 0, 0, $Width, $Height, 0x0002 -bor 0x0004) | Out-Null }
PS
  powershell -NoProfile -ExecutionPolicy Bypass -File "$(cygpath -w "$WORK/resize.ps1")" -ProcId "$APP_PID" -Width "$1" -Height "$2" >/dev/null 2>&1
  sleep 0.6
}

pass=0; fail=0
check() { # label script pattern
  local out; out=$(ev "$2")
  if echo "$out" | grep -q "$3"; then pass=$((pass+1)); echo "PASS $1"; else fail=$((fail+1)); echo "FAIL $1 :: $out"; fi
}
wait_match() { # label script pattern tries
  local label=$1 script=$2 pattern=$3 tries=${4:-30} i out
  for i in $(seq 1 "$tries"); do
    out=$(ev "$script")
    if echo "$out" | grep -q "$pattern"; then pass=$((pass+1)); echo "PASS $label"; return 0; fi
    sleep 0.4
  done
  fail=$((fail+1)); echo "FAIL $label :: $out"; return 1
}
show_skills() { ev "window.__mkbApp.getState().showPage('skills'); true" >/dev/null; }
show_work() { ev "window.__mkbApp.getState().showPage('work'); true" >/dev/null; }

# ===== vault run ===========================================================
start_app "$APPD"
wait_match "overview" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
wait_match "index ready" "!document.body.innerText.includes('未索引')" '"value":true' 90

echo "== AC-PAGE-2: switch, subview, Esc, back with focus"
# The automation backend refuses pointer input for WebView2 pages ("use eval"),
# so the switch is a DOM click; the focus step a real click performs is done
# explicitly below.
click '[data-testid="open-skills"]'
wait_match "skills page open" "document.querySelector('[data-testid=\"skills-page\"]') !== null" '"value":true' 20
check "page fills below the title bar" "(() => { const r=document.querySelector('[data-testid=\"skills-page\"]').getBoundingClientRect(); return r.top >= 40 && r.width > window.innerWidth - 24; })()" '"value":true'
check "workbench stays inert under the page" "document.querySelector('[data-testid=\"workbench-host\"]')?.hasAttribute('inert') === true" '"value":true'
# M10 T03: the skills host is transparent so the window colour field shows
# through; the hidden workbench no longer needs an opaque cover because it
# paints nothing (opacity 0) and takes no pointer input.
check "hidden workbench paints nothing and takes no input" "(() => { const wb=document.querySelector('[data-testid=\"workbench-host\"]'); const hit=document.elementFromPoint(Math.round(window.innerWidth/2), Math.round(window.innerHeight/2)); const s=getComputedStyle(wb); return s.opacity === '0' && s.pointerEvents === 'none' && !hit?.closest('[data-testid=\"workbench-host\"]'); })()" '"value":true'
check "skills host lets the window colour field through" "(() => { const host=document.querySelector('[data-testid=\"skills-host\"]'); const field=document.querySelector('[data-testid=\"background-field\"]'); return getComputedStyle(host).backgroundColor === 'rgba(0, 0, 0, 0)' && !!field && getComputedStyle(field).zIndex === '-1' && field.getBoundingClientRect().top === 0; })()" '"value":true'
wait_match "update check entry" "!!document.querySelector('[data-testid=\"skills-check-updates\"]')" '"value":true' 20
click '[data-testid="skills-check-updates"]'
sleep 0.6
check "manual update check stays usable" "(() => { const b=document.querySelector('[data-testid=\"skills-check-updates\"]'); return !!b && !b.disabled; })()" '"value":true'
wait_match "kb cards" "document.querySelector('[data-testid^=\"kb-card-\"]') !== null" '"value":true' 40

echo "== 1.6.1: the kb view scrolls and list mode keeps one row per skill"
wait_match "scroll fixtures scanned" "document.querySelectorAll('[data-testid^=\"kb-card-scroll-\"]').length >= 40" '"value":true' 40
check "kb content is bounded by the page" "(() => { const page=document.querySelector('[data-testid=\"skills-page\"]').getBoundingClientRect(); const r=document.querySelector('.skills-page .kb-content').getBoundingClientRect(); return r.bottom <= page.bottom + 1; })()" '"value":true'
check "kb content scrolls (grid)" "(() => { const el=document.querySelector('.skills-page .kb-content'); el.scrollTop=0; el.scrollTop=300; const moved=el.scrollTop > 0; el.scrollTop=0; return el.scrollHeight > el.clientHeight && moved; })()" '"value":true'
click '.skills-page .kb-view-toggle [aria-label="列表"]'
wait_match "list mode" "document.querySelector('[data-testid=\"kb-list-zz-scroll\"]') !== null" '"value":true' 20
check "list card is a single row" "(() => { const c=document.querySelector('[data-testid=\"kb-card-scroll-00\"]'); const [a,b,m,f]=['.kb-card-top','.kb-card-desc','.kb-card-meta','.kb-card-foot'].map((s)=>c.querySelector(s).getBoundingClientRect()); return c.getBoundingClientRect().height < 80 && a.right <= b.left + 1 && b.right <= m.left + 1 && m.right <= f.left + 1; })()" '"value":true'
check "list name is not squeezed" "(() => { const c=document.querySelector('[data-testid=\"kb-card-scroll-00\"]'); const n=c.querySelector('.kb-card-name'); return n.scrollWidth <= n.clientWidth && c.querySelector('.kb-card-top').getBoundingClientRect().width >= 179; })()" '"value":true'
check "kb content scrolls (list)" "(() => { const el=document.querySelector('.skills-page .kb-content'); el.scrollTop=0; el.scrollTop=300; const moved=el.scrollTop > 0; el.scrollTop=0; return el.scrollHeight > el.clientHeight && moved; })()" '"value":true'
click '.skills-page .kb-view-toggle [aria-label="网格"]'
wait_match "grid mode back" "document.querySelector('[data-testid=\"kb-grid-zz-scroll\"]') !== null" '"value":true' 20

ev "(() => { const card=document.querySelector('[data-testid^=\"kb-card-\"]'); const b=[...card.querySelectorAll('button')].find((x)=>x.textContent.trim()==='分发'); if(!b) throw new Error('missing distribute'); b.click(); return true; })()" >/dev/null
wait_match "distribute subview" "document.querySelector('[data-testid=\"distribute-dialog\"]') !== null" '"value":true' 20
ev "document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); true" >/dev/null
wait_match "first Esc back to the list" "!document.querySelector('[data-testid=\"distribute-dialog\"]') && !!document.querySelector('[data-testid=\"skills-page\"]')" '"value":true' 20
ev "document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); true" >/dev/null
sleep 0.5
check "second Esc keeps the page" "document.querySelector('[data-testid=\"skills-page\"]') !== null" '"value":true'
click '[data-testid="page-switch"] .page-switch-item'
foreground_app
ev "(() => { const b=document.querySelector('[data-testid=\"page-switch\"] .page-switch-item'); window.__workFocused=false; b && b.addEventListener('focus', () => { window.__workFocused=true; }, { once: true }); b && b.focus(); return true; })()" >/dev/null
wait_match "workbench back" "document.querySelector('[data-testid=\"workbench-host\"]')?.hasAttribute('hidden') === false" '"value":true' 20
wait_match "focus on the work segment" "window.__workFocused === true" '"value":true' 20
check "no sidebar import entry" "document.body.innerText.indexOf('导入技能') < 0" '"value":true'

echo "== AC-PAGE-1: stream, draft, doc edits and scroll survive the switch"
ev "(() => { window.__mkbComposer.getState().setText('请给我一段长回答，用于切换页面的连续性测试'); return true; })()" >/dev/null
ev "window.__mkbComposer.getState().send('$VID'); true" >/dev/null
wait_match "stream started" "document.querySelector('[data-testid=chat-answer][data-streaming=true]') !== null" '"value":true' 20
sleep 0.3
LEN1=$(eval_value "(() => { const a=[...document.querySelectorAll('[data-testid=chat-answer]')].pop(); return a ? a.textContent.length : 0; })()")
ev "window.__mkbComposer.getState().setText('未发送的草稿'); true" >/dev/null
wait_match "doc opened" "window.__mkbDocs.getState().open('$VID','raws/共享笔记.md').then(()=>true).catch(()=>false)" '"value":true' 20
ev "window.__mkbDocs.getState().setText('raws/共享笔记.md','# 共享笔记\n\n未保存的改动。\n'); true" >/dev/null
show_skills
sleep 5
check "skills page visible during the stream" "document.querySelector('[data-testid=\"skills-page\"]') !== null" '"value":true'
LEN2=$(eval_value "(() => { const a=[...document.querySelectorAll('[data-testid=chat-answer]')].pop(); return a ? a.textContent.length : 0; })()")
if [ "${LEN2:-0}" -gt "${LEN1:-0}" ]; then pass=$((pass+1)); echo "PASS stream kept growing ($LEN1 -> $LEN2)"; else fail=$((fail+1)); echo "FAIL stream stalled ($LEN1 -> $LEN2)"; fi
check "composer draft kept" "window.__mkbComposer.getState().text === '未发送的草稿'" '"value":true'
check "doc stays dirty" "(() => { const d=window.__mkbDocs.getState().docs['raws/共享笔记.md']; return !!d && d.text !== d.savedText; })()" '"value":true'
show_work
wait_match "stream finished after the return" "window.__mkbAgent.getState().buckets[window.__mkbAgent.getState().agent].busy === false" '"value":true' 80
check "long answer rendered" "[...document.querySelectorAll('[data-testid=chat-answer]')].some(e=>e.innerText.includes('知识库的整理原则'))" '"value":true'
sleep 1
ev "(() => { const b=document.querySelector('[data-testid=chat-body]'); b.scrollTop=Math.min(120, b.scrollHeight - b.clientHeight); return b.scrollTop; })()" >/dev/null
sleep 0.6
BEFORE_SCROLL=$(eval_value "document.querySelector('[data-testid=chat-body]').scrollTop")
show_skills
sleep 1
show_work
sleep 0.8
AFTER_SCROLL=$(eval_value "document.querySelector('[data-testid=chat-body]').scrollTop")
if [ "${BEFORE_SCROLL:-x}" = "${AFTER_SCROLL:-y}" ]; then pass=$((pass+1)); echo "PASS chat scroll kept ($BEFORE_SCROLL)"; else fail=$((fail+1)); echo "FAIL chat scroll $BEFORE_SCROLL -> $AFTER_SCROLL"; fi

echo "== AC-PAGE-3: settings drafts keep the leave guard"
ev "window.__mkbApp.getState().openSettings(); true" >/dev/null
wait_match "settings open" "document.querySelector('[data-panel=\"settings-nav\"]') !== null" '"value":true' 20
click '[data-testid="settings-nav-agents"]'
wait_match "steering field" "document.querySelector('[data-testid=\"steering-input\"]') !== null" '"value":true' 20
ev "(() => { const t=document.querySelector('[data-testid=\"steering-input\"]'); const set=Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set; set.call(t,'改过的 steering'); t.dispatchEvent(new Event('input',{bubbles:true})); return true; })()" >/dev/null
show_skills
wait_match "leave dialog" "document.querySelector('[data-testid=\"settings-leave-dialog\"]') !== null" '"value":true' 20
click '[data-testid="settings-leave-stay"]'
sleep 0.4
check "stay keeps settings and the draft" "document.querySelector('[data-panel=\"settings-nav\"]') !== null && !!document.querySelector('[data-testid=\"steering-input\"]')" '"value":true'
show_skills
wait_match "leave dialog again" "document.querySelector('[data-testid=\"settings-leave-dialog\"]') !== null" '"value":true' 20
click '[data-testid="settings-leave-discard"]'
wait_match "skills page after discarding" "document.querySelector('[data-testid=\"skills-page\"]') !== null && document.querySelector('[data-panel=\"settings-nav\"]') === null" '"value":true' 20

echo "== AC-PAGE-5: narrow window keeps the rail and the subviews inside"
resize_window 744 640
sleep 0.6
check "narrow viewport" "window.innerWidth <= 1024" '"value":true'
check "nav collapses to 56px" "Math.round(document.querySelector('[data-testid=\"skills-page\"] .skills-nav').getBoundingClientRect().width) <= 60" '"value":true'
check "no horizontal scroll" "document.documentElement.scrollWidth <= window.innerWidth" '"value":true'
wait_match "kb cards still there" "document.querySelector('[data-testid^=\"kb-card-\"]') !== null" '"value":true' 20
ev "(() => { const card=document.querySelector('[data-testid^=\"kb-card-\"]'); const b=[...card.querySelectorAll('button')].find((x)=>x.textContent.trim()==='分发'); if(!b) throw new Error('missing distribute'); b.click(); return true; })()" >/dev/null
wait_match "distribute subview in the narrow window" "(() => { const r=document.querySelector('[data-testid=\"distribute-dialog\"]')?.getBoundingClientRect(); return !!r && r.left >= 0 && r.right <= window.innerWidth && r.bottom <= window.innerHeight; })()" '"value":true' 20
ev "document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); true" >/dev/null
sleep 0.4
click '[data-testid="skills-nav"] button[title="全部工具"]'
wait_match "tool row" "document.querySelector('[data-testid=\"tool-skill-demo-claude-code\"]') !== null" '"value":true' 40
click '[data-testid="tool-uninstall-demo-claude-code"]'
wait_match "confirm card complete in the narrow window" "(() => { const r=document.querySelector('[data-testid=\"uninstall-dialog\"]')?.getBoundingClientRect(); return !!r && r.left >= 0 && r.right <= window.innerWidth && r.top >= 40 && r.bottom <= window.innerHeight; })()" '"value":true' 20
ev "document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); true" >/dev/null
sleep 0.4
check "Esc cancels the confirm" "document.querySelector('[data-testid=\"uninstall-dialog\"]') === null" '"value":true'
stop_app

echo "== AC-PAGE-6: reduced motion"
if [ "${M8_SYSTEM_MOTION:-0}" = "1" ]; then
  # Q-M8-4: the user turned the system animation effect off for this run.
  start_app "$APPD"
else
  start_app "$APPD" WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--force-prefers-reduced-motion
fi
show_skills
wait_match "motion query" "matchMedia('(prefers-reduced-motion: reduce)').matches === true" '"value":true' 20
wait_match "cards visible" "document.querySelector('[data-testid^=\"kb-card-\"]') !== null" '"value":true' 30
check "no entrance animation" "getComputedStyle(document.querySelector('[data-testid^=\"kb-card-\"]')).animationName === 'none'" '"value":true'
check "field stops drifting" "getComputedStyle(document.querySelector('.background-field span')).animationName === 'none'" '"value":true'
check "hover lift disabled by the media rule" "[...document.styleSheets].some((sheet) => { try { return [...sheet.cssRules].some((rule) => rule.conditionText?.includes('prefers-reduced-motion') && rule.cssText.includes('.skills-page .kb-card:hover') && rule.cssText.includes('transform: none')); } catch { return false; } })" '"value":true'
check "no hover lift now" "getComputedStyle(document.querySelector('[data-testid^=\"kb-card-\"]')).transform === 'none'" '"value":true'
stop_app

echo "== AC-PAGE-4: no vault, tools still usable"
start_app "$APPD2"
wait_match "welcome" "document.querySelector('[data-testid=\"welcome-stub\"], [data-testid=\"welcome-page\"], .welcome-shell') !== null" '"value":true' 60
click '[data-testid="open-skills"]'
wait_match "skills page without a vault" "document.querySelector('[data-testid=\"skills-page\"]') !== null" '"value":true' 20
check "kb group explains the empty state" "document.querySelector('[data-testid=\"skills-no-vault\"]')?.textContent.includes('还没有打开知识库')" '"value":true'
click '[data-testid="skills-nav"] button[title="全部工具"]'
wait_match "tool directory works without a vault" "document.querySelector('[data-testid=\"tool-skill-demo-claude-code\"]') !== null" '"value":true' 40
click '[data-testid="tool-uninstall-demo-claude-code"]'
wait_match "uninstall confirm" "document.querySelector('[data-testid=\"uninstall-dialog\"]') !== null" '"value":true' 20
click '[data-testid="uninstall-confirm"]'
wait_match "uninstalled without a vault" "document.querySelector('[data-testid=\"tool-skill-demo-claude-code\"]') === null" '"value":true' 40
stop_app

echo "RESULT pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
