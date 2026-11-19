#!/usr/bin/env bash
# G-PERF for M2 (T20): stream frame pacing, composer key latency during a
# stream, history viewer first frame and message virtualization, measured
# through the same automation channel the acceptance scripts use. A fake
# provider streams a 10k-character answer; the 50-conversation sample reuses
# that answer's text in the real event format.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.perf-ai-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"
mkdir -p "$VAULT" "$APPD/Mile Knowledge base"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null)
VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge base" <<'PY'
import hashlib,json,os,sys
v,out=sys.argv[1],sys.argv[2]; c=os.path.abspath(v).replace('/',chr(92)).lower(); vid=hashlib.sha256(c.encode()).hexdigest()[:16]
doc={"version":1,"theme":"light","closeBehavior":"quit","layout":{"sidebarWidth":232,"docWidth":620,"docCollapsed":False,"sidebarCollapsed":False,"agnesControlsCollapsed":False,"agnesHistoryOpen":True,"agnesComposerHeight":96},"recentVaults":[{"id":vid,"root":v,"displayName":"perf-ai","lastOpenedAt":"2026-11-19T09:00:00Z"}],"lastVaultId":vid}
open(os.path.join(out,"settings.json"),"w",encoding="utf-8").write(json.dumps(doc,ensure_ascii=False,indent=2)+"\n"); print(vid)
PY
)
PORT_AI=18422
python -I - "$VAULT" "$PORT_AI" <<'PY'
import json,os,sys
vault,port=sys.argv[1],sys.argv[2]
path=os.path.join(vault,".mile-kb","settings","providers.json")
os.makedirs(os.path.dirname(path),exist_ok=True)
doc={"version":1,"providers":[{"name":"fake","kind":"openai","baseUrl":"http://127.0.0.1:"+port,"models":["fake-model"],"defaultModel":"fake-model","apiKeyEnv":""}]}
open(path,"w",encoding="utf-8").write(json.dumps(doc,ensure_ascii=False,indent=2)+"\n")
PY
DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1
python -I "$ROOT/scripts/e2e/fake-openai.py" --port "$PORT_AI" >"$WORK/fake-ai.log" 2>&1 &
AI_PID=$!
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then taskkill //F //PID "$APP_PID" >/dev/null 2>&1; fi
  taskkill //F //IM "Mile Knowledge Base.exe" >/dev/null 2>&1
  if [ -n "${AI_PID:-}" ]; then kill "$AI_PID" >/dev/null 2>&1; fi
  rm -rf "$WORK"
}
trap cleanup EXIT
for _ in $(seq 1 50); do
  code=$(curl.exe -s -o NUL -w "%{http_code}" "http://127.0.0.1:$PORT_AI/health")
  [ "$code" = "200" ] && break
  sleep 0.2
done
PORT=18421
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
  timeout 60 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}
ev_json() {
  local out; out=$(ev "$1")
  python -I - "$out" <<'PY'
import json,sys
try:
    doc=json.loads(sys.argv[1])
    value=doc.get("result",{}).get("value")
    sys.stdout.write(value if isinstance(value,str) else json.dumps(value))
except Exception:
    sys.stdout.write("")
PY
}
wait_ready() {
  for _ in $(seq 1 60); do
    ev "window.__mkbComposer.getState().session?.engine?.phase ?? 'none'" | grep -q '"value":"ready"' && return 0
    sleep 0.5
  done
  return 1
}
RESULT="$WORK/result.txt"; : > "$RESULT"
report() { echo "$1" | tee -a "$RESULT"; }

start_app
if ! wait_ready; then report "engine=not-ready"; exit 1; fi
report "engine=ready"

echo "== stream one long answer with probes armed"
ev "(()=>{window.__frames=[];window.__lat=[];window.__probeOn=true;window.__lastFrame=0;const tick=(now)=>{if(!window.__probeOn)return;if(window.__lastFrame)window.__frames.push(now-window.__lastFrame);window.__lastFrame=now;requestAnimationFrame(tick)};requestAnimationFrame(tick);window.__probeTimer=setInterval(()=>{const input=document.querySelector('.composer-input');if(!input)return;const t0=performance.now();input.dispatchEvent(new KeyboardEvent('keydown',{key:'a',bubbles:true}));requestAnimationFrame(()=>window.__lat.push(performance.now()-t0))},50);return 'armed'})()" >/dev/null
ev "(()=>{const c=window.__mkbComposer.getState();c.setText('请给我一段长回答，用于性能采样');c.send('$VID');return 'sent'})()" >/dev/null
sleep 0.5
report "probe_mid=$(ev_json "JSON.stringify({on:!!window.__probeOn,frames:(window.__frames||[]).length,lat:(window.__lat||[]).length})")"
DONE=0
for _ in $(seq 1 80); do
  out=$(ev "(()=>{const s=window.__mkbAgent.getState();return Object.values(s.buckets).some(b=>!b.busy&&b.transcript.lines.some(l=>l.role==='agent'&&l.text&&l.text.length>1000))})()")
  case "$out" in *'"value":true'*) DONE=1; break;; esac
  sleep 0.25
