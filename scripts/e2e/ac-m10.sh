#!/usr/bin/env bash
# AC-M10: global material and motion on a real window (PRD 19, AC-MAT-1/2/3/4/8).
#   S1  AC-MAT-1  material distribution, std/std, light: title bar, shells, tiles,
#                 sheets, menus, dialogs, search and health panels
#       AC-MAT-2/3  the four blur levels and the four animation levels through the
#                 real settings rows: attributes, computed styles, settings.json
#       no false positive: ordinary scrolling does not degrade the blur
#   S2  AC-MAT-2/3  restart keeps the pair; the first-frame chain is in place
#   S3  AC-MAT-4  --force-prefers-reduced-motion (WebView2 flag)
#   S4  AC-MAT-8  really slow frames degrade the blur for this run only
#   S5  never judged while the user blur is off
#   S6  AC-MAT-1  the welcome page (no vault)
# AC-MAT-5 (contrast) is T10, AC-MAT-6/7/9 (performance) are T09, AC-MAT-10 is the
# full e2e run. Temp APPDATA, vault and fake HOME only; nothing real is touched.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.ac-m10-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base" "$HOME_DIR"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null)
SETTINGS="$APPD/Mile Knowledge Base/settings.json"

# write_settings <blur> <motion> [novault]
write_settings() {
  python -I - "$VAULT" "$SETTINGS" "$1" "$2" "${3:-vault}" <<'PY'
import hashlib, json, os, sys
v, out, blur, motion, mode = sys.argv[1:6]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "effects": {"motion": motion, "blur": blur},
       "layout": {"sidebarWidth": 232, "docWidth": 620, "docCollapsed": False, "sidebarCollapsed": False,
                  "agnesControlsCollapsed": False, "agnesHistoryOpen": True, "agnesComposerHeight": 96}}
if mode == "novault":
    doc["recentVaults"] = []; doc["lastVaultId"] = ""
else:
    c = os.path.abspath(v).replace("/", chr(92)).lower()
    vid = hashlib.sha256(c.encode()).hexdigest()[:16]
    doc["recentVaults"] = [{"id": vid, "root": v, "displayName": "ac-m10", "lastOpenedAt": "2026-11-27T09:00:00Z"}]
    doc["lastVaultId"] = vid
open(out, "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
PY
}
read_effects() {
  python -I -X utf8 -c "import json,sys;print(json.dumps(json.load(open(sys.argv[1],encoding='utf-8')).get('effects'),sort_keys=True))" "$SETTINGS"
}

DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1

PORT=18561
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  powershell -NoProfile -Command 'Get-Process "Mile Knowledge Base" -ErrorAction SilentlyContinue | Stop-Process -Force' >/dev/null 2>&1
  sleep 3
  # KEEP_WORK=1 keeps the temp tree (app log, settings, vault) for diagnosis.
  if [ "${KEEP_WORK:-0}" = "1" ]; then echo "WORK kept: $WORK"; else rm -rf "$WORK"; fi
}
trap cleanup EXIT

if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running; close it first (this script only kills what it starts)"
  exit 2
fi

