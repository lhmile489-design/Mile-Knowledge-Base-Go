#!/usr/bin/env bash
# G-PERF for M1: cold start, index build, search p50/p95 and 1000 external
# writes on the typical and the big sample vaults.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.perf-m1-XXXXXX")
DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1
RESULT="$WORK/result.txt"; : > "$RESULT"

gen() { (cd "$ROOT" && go run ./cmd/genvault -kind "$1" -out "$(cygpath -w "$2")" >/dev/null); }
seed() {
  mkdir -p "$2/Mile Knowledge base"
  python -I - "$1" "$2/Mile Knowledge base" <<'PY'
import hashlib,json,os,sys
v,out=sys.argv[1],sys.argv[2]; c=os.path.abspath(v).replace('/',chr(92)).lower(); vid=hashlib.sha256(c.encode()).hexdigest()[:16]
doc={"version":1,"theme":"light","closeBehavior":"quit","layout":{"sidebarWidth":232,"docWidth":620,"docCollapsed":True,"sidebarCollapsed":False,"agnesControlsCollapsed":False,"agnesHistoryOpen":True,"agnesComposerHeight":96},"recentVaults":[{"id":vid,"root":v,"displayName":os.path.basename(v),"lastOpenedAt":"2026-11-18T09:00:00Z"}],"lastVaultId":vid}
open(os.path.join(out,"settings.json"),"w",encoding="utf-8").write(json.dumps(doc,ensure_ascii=False,indent=2)+"\n"); print(vid)
PY
}
ev() {
  python -I - "$WORK/req.json" "$2" <<'PY'
import json,sys
open(sys.argv[1],"w",encoding="utf-8").write(json.dumps({"id":1,"method":"eval","params":{"window":1,"script":sys.argv[2]}}))
PY
  MYGO_AUTOMATION="tcp:127.0.0.1:$1" timeout 60 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}
poll() { # port script pattern timeout_ms
  local port="$1" script="$2" pattern="$3" limit="$4" t0 now
  t0=$(date +%s%3N)
  while :; do
    now=$(date +%s%3N); [ $((now - t0)) -gt "$limit" ] && { echo -1; return; }
    if ev "$port" "$script" | grep -qE "$pattern"; then echo $(( $(date +%s%3N) - t0 )); return; fi
    sleep 0.2
  done
}

run_one() { # kind port
  local kind="$1" port="$2" vault="$WORK/vault-$1" appd="$WORK/app-$1"
  mkdir -p "$vault" "$appd"; gen "$kind" "$vault"; seed "$vault" "$appd" >/dev/null
  local t0; t0=$(date +%s%3N)
  (APPDATA="$(cygpath -w "$appd")" MYGO_AUTOMATION="tcp:127.0.0.1:$port" "$APP" >"$appd/app.log" 2>&1 &)
  local cold; cold=$(poll "$port" "document.querySelectorAll('.tree-row').length" '"value":[1-9]' 60000)
  echo "cold_start_${kind}_ms=$cold" | tee -a "$RESULT"
  ev "$port" "window.__mkbSearch.getState().openPanel();'poke'" >/dev/null
  local ready; ready=$(poll "$port" "window.__mkbSearch.getState().status?.phase ?? 'none'" '"value":"ready"' 120000)
  echo "index_ready_${kind}_ms=$ready" | tee -a "$RESULT"

  local vid; vid=$(python -I - "$vault" <<'PY'
import hashlib,os,sys
c=os.path.abspath(sys.argv[1]).replace('/',chr(92)).lower(); print(hashlib.sha256(c.encode()).hexdigest()[:16])
PY
)
  ev "$port" "(()=>{window.__q='$vid';window.__times=[];window.__run=async(text)=>{const t0=performance.now();await window.__mkbApi.index.search({vaultId:window.__q,text,limit:50});window.__times.push(performance.now()-t0)};return 'ok'})()" >/dev/null
  local queries=(知识 管理 文档 状态 技能 项目 笔记 归档 检索 链接)
  for qi in $(seq 1 5); do
    for q in "${queries[@]}"; do
      ev "$port" "window.__run('$q').then(()=>'done')" >/dev/null
    done
  done
  local stats; stats=$(ev "$port" "(()=>{const t=[...window.__times].sort((a,b)=>a-b);const p=(x)=>t[Math.min(t.length-1,Math.floor(t.length*x))];return JSON.stringify({n:t.length,p50:Math.round(p(0.5)*10)/10,p95:Math.round(p(0.95)*10)/10,max:Math.round(t[t.length-1]*10)/10})})()")
  echo "search_${kind}=$stats" | tee -a "$RESULT"

  local flat="$vault/bulk/flat"
  if [ -d "$flat" ]; then
    ev "$port" "(()=>{window.__lt=[];const o=new PerformanceObserver(l=>{for(const e of l.getEntries())window.__lt.push(e.duration)});o.observe({entryTypes:['longtask']});return 'armed'})()" >/dev/null
    for i in $(seq 0 999); do printf '\n外部改动%05d。\n' "$i" >> "$flat/$(printf '文档-%05d.md' "$i")"; done
    sleep 4
    local lt; lt=$(ev "$port" "JSON.stringify({n:window.__lt.length,max:Math.max(0,...window.__lt)})")
    echo "external_1000_${kind}=$lt" | tee -a "$RESULT"
  fi
  taskkill //F //IM "Mile Knowledge Base.exe" >/dev/null 2>&1
  sleep 1
}

run_one typical 18411
run_one big 18412
echo "WORK=$WORK"
cat "$RESULT"
