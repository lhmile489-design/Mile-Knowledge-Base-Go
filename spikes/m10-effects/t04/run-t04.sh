#!/usr/bin/env bash
# M10 T04 driver: open Settings in a real window (temp APPDATA, vault and HOME),
# drive the 动画 / 模糊 rows, read the persisted settings.json and take shots.
#
# Usage (repo root):
#   bash spikes/m10-effects/t04/run-t04.sh <app.exe>
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
PORT=18462
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
SETTINGS_JSON="$APPD/Mile Knowledge Base/settings.json"
SNAP="$HERE/state.txt"
: > "$SNAP"
say() { echo "$*" | tee -a "$SNAP"; }
persisted() { python -I -c "import json,sys;d=json.load(open(sys.argv[1],encoding='utf-8'));print(json.dumps(d.get('effects')))" "$SETTINGS_JSON"; }
freeze() {
  ev "(() => { if (document.activeElement && document.activeElement.blur) document.activeElement.blur(); document.getAnimations().forEach((a) => { try { a.pause(); a.currentTime = 0; } catch {} }); return 'ok'; })()" >/dev/null
  sleep 0.4
}
html_attrs() { raw "(() => { const r=document.documentElement.dataset; return JSON.stringify({theme:r.theme,motion:r.motion,blur:r.blur,blurAuto:r.blurAuto||null,field:!!document.querySelector('.background-field')}); })()"; }
checked() { raw "(() => { const out = []; for (const k of ['motion', 'blur']) { out.push([...document.querySelectorAll('[data-testid^=effects-' + k + '-][role=radio]')].map((b) => b.textContent + (b.getAttribute('aria-checked') === 'true' ? '*' : '')).join(' ')); } return JSON.stringify(out); })()"; }
notes() { raw "(() => { const m = document.querySelector('[data-testid=effects-motion-note]'); const b = document.querySelector('[data-testid=effects-blur-note]'); return JSON.stringify({ motion: m ? m.textContent : null, blur: b ? b.textContent : null }); })()"; }
click_id() { ev "(() => { const el=document.querySelector('[data-testid=\"$1\"]'); if(!el) throw new Error('missing $1'); el.click(); return true; })()" >/dev/null; }

ev "window.__mkbApp.getState().openSettings(); true" >/dev/null
poll "!!document.querySelector('[data-testid=effects-blur]')" || { echo "settings not ready"; exit 1; }
sleep 1
say "== default"
say "html: $(html_attrs)"
say "levels: $(checked)"
say "notes: $(notes)"
say "persisted: $(persisted)"
freeze; capture "$HERE/shot-default-light.png"

say "== click 动画 关"
click_id effects-motion-off
poll "document.documentElement.dataset.motion === 'off'" || say "motion attr did not change"
sleep 0.6
say "html: $(html_attrs)"
say "levels: $(checked)"
say "persisted: $(persisted)"

say "== click 模糊 弱"
click_id effects-blur-low
poll "document.documentElement.dataset.blur === 'low'" || say "blur attr did not change"
sleep 0.6
say "html: $(html_attrs)"
say "levels: $(checked)"
say "persisted: $(persisted)"

say "== click 动画 标准 and 模糊 标准 (back to the default for the shots)"
click_id effects-motion-std
click_id effects-blur-std
sleep 0.8
say "html: $(html_attrs)"
say "persisted: $(persisted)"

say "== notes visible"
ev "window.__mkbApp.setState({ effectsDegraded: true, systemReducedMotion: true }); true" >/dev/null
sleep 0.6
say "notes: $(notes)"
say "levels: $(checked)"
freeze; capture "$HERE/shot-notes-light.png"

say "== dark"
ev "document.documentElement.dataset.theme='dark'; true" >/dev/null
sleep 0.5
freeze; capture "$HERE/shot-notes-dark.png"

say "== blur off removes the field"
ev "document.documentElement.dataset.theme='light'; true" >/dev/null
click_id effects-blur-off
poll "document.documentElement.dataset.blur === 'off'" || say "blur attr did not change"
sleep 0.6
say "html: $(html_attrs)"
say "persisted: $(persisted)"
freeze; capture "$HERE/shot-blur-off-light.png"
