#!/usr/bin/env bash
# M10 T07 driver: the settings page, its dialogs and the welcome page with the new-vault
# guide, in the std / off / high blur tiers (temp APPDATA, vault and HOME only).
#
# Usage (repo root):
#   bash spikes/m10-effects/t07/run-t07.sh <app.exe> <out-prefix>
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
PORT=18465
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

launch() {
  APPDATA="$(cygpath -w "$APPD")" HOME="$(cygpath -w "$HOMEDIR")" SKILLS_FAKE_HOME="$(cygpath -w "$HOMEDIR")" \
    "$APP" >"$WORK/app.log" 2>&1 &
  APP_PID=$!
}
stop_app() {
  # $! is an MSYS pid, not the Windows one: kill by path, and only the build in this repo
  powershell.exe -NoProfile -Command 'Get-Process "Mile Knowledge Base" -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "*Mile-Knowledge-Base-Go*" } | Stop-Process -Force' >/dev/null 2>&1
  sleep 3
}
PREFIX="${2:-$HERE/out}"
STATE="$PREFIX-state.txt"; : > "$STATE"
say() { echo "$*" | tee -a "$STATE"; }
freeze() {
  ev "(() => { if (document.activeElement && document.activeElement.blur) document.activeElement.blur(); document.getAnimations().forEach((a) => { try { a.pause(); a.currentTime = 0; a.finish(); } catch {} }); return 'ok'; })()" >/dev/null
  sleep 0.5
}
tier() { ev "document.documentElement.dataset.blur='$1'; document.documentElement.dataset.theme='$2'; true" >/dev/null; sleep 0.6; }
sty() { # selector -> compact computed style
  raw "(() => { const el=document.querySelector('$1'); if(!el) return 'none'; const s=getComputedStyle(el); return JSON.stringify({bg:s.backgroundColor, filter:s.backdropFilter, shadow:s.boxShadow.slice(0,60)}); })()"
}

# ---- A. settings page with a vault -------------------------------------------------
launch
poll "!!document.querySelector('.tree-row')" || { echo "app not ready"; tail -5 "$WORK/app.log"; exit 1; }
sleep 1.5
ev "window.__mkbApp.getState().openSettings(); true" >/dev/null
poll "!!document.querySelector('[data-testid=effects-blur]')" || { echo "settings not ready"; exit 1; }
sleep 1
for T in "std light" "std dark" "off light" "high light"; do
  set -- $T; tier $1 $2; freeze
  say "settings $1 $2: nav=$(sty '[data-panel=settings-nav]') content=$(sty '[data-panel=settings-content]') card=$(sty '.setting-card')"
  capture "$PREFIX-settings-$1-$2.png"
done

# dialog + wizard + menu, injected with the real class names (opened by user flows elsewhere)
ev "(() => { const host=document.querySelector('.settings-shell'); host.insertAdjacentHTML('beforeend', '<div class=\"settings-dialog-backdrop\" id=\"inj-dlg\" role=\"presentation\"><div class=\"settings-dialog settings-dialog-narrow\"><div class=\"settings-dialog-head\"><strong>有 2 处未保存的改动</strong></div><p class=\"field-hint\">模型服务、Agents。关闭设置或切换分节会丢弃它们。</p><div class=\"settings-dialog-actions\"><button class=\"btn-quiet\">继续编辑</button></div></div></div>'); return true; })()" >/dev/null
sleep 0.5
for T in "std light" "off light"; do
  set -- $T; tier $1 $2; freeze
  say "injected dialog $1 $2: dialog=$(sty '#inj-dlg .settings-dialog') scrim=$(raw "JSON.stringify({f:getComputedStyle(document.querySelector('#inj-dlg'),'::before').backdropFilter,b:getComputedStyle(document.querySelector('#inj-dlg'),'::before').backgroundColor})")"
  capture "$PREFIX-dialog-$1-$2.png"
done
ev "document.getElementById('inj-dlg').remove(); true" >/dev/null
ev "(() => { document.body.insertAdjacentHTML('beforeend', '<div class=\"skill-wizard-backdrop\" id=\"inj-wiz\"><div class=\"skill-wizard\"><strong>导入技能</strong></div></div><div id=\"inj-menu\" class=\"setting-card\" style=\"position:fixed;top:80px;right:40px;z-index:50\"><div class=\"provider-menu\" style=\"position:static\"><button class=\"provider-menu-item\">编辑</button><button class=\"provider-menu-item\">删除</button></div></div>'); return true; })()" >/dev/null
tier std light
sleep 0.4
say "injected wizard: card=$(sty '#inj-wiz .skill-wizard') menu=$(sty '#inj-menu .provider-menu')"
ev "document.getElementById('inj-wiz').remove(); document.getElementById('inj-menu').remove(); true" >/dev/null
stop_app

# ---- B. welcome page without a vault ----------------------------------------------
python -I - "$APPD/Mile Knowledge Base" <<'PY3'
import json, os, sys
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": False, "sidebarCollapsed": False,
                  "agnesControlsCollapsed": False, "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [], "lastVaultId": ""}
open(os.path.join(sys.argv[1], "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
PY3
launch
poll "!!document.querySelector('.welcome-shell')" || { echo "welcome not ready"; tail -5 "$WORK/app.log"; exit 1; }
sleep 1.5
for T in "std light" "std dark" "off light" "high light"; do
  set -- $T; tier $1 $2; freeze
  say "welcome $1 $2: shell=$(sty '.welcome-shell') hero=$(sty '.welcome-hero')"
  capture "$PREFIX-welcome-$1-$2.png"
done
tier std light
ev "[...document.querySelectorAll('.welcome-actions button')].find((b) => b.textContent.includes('新建'))?.click(); true" >/dev/null
poll "!!document.querySelector('.onboarding')" || say "onboarding guide did not open"
sleep 0.8
for T in "std light" "off light"; do
  set -- $T; tier $1 $2; freeze
  say "guide $1 $2: card=$(sty '.onboarding') scrim=$(raw "JSON.stringify({f:getComputedStyle(document.querySelector('.welcome-guide-backdrop'),'::before').backdropFilter})")"
  capture "$PREFIX-guide-$1-$2.png"
done
