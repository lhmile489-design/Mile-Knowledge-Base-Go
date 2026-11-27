#!/usr/bin/env bash
# M10 T00 tuning experiment (Q-M10-2): the shipped high values are nearly
# indistinguishable from std in WebView2. This run measures two candidate
# replacements in the same real window:
#   B: stronger colour-field blobs at high (glass opacity unchanged)
#   C: B plus slightly clearer glass at high
# Each candidate is compared against a std capture from the same session and
# its text contrast is sampled from pixels. Nothing is written to the product.
#
# Usage (repo root):  bash spikes/m10-effects/blur/tune.sh
set -u
export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
WORK=$(mktemp -d "$HERE/.tune-XXXXXX")
APP="$WORK/spike.exe"
DRIVE="$WORK/drive.exe"
WEB="$HERE/web"
PORT=18432
export MYGO_AUTOMATION="tcp:127.0.0.1:$PORT"
OUT="$HERE/tune-result.json"

cleanup() {
  powershell.exe -NoProfile -Command 'Get-Process spike -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "*m10-effects*" } | Stop-Process -Force' >/dev/null 2>&1
  rm -rf "$WORK"
}
trap cleanup EXIT

rm -rf "$WEB"
mkdir -p "$WEB/resources"
python -I - "$ROOT/design/r2/global-material.html" "$WEB/index.html" <<'PY'
import sys
text = open(sys.argv[1], encoding="utf-8").read()
text = text.replace("../../src/styles/tokens.css", "tokens.css")
text = text.replace("../../resources/icon.png", "resources/icon.png")
open(sys.argv[2], "w", encoding="utf-8").write(text)
PY
cp "$ROOT/src/styles/tokens.css" "$WEB/tokens.css"
cp "$ROOT/resources/icon.png" "$WEB/resources/icon.png"
(cd "$HERE" && go build -tags e2e -o "$APP" .) || exit 1
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1

ev() {
  python -I - "$WORK/req.json" "$1" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "eval", "params": {"window": 1, "script": sys.argv[2]}}))
PY
  timeout 60 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}
raw() { ev "$1" | python -I -c "import json,sys;d=json.loads(sys.stdin.read());v=d['result']['value'];print(v if isinstance(v,str) else json.dumps(v))"; }
capture() {
  python -I - "$WORK/capreq.json" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "capture", "params": {"window": 1}}))
PY
  timeout 60 "$DRIVE" -f "$WORK/capreq.json" 2>&1 | grep -m1 '{"id"' > "$WORK/cap.json"
  python -I - "$WORK/cap.json" "$1" <<'PY'
import base64, json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
open(sys.argv[2], "wb").write(base64.b64decode(d["result"]["png"]))
PY
}
wait_ready() {
  for _ in $(seq 1 80); do
    [ "$(raw "document.readyState==='complete' && !!document.querySelector('$1')" 2>/dev/null)" = "true" ] && return 0
    sleep 0.3
  done
  return 1
}

HIDE="(()=>{let s=document.getElementById('__m10hide');if(!s){s=document.createElement('style');s.id='__m10hide';s.textContent='*{color:transparent!important;-webkit-text-fill-color:transparent!important;text-shadow:none!important}svg{stroke:transparent!important}';document.head.appendChild(s)}return 'ok'})()"
UNHIDE="(()=>{document.getElementById('__m10hide')?.remove();return 'ok'})()"

