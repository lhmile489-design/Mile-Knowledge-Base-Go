#!/usr/bin/env bash
# M10 T00 driver: open the approved global-material prototype in a real
# mygo + WebView2 window, capture the four blur levels in both themes for the
# work-doc and menu states, then sample text contrast from the pixels.
#
# Nothing real is touched: the spike app runs with a temp APPDATA, and the
# page is generated from design/r2/global-material.html plus src/styles/tokens.css.
#
# Usage (repo root):  bash spikes/m10-effects/blur/run.sh
set -u
export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
WORK=$(mktemp -d "$HERE/.run-XXXXXX")
APP="$WORK/spike.exe"
DRIVE="$WORK/drive.exe"
WEB="$HERE/web"
SHOTS="$ROOT/spikes/m10-effects/shots"
RESULT="$HERE/result.json"
PORT=18431
export MYGO_AUTOMATION="tcp:127.0.0.1:$PORT"
APP_PID=""
mkdir -p "$SHOTS"

cleanup() {
  powershell.exe -NoProfile -Command 'Get-Process spike -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "*m10-effects*" } | Stop-Process -Force' >/dev/null 2>&1
  rm -rf "$WORK"
}
trap cleanup EXIT

# --- web/ derived from the approved prototype -------------------------------
rm -rf "$WEB"
mkdir -p "$WEB/resources"
python -I - "$ROOT/design/r2/global-material.html" "$WEB/index.html" <<'PY'
import sys
src, out = sys.argv[1], sys.argv[2]
text = open(src, encoding="utf-8").read()
text = text.replace("../../src/styles/tokens.css", "tokens.css")
text = text.replace("../../resources/icon.png", "resources/icon.png")
open(out, "w", encoding="utf-8").write(text)
PY
cp "$ROOT/src/styles/tokens.css" "$WEB/tokens.css"
cp "$ROOT/resources/icon.png" "$WEB/resources/icon.png"

(cd "$HERE" && go build -tags e2e -o "$APP" .) || exit 1
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1

ev() { # script -> raw automation response line
  python -I - "$WORK/req.json" "$1" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "eval", "params": {"window": 1, "script": sys.argv[2]}}))
PY
  timeout 60 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}
raw() { ev "$1" | python -I -c "import json,sys;d=json.loads(sys.stdin.read());v=d['result']['value'];print(v if isinstance(v,str) else json.dumps(v))"; }
capture() { # out.png
  python -I - "$WORK/capreq.json" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "capture", "params": {"window": 1}}))
PY
  timeout 60 "$DRIVE" -f "$WORK/capreq.json" 2>&1 | grep -m1 '{"id"' > "$WORK/cap.json"
  python -I - "$WORK/cap.json" "$1" <<'PY'
import base64, json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
open(sys.argv[2], "wb").write(base64.b64decode(d["result"]["png"]))
print("capture %s %sx%s" % (sys.argv[2], d["result"]["width"], d["result"]["height"]))
PY
}

wait_ready() { # css selector
  for _ in $(seq 1 80); do
    [ "$(raw "document.readyState==='complete' && !!document.querySelector('$1')" 2>/dev/null)" = "true" ] && return 0
    sleep 0.3
  done
  return 1
}

HIDE="(()=>{let s=document.getElementById('__m10hide');if(!s){s=document.createElement('style');s.id='__m10hide';s.textContent='*{color:transparent!important;-webkit-text-fill-color:transparent!important;text-shadow:none!important}svg{stroke:transparent!important}';document.head.appendChild(s)}return 'ok'})()"
UNHIDE="(()=>{document.getElementById('__m10hide')?.remove();return 'ok'})()"

run_state() { # state probe-file selector
  local state="$1" probe="$2" sel="$3"
  ev "(()=>{location.href='/?state=$state&theme=light&blur=std&shot=1&motion=off';return 'nav'})()" >/dev/null
  wait_ready "$sel" || { echo "{\"error\":\"$state not ready\"}"; return 1; }
  sleep 0.6
  for theme in light dark; do
    for blur in off low std high; do
      local tag="$state-$blur-$theme"
      ev "(()=>{const r=document.documentElement;r.dataset.theme='$theme';r.dataset.blur='$blur';return 'ok'})()" >/dev/null
      sleep 0.4
      raw "$(cat "$probe")" > "$WORK/$tag.probe.json"
      capture "$SHOTS/$tag.png"
      ev "$HIDE" >/dev/null
      sleep 0.25
      capture "$WORK/$tag.bg.png"
      ev "$UNHIDE" >/dev/null
      sleep 0.15
      echo "done $tag"
    done
  done
}

APPD="$WORK/appdata"
mkdir -p "$APPD"
APPDATA="$(cygpath -w "$APPD")" MYGO_FRONTEND_DIST="$WEB" "$APP" >"$WORK/app.log" 2>&1 &
APP_PID=$!

if ! wait_ready ".app"; then
  echo "spike window did not become ready"; tail -20 "$WORK/app.log"; exit 1
fi

run_state work-doc "$HERE/probe-work.js" ".reader" || true
run_state menu "$HERE/probe-menu.js" ".menu" || true

python -I "$HERE/sample.py" "$WORK" "$SHOTS" "$RESULT"
STATUS=$?
echo "RESULT_FILE=$RESULT"
exit $STATUS