# Bring the window to the front: a hidden or fully occluded WebView2 page
# stops rAF and (since M10 T09) hides the colour field, so the material
# assertions below would read an invisible page.
foreground_app() {
  [ -n "${APP_PID:-}" ] || return 0
  cat > "$WORK/focus.ps1" <<'PS'
param([int]$ProcId)
Add-Type -Namespace Win -Name Fg -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
'@
$p = Get-Process -Id $ProcId -ErrorAction SilentlyContinue
if ($p -and $p.MainWindowHandle -ne 0) { [Win.Fg]::SetForegroundWindow($p.MainWindowHandle) | Out-Null }
try { (New-Object -ComObject WScript.Shell).AppActivate($ProcId) | Out-Null } catch {}
PS
  powershell -NoProfile -ExecutionPolicy Bypass -File "$(cygpath -w "$WORK/focus.ps1")" -ProcId "$APP_PID" >/dev/null 2>&1
  sleep 0.8
}
start_app() { # [extra env ...]
  local attempt local_dir
  for attempt in 1 2 3; do
    # Wait for the automation port: the previous instance may still be closing.
    for _ in $(seq 1 40); do
      [ "$(powershell.exe -NoProfile -Command "@(Get-NetTCPConnection -State Listen -LocalPort $PORT -ErrorAction SilentlyContinue).Count" | tr -cd '0-9')" = "0" ] && break
      sleep 0.5
    done
    # A fresh WebView2 profile per launch: the real LOCALAPPDATA is shared with
    # everything else, and a launch right after a stop races the previous
    # profile's shutdown (T09: the S4 app never became ready).
    local_dir="$WORK/local-$(date +%s%N)"
    mkdir -p "$local_dir"
    env APPDATA="$(cygpath -w "$APPD")" LOCALAPPDATA="$(cygpath -w "$local_dir")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" SKILLS_FAKE_HOME="$(cygpath -w "$HOME_DIR")" "$@" "$APP" >"$WORK/app-$attempt.log" 2>&1 &
    sleep 6
    APP_PID=""
    for _ in $(seq 1 40); do
      APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' -ErrorAction SilentlyContinue | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -cd '0-9')
      [ -n "$APP_PID" ] && break
      sleep 0.5
    done
    if [ -n "$APP_PID" ]; then
      # Running is not enough: WebView2 can fail to initialise and the process
      # stays alive with no page. Only a responsive store counts as started.
      for _ in $(seq 1 40); do
        if ev "!!window.__mkbApp && !!window.__mkbApp.getState().settings" | grep -q '"value":true'; then
          # The page must also be visible: rAF (the scroll and slow-frame
          # measurements) and, since T09, the colour field depend on it.
          for _ in $(seq 1 40); do
            ev "document.visibilityState === 'visible'" | grep -q '"value":true' && return 0
            foreground_app
            sleep 0.5
          done
          echo "app started but the page stays hidden" >&2
        fi
        sleep 0.5
      done
    fi
    echo "start_app attempt $attempt failed (pid=${APP_PID:-none}); retrying" >&2
    [ -n "${APP_PID:-}" ] && powershell.exe -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1
    powershell.exe -NoProfile -Command 'Get-Process "Mile Knowledge Base" -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "*Mile-Knowledge-Base-Go*" } | Stop-Process -Force' >/dev/null 2>&1
    APP_PID=""
    sleep 3
  done
  echo "app did not start after 3 attempts" >&2
  return 1
}
stop_app() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  # Fallback: the ABORT guard above means any instance now is ours.
  powershell -NoProfile -Command 'Get-Process "Mile Knowledge Base" -ErrorAction SilentlyContinue | Stop-Process -Force' >/dev/null 2>&1
  APP_PID=""
  sleep 3
}
export MYGO_AUTOMATION="tcp:127.0.0.1:$PORT"

ev() {
  python -I - "$WORK/req.json" "$1" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "eval", "params": {"window": 1, "script": sys.argv[2]}}))
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

# Helpers every JS body can use. A body returns true, or a diagnostic string.
H=$(cat <<'JS'
const q = (s) => document.querySelector(s);
const cs = (s) => { const e = q(s); return e ? getComputedStyle(e) : null; };
const al = (s) => {
  const e = cs(s); if (!e) return -1;
  const m = e.backgroundColor.match(/\/\s*([0-9.]+)\)/); if (m) return +m[1];
  const r = e.backgroundColor.match(/^rgba\(.*,\s*([0-9.]+)\)$/); return r ? +r[1] : 1;
};
const bf = (s) => { const e = cs(s); return e ? e.backdropFilter : "missing"; };
const px = (s) => { const m = bf(s).match(/blur\(([0-9.]+)px\)/); return m ? +m[1] : 0; };
const root = (n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim();
const attr = (n) => document.documentElement.dataset[n] || null;
// the build minifier rewrites "520ms" to ".52s", so durations are compared as milliseconds
const ms = (n) => { const m = root(n).match(/^(-?[0-9.]+)(ms|s)$/); return m ? Math.round((m[2] === "s" ? 1000 : 1) * +m[1]) : root(n); };
const len = (n) => parseFloat(root(n));
JS
)
chk() { check "$1" "(() => { $H $2 })()" '"value":true'; }
wait_chk() { wait_match "$1" "(() => { $H $2 })()" '"value":true' "${3:-40}"; }

# ====================================================================== S1
echo "== S1: std/std, light: AC-MAT-1 distribution, AC-MAT-2/3 levels"
write_settings std std
VID=$(python -I -c "import json,sys;print(json.load(open(sys.argv[1],encoding='utf-8'))['lastVaultId'])" "$SETTINGS")
start_app
wait_match "boot" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
chk "the window is visible, so the material can be measured" 'return document.visibilityState === "visible" || document.visibilityState;'
chk "attributes on <html> from the stored pair" 'return attr("blur")==="std" && attr("motion")==="std" && attr("theme")==="light" || JSON.stringify({b:attr("blur"),m:attr("motion"),t:attr("theme")});'

