#!/usr/bin/env bash
# M10 T06 driver: overview tiles, composer, health panel, plan card, notices and menus
# on a real window, in the std / off / high blur tiers (light + dark where it matters).
#
# Usage (repo root):
#   bash spikes/m10-effects/t06/run-t06.sh <app.exe> <out-prefix>
set -u
export PYTHONUTF8=1 PYTHONIOENCODING=utf-8
export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
APP="$1"
OUT="${2:-}"
SHOT="${3:-}"
WORK=$(mktemp -d "$HERE/.run-XXXXXX")
VAULT="$WORK/vault"
APPD="$WORK/appdata"
HOMEDIR="$WORK/home"
PORT=18464
export MYGO_AUTOMATION="tcp:127.0.0.1:$PORT"
APP_PID=""
mkdir -p "$VAULT/skills/demo" "$VAULT/skills/zz-scroll" "$APPD/Mile Knowledge Base" "$HOMEDIR"

cleanup() {
  if [ -n "${APP_PID:-}" ]; then powershell.exe -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  sleep 1
  powershell.exe -NoProfile -Command 'Get-Process "Mile Knowledge Base" -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "*.m10-*" -or $_.Path -like "*Mile-Knowledge-Base-Go*" } | Stop-Process -Force' >/dev/null 2>&1
  rm -rf "$WORK"
}
trap cleanup EXIT

mkdir -p "$VAULT/raws"
printf '# 共享笔记

普通 `.md` 文件是唯一事实源，应用不改编码、换行与未编辑过的字节。

## 整理原则

暂存区只放待处理的剪藏；整理后归入对应分区，见 [[分类说明]]。

## 恢复

破坏性操作前会捕获原文件，可以从操作日志按回合恢复。
' > "$VAULT/raws/共享笔记.md"
printf -- '---\nname: demo\ndescription: M10 T02 parity sample.\n---\n\n# demo\n' > "$VAULT/skills/demo/SKILL.md"
python -I - "$VAULT" <<'PY'
import os, sys
for i in range(6):
    d = os.path.join(sys.argv[1], "skills", "zz-scroll", "s-%02d" % i)
    os.makedirs(d, exist_ok=True)
    open(os.path.join(d, "SKILL.md"), "w", encoding="utf-8").write(
        "---\nname: s-%02d\ndescription: parity sample %02d.\n---\n\n# s-%02d\n" % (i, i, i))
PY
python -I - "$VAULT" "$APPD/Mile Knowledge Base" <<'PY'
import hashlib, json, os, sys
v, out = sys.argv[1], sys.argv[2]
root = os.path.abspath(v)
vid = hashlib.sha256(root.replace('/', chr(92)).lower().encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": True, "sidebarCollapsed": False,
                  "agnesControlsCollapsed": False, "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": root, "displayName": "parity", "lastOpenedAt": "2026-11-27T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(out, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
PY

DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1
ev() {
  python -I - "$WORK/req.json" "$1" <<'PY2'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "eval", "params": {"window": 1, "script": sys.argv[2]}}))
PY2
  timeout 60 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}
raw() {
  local resp
  resp="$(ev "$1")"
  printf '%s' "$resp" | python -I -X utf8 -c "import json,sys;d=json.loads(sys.stdin.read());v=d['result']['value'];print(v if isinstance(v,str) else json.dumps(v))" 2>/dev/null || echo "RAW-ERR: $resp"
}
capture() {
  python -I - "$WORK/capreq.json" <<'PY2'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "capture", "params": {"window": 1}}))
PY2
  timeout 60 "$DRIVE" -f "$WORK/capreq.json" 2>&1 | grep -m1 '{"id"' > "$WORK/cap.json"
  python -I - "$WORK/cap.json" "$1" <<'PY2'
import base64, json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
open(sys.argv[2], "wb").write(base64.b64decode(d["result"]["png"]))
PY2
}
poll() {
  for _ in $(seq 1 100); do
    [ "$(raw "$1" 2>/dev/null)" = "true" ] && return 0
    sleep 0.3
  done
  return 1
}

APPDATA="$(cygpath -w "$APPD")" HOME="$(cygpath -w "$HOMEDIR")" SKILLS_FAKE_HOME="$(cygpath -w "$HOMEDIR")" \
  "$APP" >"$WORK/app.log" 2>&1 &
