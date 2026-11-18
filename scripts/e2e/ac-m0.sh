#!/usr/bin/env bash
# AC-M0-1: one end-to-end run through the first business loop, driven the way a
# user drives it (tree clicks, toolbar clicks), asserting at every step.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "E:/WebApp/Mile-Project/.ac-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null)
VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge Base" <<'PY'
import hashlib,json,os,sys
v,out=sys.argv[1],sys.argv[2]; c=os.path.abspath(v).replace('/',chr(92)).lower(); vid=hashlib.sha256(c.encode()).hexdigest()[:16]
doc={"version":1,"theme":"light","closeBehavior":"quit","layout":{"sidebarWidth":232,"docWidth":560,"docCollapsed":True,"sidebarCollapsed":False,"agnesControlsCollapsed":False,"agnesHistoryOpen":True,"agnesComposerHeight":96},"recentVaults":[{"id":vid,"root":v,"displayName":"ac-vault","lastOpenedAt":"2026-11-18T09:00:00Z"}],"lastVaultId":vid}
open(os.path.join(out,"settings.json"),"w",encoding="utf-8").write(json.dumps(doc,ensure_ascii=False,indent=2)+"\n"); print(vid)
PY
)
DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1
(APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION=tcp:127.0.0.1:18151 "$APP" >"$WORK/app.log" 2>&1 &)
sleep 8
export MYGO_AUTOMATION=tcp:127.0.0.1:18151

pass=0; fail=0
check() { # label script pattern
  python -I - "$WORK/req.json" "$2" <<'PY'
import json,sys
open(sys.argv[1],"w",encoding="utf-8").write(json.dumps({"id":1,"method":"eval","params":{"window":1,"script":sys.argv[2]}}))
PY
  local out; out=$(timeout 30 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"')
  if echo "$out" | grep -q "$3"; then pass=$((pass+1)); echo "PASS $1"; else fail=$((fail+1)); echo "FAIL $1 :: $out"; fi
}

check "workbench with vault" "!!document.querySelector('.vault-sidebar')" '"value":true'
check "tree has area sections" "document.querySelectorAll('.tree-area').length >= 8" '"value":true'
check "expand skills" "(()=>{const r=[...document.querySelectorAll('[role=treeitem]')].find(e=>e.title==='skills');r?.click();return !!r})()" '"value":true'
sleep 0.6
check "skills children visible" "!!document.querySelector('[title=\"skills/react\"]')" '"value":true'
check "open a document by clicking it" "(()=>{const r=[...document.querySelectorAll('[role=treeitem]')].find(e=>e.title==='raws');r?.click();return true})()" '"value":true'
sleep 0.6
check "open 首页.md via tree" "(()=>{const r=[...document.querySelectorAll('[role=treeitem]')].find(e=>e.title==='raws/首页.md');r?.click();return !!r})()" '"value":true'
sleep 0.8
check "reading mode rendered" "!!document.querySelector('.prose h1')" '"value":true'
check "switch to source mode" "(()=>{const b=[...document.querySelectorAll('.editor-toolbar .segmented-item')].find(x=>x.innerText==='源码');b?.click();return !!b})()" '"value":true'
sleep 0.4
check "source editor visible" "!!document.querySelector('.cm-content')" '"value":true'
check "edit in store" "(()=>{const d=window.__mkbDocs.getState();d.setText('raws/首页.md', d.docs['raws/首页.md'].text+'AC 追加。\\n');return true})()" '"value":true'
sleep 0.5
check "dirty dot appears" "!!document.querySelector('.tab-dirty')" '"value":true'
check "save clears dirty" "window.__mkbDocs.getState().save('$VID','raws/首页.md').then(()=>!document.querySelector('.tab-dirty'))" '"value":true'
grep -q 'AC 追加。' "$VAULT/raws/首页.md" && { pass=$((pass+1)); echo "PASS file on disk updated"; } || { fail=$((fail+1)); echo "FAIL file on disk updated"; }
printf '\n外部改。\n' >> "$VAULT/raws/首页.md"
sleep 2.5
check "external change adopted" "window.__mkbDocs.getState().docs['raws/首页.md'].text.includes('外部改。')" '"value":true'
check "csp present in built page" "!!document.querySelector('meta[http-equiv=\"Content-Security-Policy\"]')" '"value":true'

echo "RESULT pass=$pass fail=$fail"
taskkill //F //IM "Mile Knowledge Base.exe" >/dev/null 2>&1
rm -rf "$WORK"
# The gate reads the exit code: any failed assertion fails the run.
[ "$fail" -eq 0 ]