# ordinary use must not trip the degrade guard: scroll for real, keep measuring frames
ev "(() => { window.__gaps = []; let p = 0; const t = (now) => { if (p) window.__gaps.push(now - p); p = now; if (window.__gaps.length < 200) requestAnimationFrame(t); }; requestAnimationFrame(t); const s = document.querySelector('.tree-scroll'); s.dispatchEvent(new Event('scroll')); let n = 0; const id = setInterval(() => { s.scrollTop = n % 2 ? 0 : 120; n += 1; if (n > 20) clearInterval(id); }, 80); return true; })()" >/dev/null
sleep 5
echo "evidence (normal frames while scrolling): $(ev "(() => { const g = window.__gaps.slice().sort((a,b)=>a-b); return JSON.stringify({n: g.length, median: +g[Math.floor(g.length/2)].toFixed(2), p95: +g[Math.floor(g.length*0.95)].toFixed(2)}); })()")"
chk "normal scrolling does not degrade the blur" 'const g = window.__gaps.slice().sort((a,b)=>a-b); const p95 = g[Math.floor(g.length*0.95)]; return (attr("blurAuto")===null && window.__mkbApp.getState().effectsDegraded===false && g.length>=120 && p95<33.4) || JSON.stringify({auto:attr("blurAuto"),n:g.length,p95});'

ev "window.__mkbDocs.getState().open('$VID', 'raws/富文本样例.md'); true" >/dev/null
wait_match "doc open" "document.querySelector('[data-panel=editor] .editor-body') !== null" '"value":true' 60
sleep 1.5

chk "AC-MAT-1 title bar and shells are glass without a filter of their own" 'const names = ["header.titlebar", "[data-panel=sidebar]", "[data-panel=agnes]", "[data-panel=editor]"]; const bad = names.filter((n) => !(al(n) > 0.3 && al(n) < 1 && bf(n) === "none")); return bad.length === 0 || JSON.stringify(bad.map((n) => [n, al(n), bf(n)]));'
chk "AC-MAT-1 colour field sits behind everything and drifts" 'const f = q(".background-field"); if (!f) return "no field"; const r = f.getBoundingClientRect(); const s = getComputedStyle(f); const a = getComputedStyle(f.querySelector("span")).animationName; return (s.zIndex === "-1" && Math.abs(r.top) < 1 && r.width >= innerWidth - 1 && s.pointerEvents === "none" && a !== "none") || JSON.stringify({z:s.zIndex,top:r.top,w:r.width,a});'
# M10 T09: while the page is hidden the field is not only paused but removed
# from the render tree (the 48px blur layers cost CPU when minimized). The
# attribute itself is set from visibilitychange; the real minimize was checked
# with a probe (spikes/m10-effects/t09), this asserts the rule.
chk "a hidden page removes the colour field entirely" 'const htmlEl = document.documentElement; htmlEl.setAttribute("data-field-paused", ""); const hidden = getComputedStyle(q(".background-field")).display; htmlEl.removeAttribute("data-field-paused"); const shown = getComputedStyle(q(".background-field")).display; return (hidden === "none" && shown !== "none") || JSON.stringify({ hidden, shown });'
chk "AC-MAT-1 text-dense regions are opaque sheets" 'const names = [".tree-scroll", ".history-list", "[data-panel=editor] .editor-body", ".composer-input"]; const bad = names.filter((n) => al(n) !== 1 || bf(n) !== "none"); return bad.length === 0 || JSON.stringify(bad.map((n) => [n, al(n), bf(n)]));'
chk "AC-MAT-1 overview tiles and run parameters are card-tier glass" 'const names = ["[data-testid=overview-skills]", ".runtime-controls"]; const bad = names.filter((n) => !(al(n) > 0.3 && al(n) < 1 && px(n) === 18)); return bad.length === 0 || JSON.stringify(bad.map((n) => [n, al(n), bf(n)]));'
chk "AC-MAT-1 an empty chat is not a sheet, so the tiles sit on the glass" 'const b = q("[data-testid=chat-body]"); return (b.classList.contains("sheet") === false && b.getAttribute("data-empty") === "true") || b.className;'