APP_PID=$!

poll "!!document.querySelector('.tree-row')" || { echo "app not ready"; tail -5 "$WORK/app.log"; exit 1; }
VID_JS="window.__mkbApp.getState().activeVaultId"
poll "!!document.querySelector('.tree-row')" || { echo "tree not ready"; exit 1; }
poll "!!document.querySelector('[data-testid=overview-grid]')" || { echo "overview not ready"; exit 1; }
sleep 1.5
PREFIX="${2:-$HERE/out}"
freeze() {
  ev "(() => { if (document.activeElement && document.activeElement.blur) document.activeElement.blur(); document.getAnimations().forEach((a) => { try { a.pause(); a.currentTime = 0; a.finish(); } catch {} }); return 'ok'; })()" >/dev/null
  sleep 0.5
}
tier() { # blur theme
  ev "document.documentElement.dataset.blur='$1'; document.documentElement.dataset.theme='$2'; true" >/dev/null; sleep 0.6
}
STATE="$PREFIX-state.txt"; : > "$STATE"
say() { echo "$*" | tee -a "$STATE"; }
CARD_PROBE="(() => { const q=(s)=>document.querySelector(s); const f=(el)=>{ if(!el) return null; const s=getComputedStyle(el); return {bg:s.backgroundColor, filter:s.backdropFilter, shadow:s.boxShadow.slice(0,70), anim:s.animationName}; }; return JSON.stringify({tile:f(q('[data-testid=overview-skills]')), runtime:f(q('.runtime-controls')), composer:f(q('.composer-card')), textarea:f(q('.composer-input')), chatBody:f(q('[data-testid=chat-body]')), chatBodySheet:q('[data-testid=chat-body]')?.classList.contains('sheet')}); })()"

say "== overview std light"
tier std light; freeze
say "$(raw "$CARD_PROBE")"
capture "$PREFIX-overview-std-light.png"
tier std dark; freeze; capture "$PREFIX-overview-std-dark.png"
tier high light; freeze; capture "$PREFIX-overview-high-light.png"
say "== overview off light"
tier off light; freeze
say "$(raw "$CARD_PROBE")"
capture "$PREFIX-overview-off-light.png"

# hover lift on a clickable tile (std motion)
tier std light
ev "document.documentElement.dataset.motion='std'; true" >/dev/null
ev "(() => { const el=document.querySelector('[data-testid=overview-skills]'); window.__rest = getComputedStyle(el).transform; el.dispatchEvent(new MouseEvent('mouseover',{bubbles:true})); return true; })()" >/dev/null
say "tile transform at rest: $(raw "getComputedStyle(document.querySelector('[data-testid=overview-skills]')).transform")"

# health panel
say "== health panel"
ev "window.__mkbApp.getState().openHealth(); true" >/dev/null
poll "!!document.querySelector('.health-panel')" || say "health panel did not open"
sleep 0.8
for T in "std light" "std dark" "off light" "high light"; do
  set -- $T; tier $1 $2; freeze
  say "health $1 $2: $(raw "(() => { const p=document.querySelector('.health-panel'); const s=getComputedStyle(p); const b=getComputedStyle(document.querySelector('.health-backdrop'), '::before'); return JSON.stringify({bg:s.backgroundColor, filter:s.backdropFilter, scrimFilter:b.backdropFilter, bodySheet:document.querySelector('.health-body')?.classList.contains('sheet'), backdropRect:(() => { const r=document.querySelector('.health-backdrop').getBoundingClientRect(); return [r.left,r.top,r.width,r.height].map(Math.round); })(), panelRect:(() => { const r=p.getBoundingClientRect(); return [r.left,r.top,r.width,r.height].map(Math.round); })(), pos:getComputedStyle(document.querySelector('.health-backdrop')).position, parent:p.parentElement.className}); })()")"
  capture "$PREFIX-health-$1-$2.png"
done
ev "window.__mkbApp.getState().closeHealth && window.__mkbApp.getState().closeHealth(); true" >/dev/null
click_id() { ev "(() => { const el=document.querySelector('$1'); if(!el) throw new Error('missing $1'); el.click(); return true; })()" >/dev/null; }
click_id ".health-close" 2>/dev/null
sleep 0.5

