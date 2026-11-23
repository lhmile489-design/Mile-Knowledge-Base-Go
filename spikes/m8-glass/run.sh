#!/usr/bin/env bash
# M8 T00 driver: measure scroll frame pacing with glass on/off in a real
# mygo + WebView2 window. Writes raw JSON lines to stdout; run from the repo
# root:  bash spikes/m8-glass/run.sh
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
WORK=$(mktemp -d "$HERE/.run-XXXXXX")
APP="$WORK/app.exe"
DRIVE="$WORK/drive.exe"
PORT=18421
export MYGO_AUTOMATION="tcp:127.0.0.1:$PORT"

kill_spike() {
  powershell.exe -NoProfile -Command 'Get-Process app -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "*m8-glass*" } | Stop-Process -Force' >/dev/null 2>&1
}

cleanup() {
  kill_spike
  rm -rf "$WORK"
}
trap cleanup EXIT

(cd "$HERE" && go build -tags e2e -o "$APP" .) || exit 1
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1

ev() { # script -> raw automation response line
  python -I - "$WORK/req.json" "$1" <<'PY'
import json,sys
open(sys.argv[1],"w",encoding="utf-8").write(json.dumps({"id":1,"method":"eval","params":{"window":1,"script":sys.argv[2]}}))
PY
  timeout 40 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}
raw() { ev "$1" | python -I -c "import json,sys;d=json.loads(sys.stdin.read());v=d['result']['value'];print(v if isinstance(v,str) else json.dumps(v))"; }

wait_port_free() {
  for _ in $(seq 1 40); do
    python -I -c "import socket,sys;s=socket.socket();s.settimeout(.2);sys.exit(0 if s.connect_ex(('127.0.0.1',$PORT))!=0 else 1)" && return 0
    sleep 0.3
  done
  return 1
}

wait_ready() { # size-tag
  for _ in $(seq 1 60); do
    [ "$(raw "window.__spikeSize === '$1' && document.querySelectorAll('.card').length === 200" 2>/dev/null)" = "true" ] && return 0
    sleep 0.5
  done
  return 1
}

measure() { # glass(on|off)
  ev "document.documentElement.dataset.glass='$1'; true" >/dev/null
  sleep 0.6
  ev "window.__spike.start(6000)" >/dev/null
  for _ in $(seq 1 50); do
    if [ "$(raw "!!(window.__spike.last && window.__spike.last.done)")" = "true" ]; then
      raw "window.__spike.result()"
      return 0
    fi
    sleep 0.3
  done
  echo '{"error":"timeout"}'
}

for size in 1440x900 960x720; do
  W=${size%x*}; H=${size#*x}
  wait_port_free || { echo "{\"size\":\"$size\",\"error\":\"port busy\"}"; continue; }
  MYGO_FRONTEND_DIST="$HERE/web" SPIKE_W="$W" SPIKE_H="$H" SPIKE_TAG="$size" "$APP" >"$WORK/app-$size.log" 2>&1 &
  APP_PID=$!
  if ! wait_ready "$size"; then
    echo "{\"size\":\"$size\",\"error\":\"cards not ready\"}"
  else
    echo "{\"size\":\"$size\",\"kind\":\"load\",\"value\":$(raw "window.__spike.result()")}"
    for round in 1 2 3; do
      for glass in on off; do
        r=$(measure "$glass")
        echo "{\"size\":\"$size\",\"round\":$round,\"glass\":\"$glass\",\"value\":$r}"
      done
    done
  fi
  kill_spike
  APP_PID=""
  sleep 1.5
done