echo "-- search panel"
ev "window.__mkbSearch.getState().openPanel(); true" >/dev/null
wait_match "search open" "document.querySelector('.search-body') !== null" '"value":true' 30
chk "AC-MAT-1 search panel is strong glass over an opaque result area" 'return (al(".search-panel") > 0.5 && al(".search-panel") < 1 && px(".search-panel") === 24 && al(".search-body") === 1 && bf(".search-body") === "none") || JSON.stringify({p:al(".search-panel"),f:bf(".search-panel"),b:al(".search-body")});'
ev "window.__mkbSearch.getState().closePanel(); true" >/dev/null

echo "-- route menu"
click '[data-testid="route-target"]' >/dev/null
wait_match "route menu open" "document.querySelector('.agent-picker') !== null" '"value":true' 30
chk "AC-MAT-1 menu is strong glass with a 24px blur" 'return (al(".agent-picker") > 0.5 && al(".agent-picker") < 1 && px(".agent-picker") === 24) || JSON.stringify({a:al(".agent-picker"),f:bf(".agent-picker")});'
ev "document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); true" >/dev/null

echo "-- plan card and health panel"
VID_JS="window.__mkbApp.getState().activeVaultId"
ev "window.__mkbMutation.getState().propose({ id: 'p-m10', vaultId: $VID_JS, origin: 'user', risk: 'medium', actions: [{ id: 'a1', kind: 'create', target: 'notes/m10.md', text: 'm10', reason: 'ac-m10' }] }, 'ac-m10'); true" >/dev/null
wait_match "plan card open" "document.querySelector('.plan-card') !== null" '"value":true' 30
chk "AC-MAT-1 plan card is strong glass and its scrim blurs behind it" 'const sc = getComputedStyle(q(".plan-backdrop"), "::before"); return (al(".plan-card") > 0.5 && al(".plan-card") < 1 && px(".plan-card") === 24 && /blur\(8px\)/.test(sc.backdropFilter)) || JSON.stringify({a:al(".plan-card"),f:bf(".plan-card"),s:sc.backdropFilter});'
ev "window.__mkbMutation.getState().cancel(); true" >/dev/null
ev "window.__mkbApp.getState().openHealth(); true" >/dev/null
wait_match "health open" "document.querySelector('.health-panel') !== null" '"value":true' 30
chk "AC-MAT-1 health panel is strong glass over an opaque list" 'return (al(".health-panel") > 0.5 && al(".health-panel") < 1 && px(".health-panel") === 24 && al(".health-body") === 1) || JSON.stringify({a:al(".health-panel"),f:bf(".health-panel"),b:al(".health-body")});'
ev "window.__mkbApp.getState().closeHealth(); true" >/dev/null
sleep 0.5

echo "-- switching levels on the workbench does not replay the entrance"
ev "window.__mkbApp.getState().setEffects({ blur: 'low' }); true" >/dev/null
sleep 0.6
chk "no entrance animation restarts after a blur switch" 'const n = document.getAnimations().filter((a) => a.playState === "running" && a.animationName === "effects-rise").length; return n === 0 || n;'
ev "window.__mkbApp.getState().setEffects({ blur: 'std' }); true" >/dev/null
sleep 0.6

