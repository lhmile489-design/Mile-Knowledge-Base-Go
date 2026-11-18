#!/usr/bin/env bash
# T10 performance probe v2.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "E:/WebApp/Mile-Project/.perf-XXXXXX")
DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1
OUT="$WORK/result.txt"
: > "$OUT"

gen() { (cd "$ROOT" && go run ./cmd/genvault -kind "$1" -out "$(cygpath -w "$2")" >/dev/null); }

seed() {
  mkdir -p "$2/Mile Knowledge Base"
  python -I - "$1" "$2/Mile Knowledge Base" <<'PY'
import hashlib, json, os, sys
v,out=sys.argv[1],sys.argv[2]
c=os.path.abspath(v).replace('/',chr(92)).lower(); vid=hashlib.sha256(c.encode()).hexdigest()[:16]
doc={"version":1,"theme":"light","closeBehavior":"quit","layout":{"sidebarWidth":232,"docWidth":560,"docCollapsed":True,"sidebarCollapsed":False,"agnesControlsCollapsed":False,"agnesHistoryOpen":True,"agnesComposerHeight":96},"recentVaults":[{"id":vid,"root":v,"displayName":os.path.basename(v),"lastOpenedAt":"2026-11-18T09:00:00Z"}],"lastVaultId":vid}
open(os.path.join(out,"settings.json"),"w",encoding="utf-8").write(json.dumps(doc,ensure_ascii=False,indent=2)+"\n"); print(vid)
PY
}

ev() { # port script
  python -I - "$WORK/req.json" "$2" <<'PY'
import json,sys
open(sys.argv[1],"w",encoding="utf-8").write(json.dumps({"id":1,"method":"eval","params":{"window":1,"script":sys.argv[2]}}))
PY
  MYGO_AUTOMATION="tcp:127.0.0.1:$1" timeout 30 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}

poll_until() { # port script pattern timeout_ms  -> elapsed ms or -1
  local port="$1" script="$2" pattern="$3" limit="$4" t0 now
  t0=$(date +%s%3N)
  while :; do
    now=$(date +%s%3N)
    if [ $((now - t0)) -gt "$limit" ]; then echo -1; return; fi
    if ev "$port" "$script" | grep -qE "$pattern"; then echo $(( $(date +%s%3N) - t0 )); return; fi
    sleep 0.15
  done
}

run_vault() { # kind port
  local kind="$1" port="$2" tmp="$WORK/app-$1" vault="$WORK/vault-$1"
  mkdir -p "$tmp" "$vault"; gen "$kind" "$vault"; seed "$vault" "$tmp" >/dev/null
  local t0; t0=$(date +%s%3N)
  (APPDATA="$(cygpath -w "$tmp")" MYGO_AUTOMATION="tcp:127.0.0.1:$port" "$APP" >"$tmp/app.log" 2>&1 &)
  local ms; ms=$(poll_until "$port" "document.querySelectorAll('.tree-row').length" '"value":[1-9]' 60000)
  echo "cold_start_${kind}_ms=$ms total_ms=$(( $(date +%s%3N) - t0 ))" | tee -a "$OUT"
  echo "$vault" > "$WORK/vaultpath-$kind"
}

run_vault typical 18121
run_vault big 18122

# Typical: open a document, time until the reader has content.
VP=$(cat "$WORK/vaultpath-typical")
VT=$(python -I - "$VP" <<'PY'
import hashlib,os,sys
c=os.path.abspath(sys.argv[1]).replace('/',chr(92)).lower();print(hashlib.sha256(c.encode()).hexdigest()[:16])
PY
)
ev 18121 "window.__mkbDocs.getState().open('$VT','raws/富文本样例.md').then(()=>'opened')" >/dev/null
MS=$(poll_until 18121 "document.querySelectorAll('.prose h1').length" '"value":[1-9]' 20000)
echo "typical_reader_ms=$MS" | tee -a "$OUT"
ev 18121 "JSON.stringify({proseChars:document.querySelector('.prose')?.innerText.length,imgW:document.querySelector('.prose img')?.naturalWidth})" | tee -a "$OUT"

# Big: expand bulk/flat, then scroll and watch long tasks.
VB=$(cat "$WORK/vaultpath-big")
ev 18122 "(()=>{[...document.querySelectorAll('[role=treeitem]')].find(e=>e.title==='bulk')?.click();return 'ok'})()" >/dev/null
sleep 0.4
ev 18122 "(()=>{[...document.querySelectorAll('[role=treeitem]')].find(e=>e.title==='bulk/flat')?.click();return 'ok'})()" >/dev/null
MS=$(poll_until 18122 "window.__mkbTree.getState().nodes['bulk/flat']?.length ?? 0" '"value":[4-9][0-9][0-9][0-9]' 30000)
echo "big_flat_load_ms=$MS" | tee -a "$OUT"
ev 18122 "JSON.stringify({flat:window.__mkbTree.getState().nodes['bulk/flat'].length,doms:document.querySelectorAll('[role=treeitem]').length})" | tee -a "$OUT"
ev 18122 "(()=>{window.__lt=[];const o=new PerformanceObserver(l=>{for(const e of l.getEntries())window.__lt.push(e.duration)});o.observe({entryTypes:['longtask']});const el=document.querySelector('.tree-scroll');let i=0;const step=()=>{el.scrollTop+=300;i++;if(i<80)requestAnimationFrame(step)};requestAnimationFrame(step);return new Promise(r=>setTimeout(()=>{o.disconnect();const a=window.__lt;r(JSON.stringify({n:a.length,max:Math.max(0,...a),scrollTop:el.scrollTop}))},3000))})()" | tee -a "$OUT"

# Big: 1000 external changes while observing the main thread.
FLAT="$VB/bulk/flat"
ev 18122 "(()=>{window.__lt2=[];const o=new PerformanceObserver(l=>{for(const e of l.getEntries())window.__lt2.push(e.duration)});o.observe({entryTypes:['longtask']});return 'armed'})()" >/dev/null
T0=$(date +%s%3N)
for i in $(seq 0 999); do printf '\n外部改动%05d。\n' "$i" >> "$FLAT/$(printf '文档-%05d.md' "$i")"; done
TOUCH_MS=$(( $(date +%s%3N) - T0 ))
sleep 4
ev 18122 "JSON.stringify({n:window.__lt2.length,max:Math.max(0,...window.__lt2)})" | tee -a "$OUT"
echo "big_touch_1000_ms=$TOUCH_MS" | tee -a "$OUT"

taskkill //F //IM "Mile Knowledge Base.exe" >/dev/null 2>&1
echo "WORK=$WORK"
cat "$OUT"