# plan card
say "== plan card"
ev "window.__mkbMutation.getState().propose({ id: 'p-1', vaultId: $VID_JS, origin: 'user', risk: 'medium', actions: [{ id: 'a1', kind: 'create', target: 'notes/示例.md', text: '示例', reason: '截图用的示例动作' }, { id: 'a2', kind: 'move', source: 'raws/a.md', target: 'notes/a.md', reason: '示例移动' }] }, '截图用变更计划'); true" >/dev/null
poll "!!document.querySelector('.plan-card')" || say "plan card did not open"
sleep 0.6
for T in "std light" "std dark" "off light"; do
  set -- $T; tier $1 $2; freeze
  say "plan $1 $2: $(raw "(() => { const s=getComputedStyle(document.querySelector('.plan-card')); return JSON.stringify({bg:s.backgroundColor, filter:s.backdropFilter}); })()")"
  capture "$PREFIX-plan-$1-$2.png"
done
ev "window.__mkbMutation.getState().cancel(); true" >/dev/null
sleep 0.4

# notice
say "== notice"
tier std light
ev "window.__mkbApp.getState().pushNotice('已保存到本机', 'info'); true" >/dev/null
sleep 0.8; freeze
say "notice: $(raw "(() => { const s=getComputedStyle(document.querySelector('.notice')); return JSON.stringify({bg:s.backgroundColor, filter:s.backdropFilter}); })()")"
capture "$PREFIX-notice-std-light.png"

# route menu (popover over the composer)
say "== route menu"
click_id "[data-testid=route-target]"
sleep 0.7; freeze
say "route menu: $(raw "(() => { const el=document.querySelector('.agent-picker, .popover'); if(!el) return 'none'; const s=getComputedStyle(el); return JSON.stringify({cls:el.className, bg:s.backgroundColor, filter:s.backdropFilter}); })()")"
capture "$PREFIX-route-menu-std-light.png"

# docked panels in the document column: search results and the operation log
say "== search + audit panels (document open, doc panel expanded)"
ev "window.__mkbApp.getState().setLayout({ docCollapsed: false }); true" >/dev/null
sleep 0.8
ev "window.__mkbDocs.getState().open($VID_JS, 'raws/共享笔记.md').then(() => 'ok')" >/dev/null
sleep 1.2
ev "window.__mkbSearch.getState().openPanel ? window.__mkbSearch.getState().openPanel() : window.__mkbSearch.setState({ open: true }); true" >/dev/null
ev "window.__mkbApp.getState().toggleAudit(); true" >/dev/null
sleep 1.2
for T in "std light" "std dark" "off light"; do
  set -- $T; tier $1 $2; freeze
  say "docked $1 $2: $(raw "(() => { const f=(s)=>{ const el=document.querySelector(s); if(!el) return null; const c=getComputedStyle(el); return {bg:c.backgroundColor, filter:c.backdropFilter}; }; return JSON.stringify({search:f('.search-panel'), searchBody:f('.search-body'), audit:f('.audit-panel'), auditBody:f('.audit-body')}); })()")"
  capture "$PREFIX-docked-$1-$2.png"
done

say "audit rects: $(raw "(() => { const r=(s)=>{ const el=document.querySelector(s); if(!el) return null; const b=el.getBoundingClientRect(); return [Math.round(b.left),Math.round(b.top),Math.round(b.width),Math.round(b.height)]; }; return JSON.stringify({audit:r('.audit-panel'), auditBody:r('.audit-body'), pane:r('.editor-pane'), editorBody:r('.editor-body'), panel:r('[data-panel=editor]'), open:window.__mkbApp.getState().auditOpen}); })()")"

# the audit panel alone (opened together with the search panel it is clipped by
# the 1.7.0 column layout, unchanged by M10)
ev "window.__mkbSearch.getState().closePanel(); true" >/dev/null
sleep 0.8
for T in "std light" "off light"; do
  set -- $T; tier $1 $2; freeze
  capture "$PREFIX-audit-$1-$2.png"
done
say "audit alone rect: $(raw "(() => { const el=document.querySelector('.audit-panel'); const b=el.getBoundingClientRect(); return JSON.stringify([Math.round(b.left),Math.round(b.top),Math.round(b.width),Math.round(b.height)]); })()")"