echo "-- AC-MAT-2: the four blur levels through the settings rows"
ev "window.__mkbApp.getState().openSettings(); true" >/dev/null
wait_match "settings open" "document.querySelector('[data-testid=\"effects-blur\"]') !== null" '"value":true' 30
sleep 1
# P00 (M10.1): the two panels must reach the bottom edge of the window content
# area minus one panel gap. On 1.8.0 this FAILS because .settings-grid has no
# height, so the panels stop at the content's end (P01 makes it pass).
chk "settings panels reach the window bottom minus one gap" 'const gap = parseFloat(root("--gap-panel")) || 8; const nav = q("[data-panel=settings-nav]").getBoundingClientRect(); const host = q("[data-panel=settings-content]").getBoundingClientRect(); const miss = [Math.round(innerHeight - gap - nav.bottom), Math.round(innerHeight - gap - host.bottom)]; return (Math.abs(miss[0]) <= 1 && Math.abs(miss[1]) <= 1) || JSON.stringify({ miss, innerHeight, gap, navBottom: nav.bottom, hostBottom: host.bottom });'
BLUR_TPL='const p = +al("[data-panel=settings-content]").toFixed(2), c = +al(".setting-card").toFixed(2); const f = q(".background-field"); const fo = f ? +getComputedStyle(f).opacity : -1; const ok = p === @P@ && c === @C@ && root("--glass-blur") === "@B@" && px("header.titlebar") === 0 && ((@F@ === 0 && f === null) || (@F@ === 1 && !!f && Math.abs(fo - @O@) < 0.01)); return ok || JSON.stringify({p, c, fo, hasField: !!f, blur: root("--glass-blur")});'
# blur_case <level> <panel alpha> <card alpha> <field 0|1> <field opacity> <--glass-blur>
blur_case() {
  click "[data-testid=\"effects-blur-$1\"]" >/dev/null
  wait_chk "blur $1 reaches <html>" "return attr(\"blur\") === \"$1\" || attr(\"blur\");" 20
  sleep 0.5
  local body="${BLUR_TPL//@P@/$2}"; body="${body//@C@/$3}"; body="${body//@F@/$4}"; body="${body//@O@/$5}"; body="${body//@B@/$6}"
  chk "AC-MAT-2 blur $1: panel alpha, card alpha, --glass-blur and the field" "$body"
  chk "AC-MAT-2 blur $1: the choice is checked" "return (q(\"[data-testid=effects-blur-$1]\").getAttribute(\"aria-checked\") === \"true\") || \"not checked\";"
  [ "$(read_effects)" = "{\"blur\": \"$1\", \"motion\": \"std\"}" ] && { pass=$((pass+1)); echo "PASS AC-MAT-2 blur $1 stored: $(read_effects)"; } || { fail=$((fail+1)); echo "FAIL AC-MAT-2 blur $1 stored :: $(read_effects)"; }
}
blur_case off 1 1 0 0 0px
blur_case low 0.76 0.7 1 0.6 10px
blur_case std 0.58 0.5 1 1 18px
blur_case high 0.34 0.28 1 1 40px

echo "-- AC-MAT-3: the four animation levels"
MOTION_TPL='const a = getComputedStyle(q(".background-field span")).animationName; const drifting = a !== "none"; return (ms("--dur-enter") === @D@ && len("--lift") === @L@ && drifting === @W@) || JSON.stringify({d: root("--dur-enter"), l: root("--lift"), a});'
# motion_case <level> <--dur-enter in ms> <--lift in px> <field drifts: true|false>
motion_case() {
  click "[data-testid=\"effects-motion-$1\"]" >/dev/null
  wait_chk "motion $1 reaches <html>" "return attr(\"motion\") === \"$1\" || attr(\"motion\");" 20
  sleep 0.4
  local body="${MOTION_TPL//@D@/$2}"; body="${body//@L@/$3}"; body="${body//@W@/$4}"
  chk "AC-MAT-3 motion $1: duration, lift and field drift" "$body"
  [ "$(read_effects)" = "{\"blur\": \"high\", \"motion\": \"$1\"}" ] && { pass=$((pass+1)); echo "PASS AC-MAT-3 motion $1 stored: $(read_effects)"; } || { fail=$((fail+1)); echo "FAIL AC-MAT-3 motion $1 stored :: $(read_effects)"; }
}
motion_case off 0 0 false
motion_case low 240 1 false
motion_case std 520 3 true
motion_case high 640 4 true
# leave a distinctive pair behind for S2
click '[data-testid="effects-motion-low"]' >/dev/null
click '[data-testid="effects-blur-low"]' >/dev/null
sleep 1.2
[ "$(read_effects)" = '{"blur": "low", "motion": "low"}' ] && { pass=$((pass+1)); echo "PASS pair saved for the restart: $(read_effects)"; } || { fail=$((fail+1)); echo "FAIL pair saved for the restart :: $(read_effects)"; }

echo "-- AC-MAT-2: blur off on the workbench is the 1.7.0 look"
click '[data-testid="effects-blur-off"]' >/dev/null
sleep 0.6
ev "window.__mkbApp.getState().closeSettings(); true" >/dev/null
wait_match "workbench back" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 30
sleep 1
chk "AC-MAT-2 blur off: shells, tiles and menus are opaque, no filter, no field" 'const names = ["header.titlebar", "[data-panel=sidebar]", "[data-panel=agnes]", "[data-panel=editor]", "[data-testid=overview-skills]", ".runtime-controls"]; const bad = names.filter((n) => al(n) !== 1 || bf(n) !== "none"); return (bad.length === 0 && q(".background-field") === null) || JSON.stringify({bad: bad.map((n) => [n, al(n), bf(n)]), field: !!q(".background-field")});'
stop_app

