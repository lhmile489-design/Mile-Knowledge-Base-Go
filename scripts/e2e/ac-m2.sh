#!/usr/bin/env bash
# AC-M2: one end-to-end pass through the AI loop on a real window with a fake
# provider (apiKeyEnv empty, nothing written to the credential store):
# engine ready -> route -> stream -> tool approval -> receipt -> restart
# restore -> history search.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.ac-m2-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"
mkdir -p "$VAULT" "$APPD/Mile Knowledge base"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null)
VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge base" <<'PY'
import hashlib,json,os,sys
v,out=sys.argv[1],sys.argv[2]; c=os.path.abspath(v).replace('/',chr(92)).lower(); vid=hashlib.sha256(c.encode()).hexdigest()[:16]
doc={"version":1,"theme":"light","closeBehavior":"quit","layout":{"sidebarWidth":232,"docWidth":620,"docCollapsed":False,"sidebarCollapsed":False,"agnesControlsCollapsed":False,"agnesHistoryOpen":True,"agnesComposerHeight":96},"recentVaults":[{"id":vid,"root":v,"displayName":"ac-m2","lastOpenedAt":"2026-11-19T09:00:00Z"}],"lastVaultId":vid}
open(os.path.join(out,"settings.json"),"w",encoding="utf-8").write(json.dumps(doc,ensure_ascii=False,indent=2)+"\n"); print(vid)
PY
)
PORT_AI=18412
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

PORT=18411
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
wait_ready() {
  for _ in $(seq 1 60); do
    local out; out=$(ev "window.__mkbComposer.getState().session?.engine?.phase ?? 'none'")
    case "$out" in *'"value":"ready"'*) return 0;; esac
    sleep 0.5
  done
  return 1
}
ASSISTANT_COUNT="(()=>{const s=window.__mkbAgent.getState();return Object.values(s.buckets).reduce((n,b)=>n+b.transcript.lines.filter(l=>l.role==='agent'&&l.text).length,0)})()"
BUSY="(()=>{const s=window.__mkbAgent.getState();return Object.values(s.buckets).some(b=>b.busy)})()"
count_assistants() {
  ev "$ASSISTANT_COUNT" | sed 's/.*"value":\([0-9]*\).*/\1/'
}
send_message() { # text
  ev "(()=>{const c=window.__mkbComposer.getState();c.setText('$1');c.send('$VID');return 'sent'})()" >/dev/null
}
wait_turn() { # label before
  local label=$1 before=$2 i out count
  for i in $(seq 1 60); do
    count=$(count_assistants)
    if [ "${count:-0}" -gt "$before" ]; then
      out=$(ev "$BUSY")
      case "$out" in *'"value":false'*) pass=$((pass+1)); echo "PASS $label"; return 0;; esac
    fi
    sleep 0.4
  done
  fail=$((fail+1)); echo "FAIL $label :: assistants=$count"; return 1
}

start_app
wait_ready && { pass=$((pass+1)); echo "PASS engine ready"; } || { fail=$((fail+1)); echo "FAIL engine ready"; }

echo "== route and stream"
ev "(()=>{window.__mkbComposer.getState().setText('帮我整理归档这个目录');return 'typed'})()" >/dev/null
wait_match "route preview targets curator" "window.__mkbComposer.getState().preview?.agent ?? 'none'" '"value":"curator"' 20
check "route line shows 发给 整理归档" "document.querySelector('[data-testid=route-target]')?.innerText.includes('整理归档')" '"value":true'
BEFORE=$(count_assistants); BEFORE=${BEFORE:-0}
send_message "帮我整理归档这个目录"
wait_turn "first turn streamed to end" "$BEFORE"
check "answer text rendered" "[...document.querySelectorAll('[data-testid=chat-answer]')].some(e=>e.innerText.includes('整理归档'))" '"value":true'
check "usage receipt shows this turn" "document.querySelector('[data-testid=usage-receipt]')?.innerText.includes('本轮')" '"value":true'
check "cost is unknown, not zero" "document.querySelector('[data-testid=usage-receipt]')?.innerText.includes('未提供')" '"value":true'

echo "== tool approval"
BEFORE=$(count_assistants); BEFORE=${BEFORE:-0}
send_message "把结论写入 raws/ac.md"
wait_match "approval card appears" "!!document.querySelector('[data-testid=permission-card]')" '"value":true' 40
check "card names the target file" "document.querySelector('[data-testid=permission-card]')?.innerText.includes('raws/ac.md')" '"value":true'
check "card names the verb" "document.querySelector('[data-testid=permission-card]')?.innerText.includes('写入')" '"value":true'
ev "(()=>{document.querySelector('[data-testid=permission-card] .perm-actions .btn-primary')?.click();return 'clicked'})()" >/dev/null
wait_match "file written after allow" "window.__mkbApi.doc.read('$VID','raws/ac.md').then(r=>!!r.doc).catch(()=>false)" '"value":true' 40
wait_match "card removed after the Go frame" "!document.querySelector('[data-testid=permission-card]')" '"value":true' 40
wait_turn "tool turn finished" "$BEFORE"
check "tool answer rendered" "[...document.querySelectorAll('[data-testid=chat-answer]')].some(e=>e.innerText.includes('已写入'))" '"value":true'

echo "== history search"
ev "(()=>{const h=window.__mkbHistory.getState();if(!h.open)h.toggleRail();return 'opened'})()" >/dev/null
wait_match "history rail visible" "!!document.querySelector('[data-testid=history-rail]')" '"value":true' 20
ev "(()=>{window.__mkbHistory.getState().setSearch('$VID','归档');return 'typed'})()" >/dev/null
wait_match "search hits the conversation" "(window.__mkbHistory.getState().hits.length ?? 0) >= 1" '"value":true' 20
check "search hit rendered" "!!document.querySelector('[data-testid=history-hit]')" '"value":true'

echo "== restart: restore and search survive"
COUNT_BEFORE=$(ev "window.__mkbHistory.getState().index?.conversations.length ?? 0" | sed 's/.*"value":\([0-9]*\).*/\1/')
COUNT_BEFORE=${COUNT_BEFORE:-1}
stop_app
start_app
wait_ready && { pass=$((pass+1)); echo "PASS engine ready after restart"; } || { fail=$((fail+1)); echo "FAIL engine ready after restart"; }
wait_match "chat restored after restart" "Object.values(window.__mkbAgent.getState().buckets).some(b=>b.transcript.lines.some(l=>l.role==='agent'&&l.text&&l.text.includes('归档')))" '"value":true' 40
check "history rail restored open" "!!document.querySelector('[data-testid=history-rail]')" '"value":true'
check "history list survived" "(window.__mkbHistory.getState().index?.conversations.length ?? 0) >= $COUNT_BEFORE" '"value":true'
ev "(()=>{window.__mkbHistory.getState().setSearch('$VID','归档');return 'typed'})()" >/dev/null
wait_match "search still hits after restart" "(window.__mkbHistory.getState().hits.length ?? 0) >= 1" '"value":true' 20

echo "RESULT pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
