#!/usr/bin/env bash
# AC-M3.5: the accent colour closed loop on a real window (PRD 16.7).
# Covers AC-ACC-1 (fresh install), AC-ACC-2 (pick blue, persists), AC-ACC-3
# (real-component contrast across presets and themes), AC-ACC-4 (custom hex)
# and AC-ACC-5 (garbage in settings falls back to orange). AC-ACC-6 is the
# visual recapture in T08. Temp APPDATA only; the real app data is untouched.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.ac-m35-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null)
VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge Base" <<'PY'
import hashlib,json,os,sys
v,out=sys.argv[1],sys.argv[2]
c=os.path.abspath(v).replace('/',chr(92)).lower(); vid=hashlib.sha256(c.encode()).hexdigest()[:16]
doc={"version":1,"theme":"light","closeBehavior":"quit","layout":{"sidebarWidth":232,"docWidth":620,"docCollapsed":False,"sidebarCollapsed":False,"agnesControlsCollapsed":False,"agnesHistoryOpen":True,"agnesComposerHeight":96},"recentVaults":[{"id":vid,"root":v,"displayName":"ac-m35","lastOpenedAt":"2026-11-20T09:00:00Z"}],"lastVaultId":vid}
open(os.path.join(out,"settings.json"),"w",encoding="utf-8").write(json.dumps(doc,ensure_ascii=False,indent=2)+"\n"); print(vid)
PY
)

DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1

PORT=18461
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then
    powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1
  fi
  sleep 2
  rm -rf "$WORK"
}
trap cleanup EXIT

if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running; close it first (this script only kills what it starts)"
  exit 2
fi

start_app() { # appdata_dir
  APPDATA="$(cygpath -w "$1")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" "$APP" >"$WORK/app.log" 2>&1 &
  sleep 6
  APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -d '\r')
}
stop_app() {
  if [ -n "${APP_PID:-}" ]; then
    powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1
  fi
  APP_PID=""
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
pass=0; fail=0
check() {
  local out; out=$(ev "$2")
  if echo "$out" | grep -q "$3"; then pass=$((pass+1)); echo "PASS $1"; else fail=$((fail+1)); echo "FAIL $1 :: $out"; fi
}
wait_match() {
  local label=$1 script=$2 pattern=$3 tries=${4:-40} i out
  for i in $(seq 1 "$tries"); do
    out=$(ev "$script")
    if echo "$out" | grep -q "$pattern"; then pass=$((pass+1)); echo "PASS $label"; return 0; fi
    sleep 0.5
  done
  fail=$((fail+1)); echo "FAIL $label :: $out"
}
jsq() { python -I -c "import json,sys;print(json.dumps(sys.argv[1]))" "$1"; }
click() { ev "(() => { const el = document.querySelector($(jsq "$1")); if (!el) throw new Error('missing'); el.click(); return true; })()"; }
set_value() {
  local proto=HTMLInputElement; [ "${3:-input}" = "textarea" ] && proto=HTMLTextAreaElement
  ev "(() => { const el = document.querySelector($(jsq "$1")); if (!el) throw new Error('missing'); const set = Object.getOwnPropertyDescriptor(window.$proto.prototype,'value').set; set.call(el, $(jsq "$2")); el.dispatchEvent(new Event('input',{bubbles:true})); return true; })()"
}
read_accent() {
  python -I -c "import json,sys;print(json.dumps(json.load(open(sys.argv[1],encoding='utf-8')).get('accent'),ensure_ascii=False))" "$APPD/Mile Knowledge Base/settings.json"
}

# ---------------------------------------------------------------- AC-ACC-1
FRESH="$WORK/fresh"; mkdir -p "$FRESH"
start_app "$FRESH"
wait_match "fresh boot" "!!window.__mkbApp && !!window.__mkbApp.getState().settings" '"value":true' 90
check "AC-ACC-1 store default orange" "window.__mkbApp.getState().settings.accent.preset === 'orange'" '"value":true'
check "AC-ACC-1 html attribute" "document.documentElement.dataset.accent === 'orange'" '"value":true'
check "AC-ACC-1 tokens and welcome button" "(() => {
  const t = document.documentElement.dataset.theme;
  const mode = t === 'dark'
    ? { accent: '#fa8234', btn: '#fa8234', bg: 'rgb(250, 130, 52)', fg: 'rgb(17, 18, 20)' }
    : { accent: '#e87226', btn: '#ba5b1e', bg: 'rgb(186, 91, 30)', fg: 'rgb(255, 255, 255)' };
  const root = getComputedStyle(document.documentElement);
  const button = document.querySelector('.btn-accent');
  const s = button ? getComputedStyle(button) : null;
  const ok = root.getPropertyValue('--raw-accent').trim() === mode.accent
    && root.getPropertyValue('--raw-accent-btn').trim() === mode.btn
    && root.getPropertyValue('--focus').trim() === mode.accent
    && !!s && s.backgroundColor === mode.bg && s.color === mode.fg;
  return ok ? 'ok' : JSON.stringify({ theme: t, button: !!s, bg: s && s.backgroundColor, fg: s && s.color });
})()" '"value":"ok"'
check "AC-ACC-1 no fault notice" "window.__mkbApp.getState().notices.every((n) => n.level !== 'fault')" '"value":true'
stop_app