# ====================================================================== S2
echo "== S2: restart keeps the pair and the first frame is already painted with it"
# S1 ended with blur off + motion low in the file (the last click above)
[ "$(read_effects)" = '{"blur": "off", "motion": "low"}' ] && { pass=$((pass+1)); echo "PASS file before restart: $(read_effects)"; } || { fail=$((fail+1)); echo "FAIL file before restart :: $(read_effects)"; }
# Earliest reachable eval after launch: the attributes come from PreloadScript +
# theme-init.js, not from React, so they are there even if settings are not loaded yet.
LOCAL2="$WORK/local-$(date +%s%N)"; mkdir -p "$LOCAL2"
env APPDATA="$(cygpath -w "$APPD")" LOCALAPPDATA="$(cygpath -w "$LOCAL2")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" SKILLS_FAKE_HOME="$(cygpath -w "$HOME_DIR")" "$APP" >"$WORK/app.log" 2>&1 &
first=""
for i in $(seq 1 120); do
  first=$(ev "JSON.stringify({blur: document.documentElement.dataset.blur || null, motion: document.documentElement.dataset.motion || null, preload: window.__MKB_EFFECTS__ || null, root: !!document.getElementById('root'), storeUp: !!window.__mkbApp, settingsLoaded: !!(window.__mkbApp && window.__mkbApp.getState().settings), href: location.pathname})")
  echo "$first" | grep -q 'root\\":true' && break
  sleep 0.1
done
echo "first reachable state: $first"
echo "$first" | grep -q 'blur\\":\\"off' && echo "$first" | grep -q 'motion\\":\\"low' && { pass=$((pass+1)); echo "PASS AC-MAT-2 first reachable state already has the saved pair"; } || { fail=$((fail+1)); echo "FAIL AC-MAT-2 first reachable state :: $first"; }
echo "$first" | grep -q 'settingsLoaded\\":false' && echo "NOTE the settings store was still empty at that moment, so the attributes can only have come from the preload" || echo "NOTE the settings store had already loaded at that moment; this reading alone is not conclusive about the first frame, the iframe checks below are"
sleep 6
APP_PID=""
for _ in $(seq 1 40); do
  APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' -ErrorAction SilentlyContinue | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -cd '0-9')
  [ -n "$APP_PID" ] && break
  sleep 0.5
done
foreground_app
chk "AC-MAT-2 the preload is the saved pair and theme-init.js runs before the app module" 'const p = window.__MKB_EFFECTS__; const order = [...document.scripts].map((s) => s.src.split("/").pop()); const i = order.findIndex((n) => n.includes("theme-init")); const j = [...document.scripts].findIndex((s) => s.type === "module"); return (p && p.blur === "off" && p.motion === "low" && i >= 0 && (j < 0 || i < j)) || JSON.stringify({p, order});'
# The check above cannot prove the very first frame (the store was already loaded when
# the first eval got through). This one can: run the shipped /theme-init.js in a clean
# document that only has the preload value and read what it wrote, before any React.
chk "AC-MAT-2 theme-init.js alone paints the pair onto a clean <html>" 'return new Promise((resolve) => { const f = document.createElement("iframe"); f.style.display = "none"; document.body.appendChild(f); f.contentWindow.__MKB_EFFECTS__ = { blur: "high", motion: "off" }; const s = f.contentDocument.createElement("script"); s.src = "/theme-init.js"; s.onload = () => { const d = f.contentDocument.documentElement.dataset; const ok = d.blur === "high" && d.motion === "off"; const seen = JSON.stringify({ blur: d.blur, motion: d.motion }); f.remove(); resolve(ok || seen); }; s.onerror = () => { f.remove(); resolve("theme-init.js did not load"); }; f.contentDocument.head.appendChild(s); });'
chk "AC-MAT-2 a garbage preload leaves <html> alone" 'return new Promise((resolve) => { const f = document.createElement("iframe"); f.style.display = "none"; document.body.appendChild(f); f.contentWindow.__MKB_EFFECTS__ = { blur: "huge", motion: 7 }; const s = f.contentDocument.createElement("script"); s.src = "/theme-init.js"; s.onload = () => { const d = f.contentDocument.documentElement.dataset; const ok = d.blur === undefined && d.motion === undefined; const seen = JSON.stringify({ blur: d.blur, motion: d.motion }); f.remove(); resolve(ok || seen); }; s.onerror = () => { f.remove(); resolve("theme-init.js did not load"); }; f.contentDocument.head.appendChild(s); });'
wait_chk "AC-MAT-2 the store agrees with the file after the restart" 'const s = window.__mkbApp && window.__mkbApp.getState().settings; return (!!s && s.effects.blur === "off" && s.effects.motion === "low" && attr("blur") === "off") || false;' 40
stop_app