CSS_B='[data-blur="high"] .field .a{background:radial-gradient(circle,color-mix(in srgb,var(--raw-accent) 50%,transparent),transparent 68%)!important}
[data-blur="high"] .field .b{background:radial-gradient(circle,color-mix(in srgb,var(--info) 33%,transparent),transparent 68%)!important}
[data-blur="high"] .field .c{background:radial-gradient(circle,color-mix(in srgb,var(--live) 24%,transparent),transparent 68%)!important}
[data-theme="dark"][data-blur="high"] .field .a{background:radial-gradient(circle,color-mix(in srgb,var(--raw-accent) 30%,transparent),transparent 68%)!important}
[data-theme="dark"][data-blur="high"] .field .b{background:radial-gradient(circle,color-mix(in srgb,var(--info) 27%,transparent),transparent 68%)!important}
[data-theme="dark"][data-blur="high"] .field .c{background:radial-gradient(circle,color-mix(in srgb,var(--live) 14%,transparent),transparent 68%)!important}'
CSS_C="$CSS_B
[data-blur=\"high\"]{--glass-panel:color-mix(in srgb,var(--raw-card) 42%,transparent);--glass-card:color-mix(in srgb,var(--raw-card) 36%,transparent);--glass-strong:color-mix(in srgb,var(--raw-card) 66%,transparent)}
[data-theme=\"dark\"][data-blur=\"high\"]{--glass-panel:color-mix(in srgb,var(--raw-raised) 38%,transparent);--glass-card:color-mix(in srgb,var(--raw-raised) 32%,transparent);--glass-strong:color-mix(in srgb,var(--raw-raised) 62%,transparent)}"
CSS_D='[data-blur="high"] .field .a{background:radial-gradient(circle,color-mix(in srgb,var(--raw-accent) 60%,transparent),transparent 68%)!important}
[data-blur="high"] .field .b{background:radial-gradient(circle,color-mix(in srgb,var(--info) 40%,transparent),transparent 68%)!important}
[data-blur="high"] .field .c{background:radial-gradient(circle,color-mix(in srgb,var(--live) 28%,transparent),transparent 68%)!important}
[data-theme="dark"][data-blur="high"] .field .a{background:radial-gradient(circle,color-mix(in srgb,var(--raw-accent) 36%,transparent),transparent 68%)!important}
[data-theme="dark"][data-blur="high"] .field .b{background:radial-gradient(circle,color-mix(in srgb,var(--info) 32%,transparent),transparent 68%)!important}
[data-theme="dark"][data-blur="high"] .field .c{background:radial-gradient(circle,color-mix(in srgb,var(--live) 17%,transparent),transparent 68%)!important}
[data-blur="high"]{--glass-panel:color-mix(in srgb,var(--raw-card) 34%,transparent);--glass-card:color-mix(in srgb,var(--raw-card) 28%,transparent);--glass-strong:color-mix(in srgb,var(--raw-card) 62%,transparent);--glass-blur:40px;--glass-sat:2}
[data-theme="dark"][data-blur="high"]{--glass-panel:color-mix(in srgb,var(--raw-raised) 30%,transparent);--glass-card:color-mix(in srgb,var(--raw-raised) 24%,transparent);--glass-strong:color-mix(in srgb,var(--raw-raised) 56%,transparent)}'
CSS_E='[data-blur="high"] .field .a{background:radial-gradient(circle,color-mix(in srgb,var(--raw-accent) 60%,transparent),transparent 68%)!important}
[data-blur="high"] .field .b{background:radial-gradient(circle,color-mix(in srgb,var(--info) 40%,transparent),transparent 68%)!important}
[data-blur="high"] .field .c{background:radial-gradient(circle,color-mix(in srgb,var(--live) 28%,transparent),transparent 68%)!important}
[data-theme="dark"][data-blur="high"] .field .a{background:radial-gradient(circle,color-mix(in srgb,var(--raw-accent) 36%,transparent),transparent 68%)!important}
[data-theme="dark"][data-blur="high"] .field .b{background:radial-gradient(circle,color-mix(in srgb,var(--info) 32%,transparent),transparent 68%)!important}
[data-theme="dark"][data-blur="high"] .field .c{background:radial-gradient(circle,color-mix(in srgb,var(--live) 17%,transparent),transparent 68%)!important}
[data-blur="high"]{--glass-panel:color-mix(in srgb,var(--raw-card) 34%,transparent);--glass-card:color-mix(in srgb,var(--raw-card) 28%,transparent);--glass-strong:color-mix(in srgb,var(--raw-card) 70%,transparent);--glass-blur:40px;--glass-sat:2}
[data-theme="dark"][data-blur="high"]{--glass-panel:color-mix(in srgb,var(--raw-raised) 30%,transparent);--glass-card:color-mix(in srgb,var(--raw-raised) 24%,transparent);--glass-strong:color-mix(in srgb,var(--raw-raised) 68%,transparent)}'

set_tune() { # css or empty to remove
  if [ -z "$1" ]; then
    ev "(()=>{document.getElementById('__m10tune')?.remove();return 'ok'})()" >/dev/null
  else
    python -I - "$WORK/tunereq.json" "$1" <<'PY'
import json, sys
css = sys.argv[2]
script = "(()=>{let s=document.getElementById('__m10tune');if(!s){s=document.createElement('style');s.id='__m10tune';document.head.appendChild(s)}s.textContent=%s;return 'ok'})()" % json.dumps(css)
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "eval", "params": {"window": 1, "script": script}}))
PY
    timeout 60 "$DRIVE" -f "$WORK/tunereq.json" >/dev/null 2>&1
  fi
}

run_state() { # state probe selector
  local state="$1" probe="$2" sel="$3"
  ev "(()=>{location.href='/?state=$state&theme=light&blur=std&shot=1&motion=off';return 'nav'})()" >/dev/null
  wait_ready "$sel" || { echo "$state not ready"; return 1; }
  sleep 0.6
  for theme in light dark; do
    set_tune ""
    ev "(()=>{const r=document.documentElement;r.dataset.theme='$theme';r.dataset.blur='std';return 'ok'})()" >/dev/null
    sleep 0.4
    capture "$WORK/base-$state-$theme.png"
    for cand in B C D E; do
      case "$cand" in
        B) set_tune "$CSS_B" ;;
        C) set_tune "$CSS_C" ;;
        D) set_tune "$CSS_D" ;;
        E) set_tune "$CSS_E" ;;
      esac
      ev "(()=>{document.documentElement.dataset.blur='high';return 'ok'})()" >/dev/null
      sleep 0.4
      local tag="cand$cand-$state-$theme"
      raw "$(cat "$probe")" > "$WORK/$tag.probe.json"
      capture "$WORK/$tag.png"
      if [ "$cand" = "E" ]; then
        cp "$WORK/$tag.png" "$ROOT/spikes/m10-effects/shots/tune-candE-$state-$theme.png"
      fi
      ev "$HIDE" >/dev/null
      sleep 0.25
      capture "$WORK/$tag.bg.png"
      ev "$UNHIDE" >/dev/null
      sleep 0.15
      echo "done $tag"
    done
  done
  set_tune ""
}

APPD="$WORK/appdata"
mkdir -p "$APPD"
APPDATA="$(cygpath -w "$APPD")" MYGO_FRONTEND_DIST="$WEB" "$APP" >"$WORK/app.log" 2>&1 &
if ! wait_ready ".app"; then echo "window not ready"; tail -10 "$WORK/app.log"; exit 1; fi
run_state work-doc "$HERE/probe-work.js" ".reader" || true
run_state menu "$HERE/probe-menu.js" ".menu" || true
python -I "$HERE/tune.py" "$WORK" "$OUT"
STATUS=$?
echo "TUNE_FILE=$OUT"
exit $STATUS
