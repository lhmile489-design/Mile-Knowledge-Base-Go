#!/usr/bin/env bash
# AC-M1: one end-to-end pass through the M1 loop on a real window:
# search -> open -> backlinks -> create -> rename -> archive -> restore ->
# restart and check that the index and the audit log survived.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.ac-m1-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"
mkdir -p "$VAULT" "$APPD/Mile Knowledge base"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null)
VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge base" <<'PY'
import hashlib,json,os,sys
v,out=sys.argv[1],sys.argv[2]; c=os.path.abspath(v).replace('/',chr(92)).lower(); vid=hashlib.sha256(c.encode()).hexdigest()[:16]
doc={"version":1,"theme":"light","closeBehavior":"quit","layout":{"sidebarWidth":232,"docWidth":620,"docCollapsed":False,"sidebarCollapsed":False,"agnesControlsCollapsed":False,"agnesHistoryOpen":True,"agnesComposerHeight":96},"recentVaults":[{"id":vid,"root":v,"displayName":"ac-m1","lastOpenedAt":"2026-11-18T09:00:00Z"}],"lastVaultId":vid}
open(os.path.join(out,"settings.json"),"w",encoding="utf-8").write(json.dumps(doc,ensure_ascii=False,indent=2)+"\n"); print(vid)
PY
)
DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1

PORT=18401
start_app() {
  (APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" "$APP" >"$WORK/app.log" 2>&1 &)
  sleep 8
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
wait_ready() {
  for _ in $(seq 1 60); do
    local out; out=$(ev "window.__mkbSearch.getState().status?.phase ?? 'none'")
    case "$out" in *'"value":"ready"'*) return 0;; esac
    ev "window.__mkbSearch.getState().openPanel();'poke'" >/dev/null
    sleep 0.4
  done
  return 1
}

start_app
wait_ready && { pass=$((pass+1)); echo "PASS index ready"; } || { fail=$((fail+1)); echo "FAIL index ready"; }
ev "window.__mkbSearch.getState().closePanel();'x'" >/dev/null

echo "== search and open"
ev "(()=>{window.__mkbSearch.getState().openPanel();window.__mkbSearch.getState().setText('知识库');return 'typed'})()" >/dev/null
sleep 1.2
check "search returns hits" "window.__mkbSearch.getState().hits.length" '"value":[1-9]'
check "search hit opens with Enter" "(()=>{const input=document.querySelector('.search-input');input?.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));return 'sent'})()" '"value":"sent"'
sleep 1
check "document opened" "!!window.__mkbDocs.getState().activePath" '"value":true'

echo "== backlinks"
ev "window.__mkbDocs.getState().open('$VID','raws/共享笔记.md').then(()=>'ok')" >/dev/null
sleep 1.2
check "backlinks list 富文本样例" "document.querySelector('.backlinks')?.innerText.includes('raws/富文本样例.md')" '"value":true'

echo "== create, rename, archive through plan cards"
ev "(()=>{document.querySelector('.vault-sidebar-new [aria-label=新建文件]')?.click();return 'clicked'})()" >/dev/null
sleep 0.4
ev "(()=>{const input=document.querySelector('.tree-inline-input');if(!input)return 'missing';const setter=Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set;setter.call(input,'验收新建');input.dispatchEvent(new Event('input',{bubbles:true}));input.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));return 'typed'})()" >/dev/null
sleep 0.8
check "create plan card" "!!document.querySelector('.plan-card')" '"value":true'
ev "(()=>{document.querySelector('.plan-actions .btn-accent')?.click();return 'x'})()" >/dev/null
sleep 1.4
check "created file exists" "window.__mkbApi.doc.read('$VID','验收新建.md').then(r=>!!r.doc).catch(()=>false)" '"value":true'

ev "(()=>{const row=[...document.querySelectorAll('[role=treeitem]')].find(e=>e.title==='验收新建.md');row?.querySelector('[aria-label=重命名]')?.click();return !!row})()" >/dev/null
sleep 0.4
ev "(()=>{const input=document.querySelector('.tree-inline-input');if(!input)return 'missing';const setter=Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set;setter.call(input,'验收改名.md');input.dispatchEvent(new Event('input',{bubbles:true}));input.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));return 'typed'})()" >/dev/null
sleep 0.8
ev "(()=>{document.querySelector('.plan-actions .btn-accent')?.click();return 'x'})()" >/dev/null
sleep 1.4
check "renamed file exists" "window.__mkbApi.doc.read('$VID','验收改名.md').then(r=>!!r.doc).catch(()=>false)" '"value":true'

ev "(()=>{const row=[...document.querySelectorAll('[role=treeitem]')].find(e=>e.title==='验收改名.md');row?.querySelector('[aria-label=归档]')?.click();return !!row})()" >/dev/null
sleep 0.8
check "archive plan card" "!!document.querySelector('.plan-card')" '"value":true'
ev "(()=>{document.querySelector('.plan-actions .btn-accent')?.click();return 'x'})()" >/dev/null
sleep 1.5
check "archived file moved" "window.__mkbApi.doc.read('$VID','_archives/验收改名.md').then(r=>!!r.doc).catch(()=>false)" '"value":true'

echo "== audit and restore"
ev "window.__mkbApp.getState().toggleAudit();'on'" >/dev/null
sleep 1
check "audit lists the archive" "document.querySelector('.audit-panel')?.innerText.includes('_archives/验收改名.md')" '"value":true'
ev "(()=>{document.querySelector('.audit-restore')?.click();return 'x'})()" >/dev/null
sleep 1
check "restore preview opens" "!!document.querySelector('.plan-card')" '"value":true'
ev "(()=>{document.querySelector('.plan-actions .btn-accent')?.click();return 'x'})()" >/dev/null
sleep 1.6
check "restored file back" "window.__mkbApi.doc.read('$VID','验收改名.md').then(r=>!!r.doc).catch(()=>false)" '"value":true'
AUDIT_LINES=$(wc -l < "$APPD/Mile Knowledge base/audit.jsonl" 2>/dev/null || echo 0)
COUNT_BEFORE=$(ev "window.__mkbSearch.getState().status?.documentCount ?? 0")
echo "audit lines: $AUDIT_LINES, documents: $COUNT_BEFORE"

echo "== restart: index and audit survive"
taskkill //F //IM "Mile Knowledge Base.exe" >/dev/null 2>&1
sleep 2
start_app
wait_ready && { pass=$((pass+1)); echo "PASS index ready after restart"; } || { fail=$((fail+1)); echo "FAIL index ready after restart"; }
check "document count kept" "window.__mkbSearch.getState().status?.documentCount ?? 0" "$(echo "$COUNT_BEFORE" | sed 's/.*"value":\([0-9]*\).*/\1/')"
AUDIT_AFTER=$(wc -l < "$APPD/Mile Knowledge base/audit.jsonl" 2>/dev/null || echo 0)
if [ "$AUDIT_AFTER" -ge "$AUDIT_LINES" ] && [ "$AUDIT_LINES" -gt 0 ]; then pass=$((pass+1)); echo "PASS audit survived restart ($AUDIT_LINES -> $AUDIT_AFTER)"; else fail=$((fail+1)); echo "FAIL audit survive ($AUDIT_LINES -> $AUDIT_AFTER)"; fi

echo "RESULT pass=$pass fail=$fail"
taskkill //F //IM "Mile Knowledge Base.exe" >/dev/null 2>&1
rm -rf "$WORK"
[ "$fail" -eq 0 ]