# ====================================================================== S3
echo "== S3: AC-MAT-4: the system animation switch is off"
write_settings high high
start_app WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--force-prefers-reduced-motion
wait_match "boot" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
chk "the media query is on and mirrored into the store" 'return (matchMedia("(prefers-reduced-motion: reduce)").matches === true && window.__mkbApp.getState().systemReducedMotion === true) || false;'
chk "AC-MAT-4 durations, lift and the field drift behave as animation off" 'const a = getComputedStyle(q(".background-field span")).animationName; return (ms("--dur-enter") === 0 && ms("--dur-hover") === 0 && len("--lift") === 0 && a === "none") || JSON.stringify({d: root("--dur-enter"), h: root("--dur-hover"), l: root("--lift"), a});'
chk "AC-MAT-4 entrance animations do not run" 'const n = document.getAnimations().filter((x) => x.playState === "running" && x.animationName === "effects-rise").length; return n === 0 || n;'
chk "AC-MAT-4 the user level is untouched" 'return (attr("motion") === "high" && window.__mkbApp.getState().settings.effects.motion === "high") || attr("motion");'
ev "window.__mkbApp.getState().openSettings(); true" >/dev/null
wait_match "settings open" "document.querySelector('[data-testid=\"effects-motion-note\"]') !== null" '"value":true' 30
chk "AC-MAT-4 the note explains it and the choice is still shown and editable" 'const n = q("[data-testid=effects-motion-note]"); const r = q("[data-testid=effects-motion-high]"); return (n.textContent === "系统已关闭动画效果，当前按「关」显示" && r.getAttribute("aria-checked") === "true" && r.disabled === false && !q("[data-testid=effects-blur-note]")) || JSON.stringify({t: n.textContent, c: r.getAttribute("aria-checked"), d: r.disabled});'
stop_app

# ====================================================================== S4
echo "== S4: AC-MAT-8: really slow frames while scrolling"
write_settings std std
start_app
wait_match "boot" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
sleep 4
# The idle phase has finished by now. Block the main thread ~55 ms per frame and
# scroll; record the gaps the page itself sees as evidence that the frames are slow.
ev "(() => { window.__spin = true; window.__slow = []; let p = 0; const spin = (now) => { if (p) window.__slow.push(now - p); p = now; const t = performance.now(); while (performance.now() - t < 55) {} if (window.__spin) requestAnimationFrame(spin); }; requestAnimationFrame(spin); const s = document.querySelector('.tree-scroll'); s.dispatchEvent(new Event('scroll')); window.__sc = setInterval(() => { s.scrollTop = s.scrollTop > 0 ? 0 : 100; }, 60); return true; })()" >/dev/null
wait_chk "AC-MAT-8 the blur switched itself off" 'return attr("blurAuto") === "off" || attr("blurAuto");' 120
ev "window.__spin = false; clearInterval(window.__sc); true" >/dev/null
sleep 1
echo "evidence (slow frames while scrolling): $(ev "(() => { const g = window.__slow.slice(0, 120).sort((a,b)=>a-b); return JSON.stringify({n: g.length, median: +g[Math.floor(g.length/2)].toFixed(2), p95: +g[Math.floor(g.length*0.95)].toFixed(2)}); })()")"
chk "the frames really were slow" 'const g = window.__slow.slice(0, 120).sort((a,b)=>a-b); const p95 = g[Math.floor(g.length*0.95)]; return (g.length > 30 && p95 > 33.4) || JSON.stringify({n: g.length, p95});'
chk "AC-MAT-8 the colour field is gone and the shells are opaque" 'const names = ["header.titlebar", "[data-panel=sidebar]", "[data-panel=agnes]", "[data-testid=overview-skills]"]; const bad = names.filter((n) => al(n) !== 1 || bf(n) !== "none"); return (bad.length === 0 && q(".background-field") === null && window.__mkbApp.getState().effectsDegraded === true) || JSON.stringify({bad, field: !!q(".background-field")});'
chk "AC-MAT-8 the user value is unchanged" 'return (attr("blur") === "std" && window.__mkbApp.getState().settings.effects.blur === "std") || attr("blur");'
[ "$(read_effects)" = '{"blur": "std", "motion": "std"}' ] && { pass=$((pass+1)); echo "PASS AC-MAT-8 settings.json unchanged: $(read_effects)"; } || { fail=$((fail+1)); echo "FAIL AC-MAT-8 settings.json :: $(read_effects)"; }
ev "window.__mkbApp.getState().openSettings(); true" >/dev/null
wait_match "settings open" "document.querySelector('[data-testid=\"effects-blur-note\"]') !== null" '"value":true' 30
chk "AC-MAT-8 the settings page says so and the control stays usable" 'const n = q("[data-testid=effects-blur-note]"); const r = q("[data-testid=effects-blur-std]"); return (n.textContent === "当前设备滚动掉帧，已自动关闭模糊（本次运行）" && n.getAttribute("data-tone") === "warn" && r.getAttribute("aria-checked") === "true" && r.disabled === false) || n.textContent;'
click '[data-testid="effects-blur-high"]' >/dev/null
sleep 1
chk "AC-MAT-8 choosing another level does not undo the degrade for this run" 'return (attr("blur") === "high" && attr("blurAuto") === "off" && al("[data-panel=settings-content]") === 1) || JSON.stringify({b: attr("blur"), a: attr("blurAuto"), al: al("[data-panel=settings-content]")});'
stop_app
echo "-- the degrade is for one run only"
start_app
wait_match "boot again" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
sleep 1
chk "AC-MAT-8 a new launch is back to the stored level" 'return (attr("blurAuto") === null && window.__mkbApp.getState().effectsDegraded === false && attr("blur") === "high" && q(".background-field") !== null) || JSON.stringify({a: attr("blurAuto"), b: attr("blur")});'
stop_app

