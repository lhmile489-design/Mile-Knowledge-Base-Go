#!/usr/bin/env bash
# G-PERF for M8 (T12): entering the skills page, scrolling 200 glass cards,
# and switching back to the workbench (AC-PAGE-7). Thresholds come from the
# T00 spike and are expressed in display-refresh intervals:
#   scrollP95Frames = p95 / R <= 1.5, slowFrameRatio (> 2R) <= 5%,
#   enterP95Ms <= 300, switchBackMs <= 100.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.perf-m8-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base" "$HOME_DIR/.claude/skills" "$HOME_DIR/.local/bin"
printf '@echo off' > "$HOME_DIR/.local/bin/claude.cmd"
printf '{"uninstall":[]}' > "$WORK/registry.json"

python -I - "$VAULT" <<'PY'
import os, sys
vault = sys.argv[1]
for i in range(200):
    d = os.path.join(vault, "skills", "cat-%02d" % (i % 10), "skill-%03d" % i)
    os.makedirs(d, exist_ok=True)
    open(os.path.join(d, "SKILL.md"), "w", encoding="utf-8").write(
        "---\nname: skill-%03d\ndescription: 用于滚动性能测试的技能 %03d，带一段足够长的中文描述。\n---\n\n# s%03d\n" % (i, i, i)
    )
print("fixtures ok")
PY

VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge Base" <<'PY'
import hashlib, json, os, sys
vault, out = sys.argv[1], sys.argv[2]
c = os.path.abspath(vault).replace("/", chr(92)).lower()
vid = hashlib.sha256(c.encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": False,
                  "sidebarCollapsed": False, "agnesControlsCollapsed": False,
                  "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": vault, "displayName": "perf-m8", "lastOpenedAt": "2026-11-24T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(out, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

PORT=18551
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  sleep 2
  rm -rf "$WORK"
}
trap cleanup EXIT
if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running"; exit 2
fi

APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" \
  SKILLS_FAKE_HOME="$(cygpath -w "$HOME_DIR")" \
  SKILLS_FAKE_PATH="$(cygpath -w "$HOME_DIR/.local/bin")" \
  SKILLS_FAKE_REGISTRY="$(cygpath -w "$WORK/registry.json")" \
  "$APP" >"$WORK/app.log" 2>&1 &
sleep 7
APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -d '\r')

python -I - "$PORT" "$(cygpath -w "$WORK/metrics.json")" <<'PY'
import json, socket, sys, time

port, out_path = int(sys.argv[1]), sys.argv[2]

def call(method, **params):
    with socket.create_connection(("127.0.0.1", port), timeout=90) as s:
        s.sendall((json.dumps({"id": 1, "method": method, "params": params}) + "\n").encode())
        line = s.makefile("rb").readline()
    data = json.loads(line)
    if "error" in data:
        raise RuntimeError(data["error"])
    return data.get("result", {})

def js(script):
    return call("eval", window=1, script=script).get("value")

def wait(script, tries=240):
    for _ in range(tries):
        try:
            if js(script):
                return
        except Exception:
            pass
        time.sleep(0.3)
    raise SystemExit("wait timeout: " + script[:80])

def p95(values):
    ordered = sorted(values)
    return ordered[max(0, int(round(0.95 * len(ordered))) - 1)]

wait("!!window.__mkbApp && !!window.__mkbApp.getState().settings")
wait("document.querySelector('[data-testid=\"overview-grid\"]') !== null")
js("window.__mkbApp.getState().showPage('skills'); true")
wait("document.querySelector('[data-testid=\"skills-page\"]') !== null", 60)
wait("document.querySelectorAll('[data-testid^=\"kb-card-\"]').length >= 200", 120)
print("cards:", js("document.querySelectorAll('[data-testid^=\"kb-card-\"]').length"))
print("glass:", js("document.querySelector('[data-testid=\"skills-page\"]').dataset.glass"))

