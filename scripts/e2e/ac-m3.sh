#!/usr/bin/env bash
# AC-M3: the M3 closed loop on a real window — overview tiles, five settings
# sections, theme persistence, alias through the save bar, the include-in-search
# switch, the about page, and a restart. Onboarding and skill import need a
# native folder dialog; they are covered by their own tests and stay manual.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.ac-m3-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"
mkdir -p "$VAULT" "$APPD/Mile Knowledge base"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null)
VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge base" <<'PY'
import hashlib,json,os,sys
v,out=sys.argv[1],sys.argv[2]; c=os.path.abspath(v).replace('/',chr(92)).lower(); vid=hashlib.sha256(c.encode()).hexdigest()[:16]
doc={"version":1,"theme":"light","closeBehavior":"quit","layout":{"sidebarWidth":232,"docWidth":620,"docCollapsed":False,"sidebarCollapsed":False,"agnesControlsCollapsed":False,"agnesHistoryOpen":True,"agnesComposerHeight":96},"recentVaults":[{"id":vid,"root":v,"displayName":"ac-m3","lastOpenedAt":"2026-11-19T09:00:00Z"}],"lastVaultId":vid}
open(os.path.join(out,"settings.json"),"w",encoding="utf-8").write(json.dumps(doc,ensure_ascii=False,indent=2)+"\n"); print(vid)
PY
)

DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1

PORT=18441
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then taskkill //F //PID "$APP_PID" >/dev/null 2>&1; fi
  taskkill //F //IM "Mile Knowledge Base.exe" >/dev/null 2>&1
  rm -rf "$WORK"
}
trap cleanup EXIT