# ---------------------------------------------------------------- AC-ACC-2
start_app "$APPD"
wait_match "seeded boot" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
ev "window.__mkbDocs.getState().open('$VID', 'raws/富文本样例.md'); window.__mkbTree.getState().reveal('raws/富文本样例.md'); true" >/dev/null
wait_match "doc opened" "document.querySelector('.prose a') !== null" '"value":true' 60
wait_match "tree row selected" "document.querySelector('.tree-row[aria-selected=\"true\"]') !== null" '"value":true' 40
check "AC-ACC-2 starts orange" "document.documentElement.dataset.accent === 'orange'" '"value":true'
click '[data-testid="accent-dot"]'
wait_match "popover open" "document.querySelector('[data-testid=\"accent-popover\"]') !== null" '"value":true' 20
# The popover and its six swatches commit together; wait for the radios, not
# just the container, so the click never races a re-render.
wait_match "popover swatches" "document.querySelectorAll('[data-testid=\"accent-popover\"] [role=\"radio\"]').length === 6" '"value":true' 20
click '[data-testid="accent-preset-blue"]'
wait_match "blue applied at once" "document.documentElement.dataset.accent === 'blue' && getComputedStyle(document.documentElement).getPropertyValue('--raw-accent').trim() === '#2563eb'" '"value":true' 20
check "blue stored" "window.__mkbApp.getState().settings.accent.preset === 'blue'" '"value":true'
check "dot label blue" "document.querySelector('[data-testid=\"accent-dot\"]').getAttribute('aria-label') === '主题色：蓝'" '"value":true'
click '[data-testid="open-settings"]'
wait_match "settings open" "document.querySelector('[data-testid=\"settings-accent-preset-blue\"]') !== null" '"value":true' 30
check "AC-ACC-2 settings row shows blue" "document.querySelector('[data-testid=\"settings-accent-preset-blue\"]').getAttribute('aria-checked') === 'true'" '"value":true'
sleep 1
[ "$(read_accent)" = '{"preset": "blue"}' ] && { pass=$((pass+1)); echo "PASS AC-ACC-2 file stores blue"; } || { fail=$((fail+1)); echo "FAIL AC-ACC-2 file stores blue :: $(read_accent)"; }
stop_app
start_app "$APPD"
wait_match "restart with blue" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
check "AC-ACC-2 blue after restart" "document.documentElement.dataset.accent === 'blue'" '"value":true'
check "AC-ACC-2 blue button token after restart" "getComputedStyle(document.documentElement).getPropertyValue('--raw-accent-btn').trim() === '#2563eb'" '"value":true'

# ---------------------------------------------------------------- AC-ACC-3
ev "window.__mkbDocs.getState().open('$VID', 'raws/富文本样例.md'); window.__mkbTree.getState().reveal('raws/富文本样例.md'); true" >/dev/null
wait_match "doc for contrast" "document.querySelector('.prose a') !== null" '"value":true' 60
wait_match "tree row for contrast" "document.querySelector('.tree-row[aria-selected=\"true\"]') !== null" '"value":true' 40
python -I - "$WORK/req.json" "$ROOT/scripts/e2e/accent-contrast.js" <<'PY'
import json,sys
script = open(sys.argv[2], encoding="utf-8").read()
open(sys.argv[1],"w",encoding="utf-8").write(json.dumps({"id":1,"method":"eval","params":{"window":1,"script":script}}))
PY
if timeout 240 "$DRIVE" -f "$WORK/req.json" > "$WORK/contrast.json" 2>&1 && python -I - "$WORK/contrast.json" <<'PY'
import json, re, sys
resp = json.loads(open(sys.argv[1], encoding="utf-8").read())
if "result" not in resp or "value" not in resp["result"]:
    print("no eval result:", resp)
    sys.exit(2)
samples = json.loads(resp["result"]["value"])
LIMITS = {"text": 4.5, "button": 4.5, "fill": 3.0}
ELEMENTS = ["segmented", "tree-text", "link", "send", "primary-button", "switch-knob"]
PRESETS = ["orange", "indigo", "blue", "green", "teal", "violet"]
def rgb(value):
    nums = [float(x) for x in re.findall(r"[0-9.]+", value)]
    if len(nums) < 3:
        raise ValueError(value)
    return nums[:3]