done
[ "$DONE" = "1" ] || report "stream=not-finished"
ev "(()=>{window.__probeOn=false;clearInterval(window.__probeTimer);return 'stopped'})()" >/dev/null
report "stream_frames=$(ev_json "(()=>{const f=window.__frames||[];return JSON.stringify({frames:f.length,slow:f.filter(d=>d>20).length,max:Math.round(Math.max(0,...f)*10)/10})})()")"
report "key_latency=$(ev_json "(()=>{const t=[...(window.__lat||[])].sort((a,b)=>a-b);const p=x=>t.length?t[Math.min(t.length-1,Math.floor(t.length*x))]:0;return JSON.stringify({n:t.length,p50:Math.round(p(0.5)*10)/10,p90:Math.round(p(0.9)*10)/10,max:Math.round(p(0.99)*10)/10})})()")"

echo "== seed one conversation with 50 long answers from the real one"
stop_app
python -I - "$APPD/Mile Knowledge base/conversations/$VID" <<'PY'
import json, os, sys
from datetime import datetime, timedelta, timezone
conv_dir = sys.argv[1]
index_path = os.path.join(conv_dir, "index.json")
index = json.load(open(index_path, encoding="utf-8"))
conversations = index.get("conversations") or []
active_id = index.get("activeId")
template = ""
for meta in conversations:
    if meta.get("id") != active_id:
        continue
    with open(os.path.join(conv_dir, meta["id"] + ".jsonl"), encoding="utf-8") as fh:
        for line in fh:
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            if event.get("t") == "message" and event.get("text"):
                template = event["text"]
if len(template) < 1000:
    raise SystemExit("no long answer to seed from")
base = datetime(2026, 11, 19, 9, 0, 0, tzinfo=timezone.utc)
at = base.strftime("%Y-%m-%dT%H:%M:%SZ")
with open(os.path.join(conv_dir, "perf0001.jsonl"), "w", encoding="utf-8") as fh:
    for i in range(50):
        for event in (
            {"t": "user", "at": at, "agent": "general", "text": "性能样本 %02d" % i},
            {"t": "message", "at": at, "agent": "general", "text": template},
            {"t": "turn-end", "at": at, "agent": "general", "stopReason": "end_turn"},
        ):
            fh.write(json.dumps(event, ensure_ascii=False) + "\n")
meta = {"id": "perf0001", "title": "性能样本（50 条长回答）", "createdAt": at, "updatedAt": at,
        "turnCount": 50, "agents": ["general"], "pinned": False}
index["conversations"] = conversations + [meta]
with open(index_path, "w", encoding="utf-8") as fh:
    json.dump(index, fh, ensure_ascii=False, indent=2)
    fh.write("\n")
print("seeded 50 turns")
PY
start_app
if ! wait_ready; then report "engine=not-ready-after-seed"; exit 1; fi

echo "== viewer first frame and virtualization"
FIRST=$(ev_json "(async()=>{const VID='$VID';const H=()=>window.__mkbHistory.getState();try{await H().refresh(VID);const t0=performance.now();await H().openViewer(VID,'perf0001');while(!document.querySelector('[data-testid=conversation-viewer] [data-testid=chat-answer]')){await new Promise(r=>requestAnimationFrame(r))}return Math.round((performance.now()-t0)*10)/10}catch(e){return String(e)}})()")
report "viewer_first_frame_ms=${FIRST:-n/a}"
RENDERED=$(ev_json "(async()=>{const body=document.querySelector('.viewer-body');const frame=()=>new Promise(r=>requestAnimationFrame(r));let max=0;for(let i=0;i<12;i++){if(!body)break;body.scrollTop=(body.scrollHeight-body.clientHeight)*i/11;await frame();await frame();const n=document.querySelectorAll('.viewer-body .chat-virtual-item').length;if(n>max)max=n}return max})()")
report "viewer_rendered_max=${RENDERED:-n/a}"

echo "RESULT"
cat "$RESULT"