start_app() {
  APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" "$APP" >"$WORK/app.log" 2>&1 &
  APP_PID=$!
  sleep 8
}
stop_app() {
  if [ -n "${APP_PID:-}" ]; then taskkill //F //PID "$APP_PID" >/dev/null 2>&1; fi
  sleep 1
  taskkill //F //IM "Mile Knowledge Base.exe" >/dev/null 2>&1
  sleep 2
}
export MYGO_AUTOMATION="tcp:127.0.0.1:$PORT"
ev() {
  python -I - "$WORK/req.json" "$1" <<'PY'
import json,sys
open(sys.argv[1],"w",encoding="utf-8").write(json.dumps({"id":1,"method":"eval","params":{"window":1,"script":sys.argv[2]}}))
PY
  timeout 40 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
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
set_value() {
  local proto=HTMLInputElement; [ "${3:-input}" = "textarea" ] && proto=HTMLTextAreaElement
  ev "(() => { const el = document.querySelector($(jsq "$1")); if (!el) throw new Error('missing'); const set = Object.getOwnPropertyDescriptor(window.$proto.prototype,'value').set; set.call(el, $(jsq "$2")); el.dispatchEvent(new Event('input',{bubbles:true})); return true; })()"
}

start_app
wait_match "overview visible" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 60
wait_match "index ready" "(() => { const el = document.querySelector('[data-testid=\"overview-index\"]'); return !!el && el.textContent.indexOf('已就绪') >= 0; })()" '"value":true' 180
check "overview recent tile" "document.querySelector('[data-testid=\"overview-recent\"]') !== null" '"value":true'
check "overview skills tile has count" "(() => { const el = document.querySelector('[data-testid=\"overview-skills\"]'); return !!el && /[1-9]/.test(el.textContent); })()" '"value":true'

ev "(() => { const app = window.__mkbApp.getState(); window.__mkbDocs.getState().open(app.activeVaultId, 'raws/首页.md'); app.setLayout({ docCollapsed: false }); return true; })()" >/dev/null
wait_match "recent list updated" "document.querySelector('[data-testid=\"overview-recent-raws/首页.md\"]') !== null" '"value":true' 40

click '[data-testid="open-settings"]'
wait_match "settings opened" "document.querySelector('[data-panel=\"settings-nav\"]') !== null" '"value":true'
for pair in "general:settings-appearance" "model:model-service-section" "agents:agents-section" "categories:settings-categories" "about:settings-about"; do
  section=${pair%%:*}; marker=${pair##*:}
  click "[data-testid=\"settings-nav-$section\"]"
  wait_match "settings $section" "document.querySelector('[data-testid=\"$marker\"]') !== null" '"value":true' 30
done
check "about shows a version" "(() => { const el = document.querySelector('[data-testid=\"about-version\"]'); return !!el && /^v[0-9]/.test(el.textContent.trim()); })()" '"value":true'

click '[data-testid="settings-nav-general"]'
wait_match "general opened" "document.querySelector('[data-testid=\"settings-appearance\"]') !== null" '"value":true'
click '[data-testid="theme-dark"]'
wait_match "theme dark applied" "document.documentElement.dataset.theme === 'dark'" '"value":true' 20
click '[data-testid="theme-light"]'
wait_match "theme light applied" "document.documentElement.dataset.theme === 'light'" '"value":true' 20

click '[data-testid="settings-nav-categories"]'
wait_match "categories opened" "document.querySelector('[data-testid=\"category-alias-projects\"]') !== null" '"value":true'
set_value '[data-testid="category-alias-projects"]' "项目库"
wait_match "save bar appears" "document.querySelector('[data-testid=\"settings-savebar\"]') !== null" '"value":true' 20
click '[data-testid="settings-savebar-save"]'
wait_match "save bar cleared" "document.querySelector('[data-testid=\"settings-savebar\"]') === null" '"value":true' 30

click '[data-testid="category-indexed-raws"]'
check "switch off" "document.querySelector('[data-testid=\"category-indexed-raws\"]').getAttribute('aria-checked') === 'false'" '"value":true'
ev "(() => { const b = [...document.querySelectorAll('button')].find((x) => (x.getAttribute('aria-label') || '') === '关闭设置'); if (b) b.click(); return true; })()" >/dev/null
sleep 0.8
check "sidebar shows alias" "(() => { const row = [...document.querySelectorAll('.tree-scroll [role=treeitem]')].find((r) => r.getAttribute('title') === 'projects'); return !!row && row.textContent.includes('项目库'); })()" '"value":true'
ev "window.__mkbSearch.getState().openPanel(); window.__mkbSearch.getState().setText('共享'); true" >/dev/null
wait_match "search settled" "(() => { const s = window.__mkbSearch.getState(); return !s.searching && s.text === '共享'; })()" '"value":true' 60
check "no raws hits while off" "window.__mkbSearch.getState().hits.every((h) => !h.path.startsWith('raws/'))" '"value":true'
ev "window.__mkbSearch.getState().closePanel(); true" >/dev/null

click '[data-testid="open-settings"]'
wait_match "settings reopened" "document.querySelector('[data-panel=\"settings-nav\"]') !== null" '"value":true'
click '[data-testid="settings-nav-categories"]'
wait_match "categories again" "document.querySelector('[data-testid=\"category-indexed-raws\"]') !== null" '"value":true'
click '[data-testid="category-indexed-raws"]'
check "switch on" "document.querySelector('[data-testid=\"category-indexed-raws\"]').getAttribute('aria-checked') === 'true'" '"value":true'
ev "window.__mkbSearch.getState().openPanel(); window.__mkbSearch.getState().setText('共享'); true" >/dev/null
wait_match "search settled again" "(() => { const s = window.__mkbSearch.getState(); return !s.searching && s.text === '共享'; })()" '"value":true' 60
wait_match "raws hits return" "window.__mkbSearch.getState().hits.some((h) => h.path.startsWith('raws/'))" '"value":true' 40
ev "window.__mkbSearch.getState().closePanel(); true" >/dev/null

click '[data-testid="settings-nav-general"]'
wait_match "general again" "document.querySelector('[data-testid=\"settings-appearance\"]') !== null" '"value":true'

# V-SET-SHELL-2: Ctrl+S saves the settings draft only, never the dirty document behind it.
ev "window.__mkbDocs.getState().open(window.__mkbApp.getState().activeVaultId, 'raws/首页.md'); true" >/dev/null
wait_match "doc opened for ctrl-s" "(() => { const d = window.__mkbDocs.getState(); const doc = d.docs['raws/首页.md']; return d.activePath === 'raws/首页.md' && !!doc && !doc.loading; })()" '"value":true' 40
ev "(() => { const d = window.__mkbDocs.getState(); const doc = d.docs['raws/首页.md']; d.setText('raws/首页.md', doc.text + String.fromCharCode(10) + 'ctrl-s 检查'); return true; })()" >/dev/null
click '[data-testid="settings-nav-agents"]'
wait_match "agents for ctrl-s" "document.querySelector('[data-testid=\"steering-input\"]') !== null" '"value":true' 20
ev "(() => { const ta = document.querySelector('[data-testid=\"steering-input\"]'); const set = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype,'value').set; set.call(ta, ta.value + String.fromCharCode(10) + '更简洁。'); ta.dispatchEvent(new Event('input',{bubbles:true})); return true; })()" >/dev/null
wait_match "draft bar" "document.querySelector('[data-testid=\"settings-savebar\"]') !== null" '"value":true' 20
ev "document.dispatchEvent(new KeyboardEvent('keydown', { key: 's', ctrlKey: true, bubbles: true })); true" >/dev/null
wait_match "ctrl-s cleared the bar" "document.querySelector('[data-testid=\"settings-savebar\"]') === null" '"value":true' 30
check "document still dirty" "(() => { const d = window.__mkbDocs.getState(); const doc = d.docs['raws/首页.md']; return !!doc && doc.text !== doc.savedText; })()" '"value":true'

# 设置页盖住工作台，先关掉才能看到总览磁贴。
ev "window.__mkbApp.getState().closeSettings(); true" >/dev/null
wait_match "workbench after ctrl-s" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 20

# AC-OVW-1: a pending card on another agent shows as 1 with the warning border.
ev "window.__mkbAgent.setState((state) => { const other = ['curator','scout','distiller','general'].find((a) => a !== state.agent); const buckets = { ...state.buckets }; buckets[other] = { ...buckets[other], pendingPermissions: [{ requestId: 'ac-ovw-1' }] }; return { buckets }; }); true" >/dev/null
wait_match "overview back" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 20
check "pending tile counts 1" "(() => { const el = document.querySelector('[data-testid=\"overview-pending\"]'); return !!el && el.getAttribute('data-count') === '1'; })()" '"value":true'
check "pending tile warns" "(() => { const el = document.querySelector('[data-testid=\"overview-pending\"]'); return !!el && el.getAttribute('data-warn') === 'true'; })()" '"value":true'

click '[data-testid="open-settings"]'
wait_match "settings for theme" "document.querySelector('[data-panel=\"settings-nav\"]') !== null" '"value":true'
click '[data-testid="settings-nav-general"]'
wait_match "general for theme" "document.querySelector('[data-testid=\"settings-appearance\"]') !== null" '"value":true'
click '[data-testid="theme-dark"]'
wait_match "dark before restart" "document.documentElement.dataset.theme === 'dark'" '"value":true' 20
stop_app
start_app
wait_match "restarted overview" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 60
check "theme persisted" "document.documentElement.dataset.theme === 'dark'" '"value":true'
check "alias persisted" "(() => { const row = [...document.querySelectorAll('.tree-scroll [role=treeitem]')].find((r) => r.getAttribute('title') === 'projects'); return !!row && row.textContent.includes('项目库'); })()" '"value":true'

# V-SET-SHELL-5: with no vault open, the settings page says so (常规 still usable).
stop_app
EMPTY="$WORK/empty-app"; mkdir -p "$EMPTY"
APPDATA="$(cygpath -w "$EMPTY")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" "$APP" >"$WORK/empty.log" 2>&1 &
APP_PID=$!
sleep 8
wait_match "welcome without vault" "document.querySelector('.welcome-shell') !== null" '"value":true' 60
ev "window.__mkbApp.getState().openSettings(); true" >/dev/null
wait_match "settings without vault" "document.querySelector('[data-panel=\"settings-nav\"]') !== null" '"value":true' 20
click '[data-testid="settings-nav-model"]'
wait_match "no-vault notice" "document.querySelector('[data-testid=\"settings-no-vault\"]') !== null" '"value":true' 20
check "no-vault notice copy" "(() => { const el = document.querySelector('[data-testid=\"settings-no-vault\"]'); return !!el && el.textContent.includes('打开一个知识库后才能配置模型服务'); })()" '"value":true'
stop_app

echo "RESULT pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
