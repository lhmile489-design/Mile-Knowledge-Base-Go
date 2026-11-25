#!/usr/bin/env bash
# G-PERF for M9 (T07; 技术设计 20.7): the detail drawer and the dialogs.
#   drawerOpenP95Ms <= 150  click on a card -> the rendered SKILL.md is in the
#                           drawer. Every sample is a different skill, so the
#                           render cache never hits. SKILL.md is ~2 KiB.
#   modalOpenMs     <= 100  click on 安装 -> the first frame with the dialog.
#   scroll with the drawer open: same measurement as perf-m8 (p95 gap over the
#   idle refresh interval <= 1.5, slow-frame ratio <= 5%), 200 glass cards.
# A control scroll without the drawer runs first in the same process and is
# printed next to it, so a regression can be told from machine noise.
# perf-m8.sh itself is not touched here (Q-M9-3 needs a separate yes).
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.perf-m9-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base" "$HOME_DIR/.claude/skills" "$HOME_DIR/.local/bin"
printf '@echo off' > "$HOME_DIR/.local/bin/claude.cmd"
printf '{"uninstall":[]}' > "$WORK/registry.json"

python -I - "$VAULT" <<'PY'
import os, sys
vault = sys.argv[1]
BODY = """
## 何时使用

当用户要求整理笔记、检查链接或批量改名时使用本技能。先读取目录结构，再决定处理顺序。

## 步骤

1. 列出候选文件，按修改时间排序。
2. 对每个文件检查标题、链接与引用。
3. 把改动汇总成一张表，等用户确认后再写入。

```bash
find . -name "*.md" -newer index.json | sort
```

| 场景 | 做法 |
| --- | --- |
| 单个文件 | 直接修改并给出差异 |
| 批量改名 | 先预览，再确认 |

> 提示：任何写入都要先备份，失败时可以恢复。
"""
for i in range(200):
    d = os.path.join(vault, "skills", "cat-%02d" % (i % 10), "skill-%03d" % i)
    os.makedirs(d, exist_ok=True)
    open(os.path.join(d, "SKILL.md"), "w", encoding="utf-8", newline="\n").write(
        "---\nname: skill-%03d\ndescription: 用于滚动性能测试的技能 %03d，带一段足够长的中文描述。\n---\n\n# skill-%03d\n%s" % (i, i, i, BODY * 2)
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
       "recentVaults": [{"id": vid, "root": vault, "displayName": "perf-m9", "lastOpenedAt": "2026-11-25T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(out, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

PORT=18561
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

# ---- drawer: a different skill every time, so the render cache never hits ----
DRAWER = """
(async (name) => {
  const card = document.querySelector('[data-testid="kb-card-' + name + '"]');
  if (!card) return -1;
  const t0 = performance.now();
  card.click();
  await new Promise((resolve) => {
    const tick = () => {
      const prose = document.querySelector('[data-testid="skill-drawer"] .prose');
      if (prose && prose.textContent.length > 20) resolve();
      else requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });
  await new Promise((resolve) => requestAnimationFrame(resolve));
  const elapsed = performance.now() - t0;
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  await new Promise((resolve) => {
    const tick = () => (document.querySelector('[data-testid="skill-drawer"]') ? requestAnimationFrame(tick) : resolve());
    requestAnimationFrame(tick);
  });
  return elapsed;
})(%s)"""
drawer_ms = []
for i in range(40):
    name = "skill-%03d" % (i * 5)
    value = js(DRAWER % json.dumps(name))
    if value is None or value < 0:
        raise SystemExit("drawer sample failed for " + name)
    drawer_ms.append(value)
    time.sleep(0.25)
print("drawer_ms:", " ".join("%.1f" % v for v in drawer_ms))

# ---- dialog: click 安装, first frame that contains the dialog ----
MODAL = """
(async () => {
  const button = document.querySelector('[data-testid="kb-install-top"]');
  if (!button) return -1;
  const t0 = performance.now();
  button.click();
  await new Promise((resolve) => {
    const tick = () => (document.querySelector('[data-testid="skills-modal"]') ? resolve() : requestAnimationFrame(tick));
    requestAnimationFrame(tick);
  });
  await new Promise((resolve) => requestAnimationFrame(resolve));
  const elapsed = performance.now() - t0;
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  await new Promise((resolve) => {
    const tick = () => (document.querySelector('[data-testid="skills-modal"]') ? requestAnimationFrame(tick) : resolve());
    requestAnimationFrame(tick);
  });
  return elapsed;
})()"""
modal_ms = []
for _ in range(20):
    value = js(MODAL)
    if value is None or value < 0:
        raise SystemExit("dialog sample failed")
    modal_ms.append(value)
    time.sleep(0.25)
print("modal_ms:", " ".join("%.1f" % v for v in modal_ms))

# ---- scroll: the perf-m8 measurement, with and without the drawer ----
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
  window.__p9 = { gaps: [], stop: false };
  const start = performance.now();
  let last = start;
  const tick = (now) => {
    if (window.__p9.stop) return;
    window.__p9.gaps.push(now - last);
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
  window.__p9.stop = true;
  const gaps = window.__p9.gaps.slice(1);
  const ordered = gaps.slice().sort((a, b) => a - b);
  const p95 = ordered[Math.max(0, Math.ceil(0.95 * ordered.length) - 1)] || 0;
  const slow = gaps.filter((gap) => gap > 2 * %s).length / Math.max(1, gaps.length);
  return JSON.stringify({ frames: gaps.length, p50: ordered[Math.floor(ordered.length / 2)] || 0, p95, max: ordered[ordered.length - 1] || 0, slow, range: document.querySelector('.kb-content').scrollHeight - document.querySelector('.kb-content').clientHeight });
})()""" % json.dumps(refresh)

def scroll_rounds(label):
    rounds = []
    for index in range(3):
        js(SCROLL)
        time.sleep(6.4)
        rounds.append(json.loads(js(STOP)))
        print("%s round %d:" % (label, index + 1), rounds[-1])
    if min(r["range"] for r in rounds) <= 0:
        raise SystemExit("FAIL: .kb-content did not scroll (range 0)")
    return rounds

control = scroll_rounds("scroll without drawer")

js("document.querySelector('[data-testid=\"kb-card-skill-007\"]').click(); true")
wait("document.querySelector('[data-testid=\"skill-drawer\"]')?.getAttribute('data-state') === 'ready'", 60)
wait("(document.querySelector('[data-testid=\"skill-drawer\"] .prose')?.textContent.length || 0) > 20", 60)
print("drawer open:", js("!!document.querySelector('[data-testid=\"skill-drawer\"]')"))
with_drawer = scroll_rounds("scroll with drawer")

# Warm control: the first control above runs on a cold page (the 1.6.1 first-
# round slow frames), so it cannot be compared with the drawer rounds. Close the
# drawer and measure again on the now warm page.
js("document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); true")
wait("document.querySelector('[data-testid=\"skill-drawer\"]') === null", 30)
control_warm = scroll_rounds("scroll without drawer (warm)")

def frames(rounds):
    return max(r["p95"] / refresh for r in rounds)

def slow(rounds):
    return max(r["slow"] for r in rounds)

print("control (cold, first)  : scrollP95Frames=%.2f slowFrameRatio=%.4f" % (frames(control), slow(control)))
print("control (warm, after)   : scrollP95Frames=%.2f slowFrameRatio=%.4f" % (frames(control_warm), slow(control_warm)))
print("drawer open             : scrollP95Frames=%.2f slowFrameRatio=%.4f" % (frames(with_drawer), slow(with_drawer)))

metrics = {
    "drawerOpenP95Ms": round(p95(drawer_ms), 1),
    "modalOpenMs": round(p95(modal_ms), 1),
    "scrollP95Frames": round(frames(with_drawer), 2),
    "slowFrameRatio": round(slow(with_drawer), 4),
}
json.dump(metrics, open(out_path, "w", encoding="utf-8"))
print("metrics:", json.dumps(metrics))
PY
PY_STATUS=$?
if [ "$PY_STATUS" -ne 0 ]; then
  echo "perf-m9: measurement failed ($PY_STATUS)"
  exit "$PY_STATUS"
fi

# The drawer and dialog keys are checked as kind=m9; the scroll keys keep the M8
# limits (the other m8 keys are absent from this file and are skipped).
python -I "$ROOT/scripts/perf-gate.py" --kind m9 --input "$(cygpath -w "$WORK/metrics.json")"
S1=$?
python -I "$ROOT/scripts/perf-gate.py" --kind m8 --input "$(cygpath -w "$WORK/metrics.json")"
S2=$?
stop_pid=$APP_PID
powershell -NoProfile -Command "Stop-Process -Id $stop_pid -Force" >/dev/null 2>&1
APP_PID=""
[ "$S1" -eq 0 ] && [ "$S2" -eq 0 ]