ENTER = """
(async () => {
  const app = window.__mkbApp.getState();
  app.showPage('work');
  await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  await new Promise((resolve) => setTimeout(resolve, 250));
  const t0 = performance.now();
  window.__mkbApp.getState().showPage('skills');
  await new Promise((resolve) => {
    const tick = () => {
      if (document.querySelector('[data-testid^="kb-card-"]')) resolve();
      else requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });
  return performance.now() - t0;
})()"""

enter_ms = [js(ENTER) for _ in range(5)]
print("enter_ms:", " ".join("%.1f" % v for v in enter_ms))

REFRESH = """
(async () => {
  const gaps = [];
  let last = 0;
  await new Promise((resolve) => {
    const tick = (now) => {
      if (last > 0) gaps.push(now - last);
      last = now;
      if (gaps.length >= 40) resolve();
      else requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });
  const ordered = gaps.slice().sort((a, b) => a - b);
  return ordered[Math.floor(ordered.length / 2)];
})()"""
refresh = js(REFRESH)
print("refresh_ms:", round(refresh, 2))

SCROLL = """
(() => {
  const el = document.querySelector('.kb-content');
  el.scrollTop = 0;
  window.__p8 = { gaps: [], stop: false };
  const start = performance.now();
  let last = start;
  const tick = (now) => {
    if (window.__p8.stop) return;
    window.__p8.gaps.push(now - last);
    last = now;
    const max = el.scrollHeight - el.clientHeight;
    el.scrollTop = Math.min(max, ((now - start) / 6000) * max);
    if (now - start < 6000) requestAnimationFrame(tick);
  };
  requestAnimationFrame(tick);
  return true;
})()"""
STOP = """
(() => {
  window.__p8.stop = true;
  const gaps = window.__p8.gaps.slice(1);
  const ordered = gaps.slice().sort((a, b) => a - b);
  const p95 = ordered[Math.max(0, Math.ceil(0.95 * ordered.length) - 1)] || 0;
  const slow = gaps.filter((gap) => gap > 2 * %s).length / Math.max(1, gaps.length);
  return JSON.stringify({ frames: gaps.length, p50: ordered[Math.floor(ordered.length / 2)] || 0, p95, max: ordered[ordered.length - 1] || 0, slow, range: document.querySelector('.kb-content').scrollHeight - document.querySelector('.kb-content').clientHeight });
})()""" % json.dumps(refresh)

rounds = []
for round_index in range(3):
    js(SCROLL)
    time.sleep(6.4)
    rounds.append(json.loads(js(STOP)))
    print("scroll round %d:" % (round_index + 1), rounds[-1])
# 1.6.1: a page that cannot scroll used to pass this gate with range 0.
if min(round_["range"] for round_ in rounds) <= 0:
    raise SystemExit("FAIL: .kb-content did not scroll (range 0)")

SWITCH = """
(async () => {
  const t0 = performance.now();
  window.__mkbApp.getState().showPage('work');
  await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  return performance.now() - t0;
})()"""
switch_ms = []
for _ in range(5):
    switch_ms.append(js(SWITCH))
    time.sleep(0.3)
    js("window.__mkbApp.getState().showPage('skills'); true")
    time.sleep(0.4)
print("switch_ms:", " ".join("%.1f" % v for v in switch_ms))

scroll_frames = max(round_["p95"] / refresh for round_ in rounds)
slow_ratio = max(round_["slow"] for round_ in rounds)

metrics = {
    "enterP95Ms": round(p95(enter_ms), 1),
    "scrollP95Frames": round(scroll_frames, 2),
    "slowFrameRatio": round(slow_ratio, 4),
    "switchBackMs": round(p95(switch_ms), 1),
}
json.dump(metrics, open(out_path, "w", encoding="utf-8"))
print("metrics:", json.dumps(metrics))
PY

python -I "$ROOT/scripts/perf-gate.py" --kind m8 --input "$(cygpath -w "$WORK/metrics.json")"
STATUS=$?
stop_pid=$APP_PID
powershell -NoProfile -Command "Stop-Process -Id $stop_pid -Force" >/dev/null 2>&1
APP_PID=""
exit $STATUS