# ====================================================================== S5
echo "== S5: nothing is judged while the user blur is off"
write_settings off std
start_app
wait_match "boot" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" '"value":true' 90
sleep 4
ev "(() => { window.__spin = true; const spin = () => { const t = performance.now(); while (performance.now() - t < 55) {} if (window.__spin) requestAnimationFrame(spin); }; requestAnimationFrame(spin); const s = document.querySelector('.tree-scroll'); s.dispatchEvent(new Event('scroll')); window.__sc = setInterval(() => { s.scrollTop = s.scrollTop > 0 ? 0 : 100; }, 60); return true; })()" >/dev/null
sleep 12
ev "window.__spin = false; clearInterval(window.__sc); true" >/dev/null
chk "no degrade and no note when the user already chose off" 'return (attr("blurAuto") === null && window.__mkbApp.getState().effectsDegraded === false) || attr("blurAuto");'
stop_app

# ====================================================================== S6
echo "== S6: AC-MAT-1: the welcome page"
write_settings std std novault
start_app
wait_match "welcome" "document.querySelector('.welcome-shell') !== null" '"value":true' 90
sleep 1.5
chk "AC-MAT-1 the colour field shows through the welcome shell" 'return (al(".welcome-shell") === 0 && q(".background-field") !== null) || al(".welcome-shell");'
chk "AC-MAT-1 the welcome card is strong glass" 'return (al(".welcome-hero") > 0.5 && al(".welcome-hero") < 1 && px(".welcome-hero") === 24) || JSON.stringify({a: al(".welcome-hero"), f: bf(".welcome-hero")});'
chk "AC-MAT-1 the title bar is glass here too" 'return (al("header.titlebar") > 0.3 && al("header.titlebar") < 1) || al("header.titlebar");'
ev "[...document.querySelectorAll('.welcome-actions button')].find((b) => b.textContent.includes('新建'))?.click(); true" >/dev/null
wait_match "guide open" "document.querySelector('.onboarding') !== null" '"value":true' 30
chk "AC-MAT-1 the new-vault guide is strong glass over a blurred scrim" 'const sc = getComputedStyle(q(".welcome-guide-backdrop"), "::before"); return (al(".onboarding") > 0.5 && al(".onboarding") < 1 && px(".onboarding") === 24 && /blur\(8px\)/.test(sc.backdropFilter)) || JSON.stringify({a: al(".onboarding"), s: sc.backdropFilter});'
stop_app

echo "RESULT pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