def lum(c):
    def f(v):
        v /= 255
        return v / 12.92 if v <= 0.04045 else ((v + 0.055) / 1.055) ** 2.4
    r, g, b = c
    return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b)
worst = {}
fails = []
for s in samples:
    a, b = lum(rgb(s["fg"])), lum(rgb(s["bg"]))
    hi, lo = max(a, b), min(a, b)
    ratio = (hi + 0.05) / (lo + 0.05)
    kind = s["kind"]
    worst[kind] = min(worst.get(kind, 99.0), ratio)
    if ratio < LIMITS[kind]:
        fails.append(f"{s['preset']}/{s['theme']}/{s['element']} {ratio:.2f} < {LIMITS[kind]}")
need = {(p, t, e) for p in PRESETS for t in ("light", "dark") for e in ELEMENTS}
have = {(s["preset"], s["theme"], s["element"]) for s in samples}
missing = sorted(need - have)
print("contrast samples=" + str(len(samples)) + " worst=" + ", ".join(f"{k}:{v:.2f}" for k, v in sorted(worst.items())))
for item in fails:
    print("FAIL", item)
for item in missing:
    print("MISSING", "/".join(item))
sys.exit(1 if fails or missing or len(samples) != 72 else 0)
PY
then
  pass=$((pass+1)); echo "PASS AC-ACC-3 real-component contrast"
else
  fail=$((fail+1)); echo "FAIL AC-ACC-3 real-component contrast"
fi

# ---------------------------------------------------------------- AC-ACC-4
ev "window.__mkbApp.getState().openSettings(); true" >/dev/null
wait_match "settings for custom" "document.querySelector('[data-testid=\"settings-accent-hex\"]') !== null" '"value":true' 30
set_value '[data-testid="settings-accent-hex"]' '#ffff00' input
wait_match "custom hint" "document.querySelector('[data-testid=\"settings-accent-hint\"]').textContent.indexOf('实际使用 #999900') >= 0" '"value":true' 20
ev "(() => { const el = document.querySelector('[data-testid=\"settings-accent-hex\"]'); el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); return true; })()" >/dev/null
wait_match "custom applied" "document.documentElement.dataset.accent === 'custom' && getComputedStyle(document.documentElement).getPropertyValue('--accent-light-accent').trim() === '#999900'" '"value":true' 30
check "AC-ACC-4 store keeps the input value" "window.__mkbApp.getState().settings.accent.hex === '#ffff00'" '"value":true'
sleep 1
python -I - "$APPD/Mile Knowledge Base/settings.json" <<'PY' && { pass=$((pass+1)); echo "PASS AC-ACC-4 file keeps the input value"; } || { fail=$((fail+1)); echo "FAIL AC-ACC-4 file keeps the input value"; }
import json, sys
accent = json.load(open(sys.argv[1], encoding="utf-8")).get("accent") or {}
sys.exit(0 if accent.get("preset") == "custom" and accent.get("hex") == "#ffff00" and accent.get("cache") else 1)
PY
stop_app
start_app "$APPD"
wait_match "restart with custom" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
check "AC-ACC-4 custom after restart" "document.documentElement.dataset.accent === 'custom' && getComputedStyle(document.documentElement).getPropertyValue('--accent-light-accent').trim() === '#999900'" '"value":true'

# ---------------------------------------------------------------- AC-ACC-5
stop_app
python -I - "$APPD/Mile Knowledge Base/settings.json" <<'PY'
import json, sys
path = sys.argv[1]
doc = json.load(open(path, encoding="utf-8"))
doc["accent"] = "乱码-not-a-colour"
open(path, "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
PY
start_app "$APPD"
wait_match "boot with garbage accent" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
check "AC-ACC-5 falls back to orange" "window.__mkbApp.getState().settings.accent.preset === 'orange' && document.documentElement.dataset.accent === 'orange'" '"value":true'
check "AC-ACC-5 no fault notice" "window.__mkbApp.getState().notices.every((n) => n.level !== 'fault')" '"value":true'
ev "window.__mkbApp.getState().openSettings(); true" >/dev/null
wait_match "settings with garbage accent" "document.querySelector('[data-testid=\"settings-accent-preset-orange\"]') !== null" '"value":true' 30
check "AC-ACC-5 orange checked in settings" "document.querySelector('[data-testid=\"settings-accent-preset-orange\"]').getAttribute('aria-checked') === 'true'" '"value":true'
click '[data-testid="settings-accent-preset-green"]'
wait_match "green picked again" "document.documentElement.dataset.accent === 'green'" '"value":true' 30
sleep 1
[ "$(read_accent)" = '{"preset": "green"}' ] && { pass=$((pass+1)); echo "PASS AC-ACC-5 can pick again"; } || { fail=$((fail+1)); echo "FAIL AC-ACC-5 can pick again :: $(read_accent)"; }
stop_app

echo "RESULT pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
