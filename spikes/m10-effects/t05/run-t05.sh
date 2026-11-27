#!/usr/bin/env bash
# M10 T05 driver: open the workbench with a document and dump background / material
# facts of the panels (probe-bg.js), then take shots in the requested states.
#
# Usage (repo root):
#   bash spikes/m10-effects/t05/run-t05.sh <app.exe> <out-prefix> [light-dark-blur-matrix]
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
PORT=18463
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
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": False, "sidebarCollapsed": False,
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
ev "window.__mkbDocs.getState().open($VID_JS, 'raws/共享笔记.md').then(() => 'ok')" >/dev/null
poll "!!document.querySelector('[data-panel=editor] .reader-view, [data-panel=editor] .reader, [data-panel=editor] article')" || echo 'reader selector not found (continuing)'
sleep 1.2
freeze() {
  ev "(() => { if (document.activeElement && document.activeElement.blur) document.activeElement.blur(); document.getAnimations().forEach((a) => { try { a.pause(); a.currentTime = 0; } catch {} }); return 'ok'; })()" >/dev/null
  sleep 0.4
}
PREFIX="${2:-$HERE/out}"
raw "$(cat "$HERE/probe-bg.js")" > "${PREFIX}-bg.txt"
raw "$(cat "$HERE/probe-tree.js")" > "${PREFIX}-tree.txt"
echo "wrote ${PREFIX}-bg.txt"
freeze; capture "${PREFIX}-light.png"
ev "document.documentElement.dataset.theme='dark'; true" >/dev/null; sleep 0.5
freeze; capture "${PREFIX}-dark.png"
ev "document.documentElement.dataset.theme='light'; true" >/dev/null
ev "document.documentElement.dataset.blur='off'; true" >/dev/null; sleep 0.5
freeze; capture "${PREFIX}-blur-off.png"
raw "$(cat "$HERE/probe-bg.js")" > "${PREFIX}-bg-off.txt"

# low / high tiers, light and dark (advisor: look at them before T10)
for TIER in low high; do
  ev "document.documentElement.dataset.blur='$TIER'; document.documentElement.dataset.theme='light'; true" >/dev/null; sleep 0.5
  freeze; capture "${PREFIX}-blur-$TIER-light.png"
  ev "document.documentElement.dataset.theme='dark'; true" >/dev/null; sleep 0.5
  freeze; capture "${PREFIX}-blur-$TIER-dark.png"
done
