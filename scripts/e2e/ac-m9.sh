#!/usr/bin/env bash
# AC-M9: the skills page experience on a real window (PRD §18, AC-UX-1–12).
# Uses a temporary APPDATA, vault and fake HOME, plus a local fake GitHub
# (SKILLS_FAKE_GITHUB, e2e build only): the real GitHub and the real tool
# directories are never contacted. AC-UX-13 is the other ac-m* scripts, which
# were rewritten for the dialogs (M9 T02/T03) and are run alongside this one.
#
# Notes on how the steps are driven:
# - The automation backend refuses pointer input for WebView2 pages, so clicks
#   are DOM clicks; focus assertions use element.focus() where a pointer would
#   have moved focus.
# - AC-UX-5 installs from the local fake GitHub rather than a local folder: the
#   local-folder source opens a native folder dialog that cannot be driven.
# - AC-UX-12 uses --force-prefers-reduced-motion. The skills page keeps the
#   keyframes but shortens them to 1 ms (DESIGN 13.5), so the assertion is the
#   computed duration, not `animation-name: none`.
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.ac-m9-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"; FIX="$WORK/fixture"
mkdir -p "$VAULT" "$APPD/Mile Knowledge Base" "$HOME_DIR/.claude/skills" "$HOME_DIR/.local/bin" "$FIX"
(cd "$ROOT" && go run ./cmd/genvault -kind typical -out "$(cygpath -w "$VAULT")" >/dev/null)
printf '@echo off\n' > "$HOME_DIR/.local/bin/claude.cmd"
REGISTRY="$WORK/registry.json"
printf '{"uninstall": []}\n' > "$REGISTRY"

# Knowledge-base skills of this run: three for the batch, one for the overwrite
# case, one deleted before its drawer opens, and 40 so the list must scroll.
python -I - "$VAULT" "$HOME_DIR" <<'PY'
import os, sys
vault, home = sys.argv[1], sys.argv[2]
def skill(category, name, body="", description=""):
    d = os.path.join(vault, "skills", category, name)
    os.makedirs(d, exist_ok=True)
    open(os.path.join(d, "SKILL.md"), "w", encoding="utf-8", newline="\n").write(
        "---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s\n" % (name, description or ("技能 " + name), name, body))
skill("前端", "vue", "组合式 API 的写法。")
skill("前端", "react", "函数组件与 Hook。")
skill("前端", "mixa", "混合示例。")
skill("后端", "go-tips", "新版 go-tips。")
for i in range(40):
    skill("zz-scroll", "scroll-%02d" % i, "", "用于滚动与列表布局回归的技能 %02d。" % i)
old = os.path.join(home, ".claude", "skills", "go-tips")
os.makedirs(old, exist_ok=True)
open(os.path.join(old, "SKILL.md"), "w", encoding="utf-8", newline="\n").write("# old go-tips\n\n独立的旧版本。\n")
PY

VID=$(python -I - "$VAULT" "$APPD/Mile Knowledge Base" <<'PY'
import hashlib, json, os, sys
vault, appdir = sys.argv[1], sys.argv[2]
c = os.path.abspath(vault).replace("/", chr(92)).lower()
vid = hashlib.sha256(c.encode()).hexdigest()[:16]
doc = {"version": 1, "theme": "light", "closeBehavior": "quit",
       "layout": {"sidebarWidth": 232, "docWidth": 560, "docCollapsed": False,
                  "sidebarCollapsed": False, "agnesControlsCollapsed": False,
                  "agnesHistoryOpen": True, "agnesComposerHeight": 96},
       "recentVaults": [{"id": vid, "root": vault, "displayName": "ac-m9", "lastOpenedAt": "2026-11-25T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)

# --- fake GitHub: one repository with two skills ------------------------------
STATE="$FIX/state.json"
python -I - "$(cygpath -w "$FIX")" "$(cygpath -w "$STATE")" <<'PY'
import io, json, os, sys, tarfile
fix, state_path = sys.argv[1], sys.argv[2]
path = os.path.join(fix, "v1.tar.gz")
files = {
    "skills/pdf/SKILL.md": b"# pdf\n\nversion one\n",
    "skills/docx/SKILL.md": b"# docx\n\nversion one\n",
}
# Enough small files that copying docx takes a moment: AC-UX-6 needs an install
# that is still running when the dialog is poked.
for i in range(1500):
    files["skills/docx/ref/r%04d.md" % i] = (b"reference %d\n" % i) * 20
with tarfile.open(path, "w:gz") as tar:
    for rel, data in files.items():
        info = tarfile.TarInfo("repo/" + rel)
        info.size = len(data)
        tar.addfile(info, io.BytesIO(data))
h1 = "1" * 40
state = {"repos": {"anthropics/skills": {"ref": "main", "head": h1,
                                         "paths": {"skills/pdf": h1, "skills/docx": h1},
                                         "archives": {h1: path}}}}
with open(state_path, "w", encoding="utf-8") as handle:
    json.dump(state, handle)
PY
GH_PORT=18533
if python -I - "$GH_PORT" <<'PY'
import socket, sys
sock = socket.socket()
try:
    rc = sock.connect_ex(("127.0.0.1", int(sys.argv[1])))
finally:
    sock.close()
sys.exit(0 if rc == 0 else 1)
PY
then
  echo "ABORT: 127.0.0.1:$GH_PORT is already listening (stale fake GitHub?)"
  rm -rf "$WORK"; exit 2
fi
python -I "$ROOT/scripts/e2e/fake-github.py" "$(cygpath -w "$STATE")" "$GH_PORT" >"$WORK/github.log" 2>&1 &
GH_PID=$(powershell -NoProfile -Command "(Get-CimInstance Win32_Process | Where-Object { \$_.Name -eq 'python.exe' -and \$_.CommandLine -match 'fake-github' -and \$_.CommandLine -match '$GH_PORT' } | Select-Object -First 1).ProcessId" | tr -d '\r')
python -I - "$GH_PORT" <<'PY' || { echo "ABORT: fake GitHub did not start"; exit 2; }
import socket, sys, time
port = int(sys.argv[1])
for _ in range(100):
    try:
        with socket.create_connection(("127.0.0.1", port), 0.2):
            sys.exit(0)
    except OSError:
        time.sleep(0.1)
sys.exit(1)
PY

DRIVE="$WORK/drive.exe"
(cd "$ROOT/spikes/r4-window/drive" && go build -o "$DRIVE" .) || exit 1
PORT=18531
APP_PID=""
pass=0; fail=0
cleanup() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  powershell -NoProfile -Command "Get-CimInstance Win32_Process | Where-Object { \$_.Name -eq 'python.exe' -and \$_.CommandLine -match 'fake-github' -and \$_.CommandLine -match '$GH_PORT' } | ForEach-Object { Stop-Process -Id \$_.ProcessId -Force -ErrorAction SilentlyContinue }" >/dev/null 2>&1
  sleep 2
  if [ "${fail:-0}" -ne 0 ]; then
    echo "WORK kept for diagnosis: $WORK"
  else
    rm -rf "$WORK"
  fi
}
trap cleanup EXIT
if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running"; exit 2
fi

w() { mkdir -p "$(dirname "$1")"; printf '%b' "$2" > "$1"; }
hash_dir() { python -I - "$1" <<'PY'
import hashlib, os, sys
root = sys.argv[1]
h = hashlib.sha256()
if os.path.isdir(root):
    for base, _, files in sorted(os.walk(root)):
        for name in sorted(files):
            path = os.path.join(base, name)
            h.update(os.path.relpath(path, root).replace(os.sep, "/").encode())
            h.update(b"\0")
            h.update(open(path, "rb").read())
            h.update(b"\0")
print(h.hexdigest())
PY
}

start_app() { # [extra env ...]
  env APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" \
    SKILLS_FAKE_HOME="$(cygpath -w "$HOME_DIR")" \
    SKILLS_FAKE_PATH="$(cygpath -w "$HOME_DIR/.local/bin")" \
    SKILLS_FAKE_REGISTRY="$(cygpath -w "$REGISTRY")" \
    SKILLS_FAKE_GITHUB="http://127.0.0.1:$GH_PORT" "$@" "$APP" >"$WORK/app.log" 2>&1 &
  sleep 7
  APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -d '\r')
}
stop_app() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  APP_PID=""
  sleep 2
}
export MYGO_AUTOMATION="tcp:127.0.0.1:$PORT"

ev() {
  python -I - "$WORK/req.json" "$1" <<'PY'
import json, sys
open(sys.argv[1], "w", encoding="utf-8").write(json.dumps({"id": 1, "method": "eval", "params": {"window": 1, "script": sys.argv[2]}}))
PY
  timeout 90 "$DRIVE" -f "$WORK/req.json" 2>&1 | grep -m1 '{"id"'
}
eval_value() { # script -> the value, as text
  ev "$1" | python -I -c "import json,sys; print(json.loads(sys.stdin.readline())['result'].get('value',''))"
}
jsq() { python -I -c "import json,sys;print(json.dumps(sys.argv[1]))" "$1"; }
click() { ev "(() => { const el=document.querySelector($(jsq "$1")); if(!el) throw new Error('missing'); el.click(); return true; })()"; }
fill() { # testid value
  ev "(() => { const el=document.querySelector('[data-testid=\"$1\"]'); if(!el) throw new Error('missing'); const setter=Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set; setter.call(el,$(jsq "$2")); el.dispatchEvent(new Event('input',{bubbles:true})); return true; })()" >/dev/null
}
esc() { ev "document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); true" >/dev/null; }

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
  sleep 0.4
}
resize_window() { # width height: restore, resize, and retry until the page reports it
  cat > "$WORK/resize.ps1" <<'PS'
param([int]$ProcId, [int]$Width, [int]$Height)
Add-Type -Namespace Win -Name Win -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr hWnd, IntPtr after, int x, int y, int cx, int cy, uint flags);
[DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int cmd);
'@
$p = Get-Process -Id $ProcId -ErrorAction SilentlyContinue
if ($p -and $p.MainWindowHandle -ne 0) {
  [Win.Win]::ShowWindow($p.MainWindowHandle, 9) | Out-Null
  [Win.Win]::SetWindowPos($p.MainWindowHandle, [IntPtr]::Zero, 0, 0, $Width, $Height, 0x0004) | Out-Null
}
PS
  local try width
  for try in 1 2 3 4 5; do
    powershell -NoProfile -ExecutionPolicy Bypass -File "$(cygpath -w "$WORK/resize.ps1")" -ProcId "$APP_PID" -Width "$1" -Height "$2" >/dev/null 2>&1
    sleep 0.8
    width=$(eval_value "window.innerWidth")
    if [ "${width:-9999}" -le "$(( $1 + 60 ))" ] 2>/dev/null; then return 0; fi
  done
  echo "resize did not take: innerWidth=$width"
}

check() { # label script pattern
  local out; out=$(ev "$2")
  if echo "$out" | grep -q "$3"; then pass=$((pass+1)); echo "PASS $1"; else fail=$((fail+1)); echo "FAIL $1 :: $out"; fi
}
check_file() { # label ok-condition(0 = true)
  if [ "$2" = "0" ]; then pass=$((pass+1)); echo "PASS $1"; else fail=$((fail+1)); echo "FAIL $1"; fi
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
T='"value":true'
diag() { echo "DIAG $(eval_value "$1")"; }
store_state() { eval_value "(() => { const s=window.__mkbSkills.getState(); const vue=(s.scan?.kb||[]).flatMap((g) => g.skills).find((k) => k.name==='vue'); return JSON.stringify({scanning:s.scan?.scanning, cachedAt:s.scan?.cachedAt, vueDist:(vue?.distributed||[]).length, kbCount:(s.scan?.kb||[]).flatMap((g) => g.skills).length, loading:s.loading, error:s.error}); })()"; }
probe_rescan() { # label script: force a scan, then see whether the condition now holds
  echo "PROBE $1 store before the forced scan: $(store_state)"
  ev "window.__mkbSkills.getState().loadScan('$VID'); true" >/dev/null
  sleep 3
  echo "PROBE $1 store after the forced scan: $(store_state)"
  echo "PROBE $1 after a forced scan: $(eval_value "$2")"
}
show_skills() { ev "window.__mkbApp.getState().showPage('skills'); true" >/dev/null; wait_match "skills page open" "document.querySelector('[data-testid=\"skills-page\"]') !== null" "$T" 20; }
card() { echo "document.querySelector('[data-testid=\"kb-card-$1\"]')"; }
within_viewport() { # selector
  echo "(() => { const r=document.querySelector($(jsq "$1"))?.getBoundingClientRect(); return !!r && r.left >= 0 && r.right <= window.innerWidth && r.top >= 0 && r.bottom <= window.innerHeight; })()"
}
open_card_button() { # skill label
  ev "(() => { const c=$(card "$1"); if(!c) throw new Error('missing card'); const b=[...c.querySelectorAll('button')].find((x)=>x.textContent.trim()==='$2'); if(!b) throw new Error('missing button'); b.click(); return true; })()" >/dev/null
}

# ===== run 1: dialogs, drawer, nav ===========================================
start_app
wait_match "overview" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" "$T" 90
wait_match "index ready" "!document.body.innerText.includes('未索引')" "$T" 90
show_skills
wait_match "scroll fixtures scanned" "document.querySelectorAll('[data-testid^=\"kb-card-scroll-\"]').length >= 40" "$T" 60
wait_match "other fixtures scanned" "$(card vue) !== null && $(card react) !== null && $(card mixa) !== null && $(card go-tips) !== null" "$T" 20

echo "== AC-UX-5: install in a dialog; scroll position and filter survive"
fill kb-skills-search "scroll"
wait_match "filter narrows the list" "$(card vue) === null && $(card scroll-00) !== null" "$T" 20
ev "(() => { const el=document.querySelector('.skills-page .kb-content'); el.scrollTop=300; return el.scrollTop; })()" >/dev/null
sleep 0.4
S0=$(eval_value "document.querySelector('.skills-page .kb-content').scrollTop")
if [ "${S0:-0}" -gt 100 ] 2>/dev/null; then pass=$((pass+1)); echo "PASS list scrolled to $S0 before installing"; else fail=$((fail+1)); echo "FAIL list did not scroll ($S0)"; fi
click '[data-testid="kb-install-top"]' >/dev/null
wait_match "install dialog opens" "document.querySelector('[data-testid=\"skills-modal\"]') !== null && document.querySelector('[data-testid=\"install-github-form\"]') !== null" "$T" 30
check "install is a dialog, not a page" "document.querySelector('[data-testid=\"skills-modal\"] [data-testid=\"install-github-form\"]') !== null" "$T"
fill install-github-input "anthropics/skills"
click '[data-testid="install-github-read"]' >/dev/null
wait_match "candidates" "document.querySelector('[data-testid=\"install-candidate-skills/pdf\"]') !== null && document.querySelector('[data-testid=\"install-candidate-skills/docx\"]') !== null" "$T" 60
click '[data-testid="install-candidate-skills/docx"]' >/dev/null
fill install-category "文档"
click '[data-testid="install-submit"]' >/dev/null
wait_match "install result in the dialog" "document.querySelector('[data-testid=\"skills-modal\"] [data-testid=\"install-result\"]') !== null" "$T" 60
check "result line" "document.querySelector('[data-testid=\"install-result\"]').textContent.includes('pdf → 知识库：已安装')" "$T"
check "a finished install can be closed (Esc)" "(() => { document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); return true; })()" "$T"
wait_match "dialog closed after the result" "document.querySelector('[data-testid=\"skills-modal\"]') === null" "$T" 20
[ -f "$VAULT/skills/文档/pdf/SKILL.md" ]; check_file "pdf landed in the vault" "$?"
check "filter kept" "document.querySelector('[data-testid=\"kb-skills-search\"]').value === 'scroll'" "$T"
S1=$(eval_value "document.querySelector('.skills-page .kb-content').scrollTop")
if [ "${S1:-0}" -ge "$((S0 - 40))" ] 2>/dev/null; then pass=$((pass+1)); echo "PASS scroll position kept ($S0 -> $S1)"; else fail=$((fail+1)); echo "FAIL scroll position lost ($S0 -> $S1)"; fi
check "still on the skills page, no full-page install view" "document.querySelector('[data-testid=\"skills-page\"]') !== null && document.querySelector('.skills-page .skills-main [data-testid=\"install-view\"]') === null" "$T"
fill kb-skills-search ""
wait_match "list refreshed with the new skill" "$(card pdf) !== null" "$T" 100 || probe_rescan "pdf card" "$(card pdf) !== null"

echo "== AC-UX-6: nothing closes the dialog while it installs"
click '[data-testid="kb-install-top"]' >/dev/null
wait_match "dialog opens again" "document.querySelector('[data-testid=\"install-github-form\"]') !== null" "$T" 30
fill install-github-input "anthropics/skills"
click '[data-testid="install-github-read"]' >/dev/null
wait_match "candidates again" "document.querySelector('[data-testid=\"install-candidate-skills/docx\"]') !== null" "$T" 60
ev "(() => { const pdf=document.querySelector('[data-testid=\"install-candidate-skills/pdf\"]'); if (pdf && pdf.checked && !pdf.disabled) pdf.click(); return true; })()" >/dev/null
fill install-category "文档"
echo "AC-UX-6 candidates: $(eval_value "JSON.stringify([...document.querySelectorAll('[data-testid^=install-candidate-]')].map((e) => [e.dataset.testid, e.checked, e.disabled]).concat([['submit-disabled', document.querySelector('[data-testid=install-submit]')?.disabled, document.querySelector('[data-testid=install-category]')?.value]]))")"
# Click 安装 and try to leave in the same turn: the install is in flight, so
# Esc, the close button and the scrim must all do nothing.
BUSY=$(eval_value "(async () => { document.querySelector('[data-testid=\"install-submit\"]').click(); await new Promise((r) => setTimeout(r, 40)); const busy=!!document.querySelector('[data-testid=\"install-busy\"]'); document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); const close=document.querySelector('[data-testid=\"skills-modal-close\"]'); close && close.click(); const scrim=document.querySelector('[data-testid=\"skills-modal\"]'); scrim.dispatchEvent(new MouseEvent('mousedown',{bubbles:true})); scrim.dispatchEvent(new MouseEvent('click',{bubbles:true})); await new Promise((r) => setTimeout(r, 40)); return JSON.stringify({busy, closeDisabled: close ? close.disabled : null, open: !!document.querySelector('[data-testid=\"skills-modal\"]')}); })()")
if echo "$BUSY" | grep -q '"busy":true' && echo "$BUSY" | grep -q '"open":true'; then pass=$((pass+1)); echo "PASS Esc, close and scrim ignored while installing"; else fail=$((fail+1)); echo "FAIL busy dialog left early :: $BUSY"; fi
wait_match "result after the install" "document.querySelector('[data-testid=\"install-result\"]') !== null" "$T" 60
esc
wait_match "closes once there is a result" "document.querySelector('[data-testid=\"skills-modal\"]') === null" "$T" 20
[ -f "$VAULT/skills/文档/docx/SKILL.md" ]; check_file "docx landed in the vault" "$?"
wait_match "docx card" "$(card docx) !== null" "$T" 40 || probe_rescan "docx card" "$(card docx) !== null"

echo "== AC-UX-7: batch distribute in a dialog"
ev "(() => { const c=$(card vue); c.querySelector('button[aria-label^=\"更多操作\"]').click(); return true; })()" >/dev/null
wait_match "card menu" "document.querySelector('[data-testid=\"kb-batch-toggle\"]') !== null" "$T" 20
click '[data-testid="kb-batch-toggle"]' >/dev/null
wait_match "batch bar" "document.querySelector('[data-testid=\"kb-batchbar\"]') !== null" "$T" 20
ev "(() => { for (const name of ['vue','react','mixa']) { const b=document.querySelector('.kb-check[aria-label=\"选择 ' + name + '\"]'); if (!b) throw new Error('missing check ' + name); if (b.getAttribute('aria-checked') !== 'true') b.click(); } return true; })()" >/dev/null
wait_match "three selected" "['vue','react','mixa'].every((n) => document.querySelector('.kb-check[aria-label=\"选择 ' + n + '\"]')?.getAttribute('aria-checked') === 'true')" "$T" 20
click '[data-testid="kb-batch-distribute"]' >/dev/null
wait_match "distribute dialog" "document.querySelector('[data-testid=\"skills-modal\"] [data-testid=\"distribute-dialog\"]') !== null" "$T" 30
wait_match "target grid" "document.querySelector('[data-testid=\"distribute-target-claude-code\"]') !== null" "$T" 20
click '[data-testid="distribute-target-claude-code"]' >/dev/null
click '[data-testid="distribute-submit"]' >/dev/null
wait_match "result in the dialog" "document.querySelector('[data-testid=\"skills-modal\"] [data-testid=\"distribute-result\"]') !== null" "$T" 60
check "three targets succeeded" "document.querySelector('[data-testid=\"distribute-summary\"]').textContent.includes('成功 3')" "$T"
click '[data-testid="distribute-done"]' >/dev/null
wait_match "dialog closed" "document.querySelector('[data-testid=\"skills-modal\"]') === null" "$T" 20
check "batch selection left" "document.querySelector('[data-testid=\"kb-batchbar\"]') === null" "$T"
wait_match "cards show distributed" "['vue','react','mixa'].every((n) => { const c=document.querySelector('[data-testid=\"kb-card-' + n + '\"]'); return !!c && !c.querySelector('.kb-card-none'); })" "$T" 100 || probe_rescan "distributed cards" "['vue','react','mixa'].every((n) => { const c=document.querySelector('[data-testid=\"kb-card-' + n + '\"]'); return !!c && !c.querySelector('.kb-card-none'); })"
for n in vue react mixa; do
  [ "$(hash_dir "$HOME_DIR/.claude/skills/$n")" = "$(hash_dir "$VAULT/skills/前端/$n")" ]; check_file "$n is in the tool directory" "$?"
done

echo "== AC-UX-8: overwrite needs a confirmation card"
OLD_HASH=$(hash_dir "$HOME_DIR/.claude/skills/go-tips")
open_card_button go-tips "分发"
wait_match "distribute dialog for go-tips" "document.querySelector('[data-testid=\"distribute-dialog\"]') !== null" "$T" 30
wait_match "preview marks the existing directory" "document.querySelector('[data-testid=\"distribute-target-claude-code\"] input[type=checkbox]') !== null" "$T" 30
click '[data-testid="distribute-target-claude-code"] input[type=checkbox]' >/dev/null
wait_match "confirmation card above the dialog" "document.querySelector('[data-testid=\"distribute-overwrite-confirm\"]') !== null && document.querySelector('[data-testid=\"skills-modal-over\"] [data-testid=\"distribute-overwrite-confirm\"]') !== null" "$T" 20
check "card says it backs up first" "document.querySelector('[data-testid=\"distribute-overwrite-confirm\"]').textContent.includes('备份')" "$T"
[ "$(hash_dir "$HOME_DIR/.claude/skills/go-tips")" = "$OLD_HASH" ]; check_file "nothing written before the yes" "$?"
click '[data-testid="distribute-overwrite-go"]' >/dev/null
wait_match "confirm closed" "document.querySelector('[data-testid=\"distribute-overwrite-confirm\"]') === null" "$T" 20
click '[data-testid="distribute-submit"]' >/dev/null
wait_match "overwrite result in the dialog" "document.querySelector('[data-testid=\"skills-modal\"] [data-testid=\"distribute-result\"]') !== null" "$T" 60
check "result says it succeeded" "document.querySelector('[data-testid=\"distribute-summary\"]').textContent.includes('成功 1')" "$T"
[ "$(hash_dir "$HOME_DIR/.claude/skills/go-tips")" = "$(hash_dir "$VAULT/skills/后端/go-tips")" ]; check_file "target now holds the vault version" "$?"
grep -rl "old go-tips" "$APPD" >/dev/null 2>&1; check_file "the old directory was backed up" "$?"
click '[data-testid="distribute-done"]' >/dev/null
wait_match "overwrite dialog closed" "document.querySelector('[data-testid=\"skills-modal\"]') === null" "$T" 20

echo "== AC-UX-1/2/3/4: detail drawer"
foreground_app
click '[data-testid="kb-card-pdf"]' >/dev/null
wait_match "drawer ready" "document.querySelector('[data-testid=\"skill-drawer\"]')?.getAttribute('data-state') === 'ready'" "$T" 30
check "drawer header and path" "(() => { const d=document.querySelector('[data-testid=\"skill-drawer\"]'); return d.textContent.includes('pdf') && d.textContent.includes('文档') && document.querySelector('[data-testid=\"skill-drawer-path\"]').textContent === 'skills/文档/pdf/SKILL.md'; })()" "$T"
check "source block names the repository" "document.querySelector('[data-testid=\"skill-drawer-source\"]')?.textContent.includes('anthropics/skills') === true" "$T"
wait_match "SKILL.md rendered" "document.querySelector('[data-testid=\"skill-drawer\"] .prose')?.textContent.includes('version one') === true" "$T" 30
check "still on the skills page" "document.querySelector('[data-testid=\"skills-page\"]') !== null && document.querySelector('[data-testid=\"skill-drawer\"]').closest('[data-testid=\"skills-page\"]') !== null" "$T"
check "list stays scrollable behind the drawer" "(() => { const el=document.querySelector('.skills-page .kb-content'); el.scrollTop=0; el.scrollTop=100; const moved=el.scrollTop > 0; el.scrollTop=0; return el.scrollHeight > el.clientHeight && moved; })()" "$T"
# AC-UX-2: another card swaps the content in place; Esc closes and returns focus
click '[data-testid="kb-card-vue"]' >/dev/null
wait_match "drawer switched" "document.querySelector('[data-testid=\"skill-drawer-path\"]')?.textContent === 'skills/前端/vue/SKILL.md' && document.querySelector('[data-testid=\"skill-drawer\"]').getAttribute('data-state') === 'ready'" "$T" 30
check "distribution lists the tool" "document.querySelector('[data-testid=\"skill-drawer-dist\"]').textContent.includes('Claude Code')" "$T"
esc
wait_match "Esc closes the drawer" "document.querySelector('[data-testid=\"skill-drawer\"]') === null" "$T" 20
check "focus is back on the second card" "document.activeElement?.getAttribute('data-testid') === 'kb-card-vue'" "$T"
# AC-UX-3: edit in the workbench
click '[data-testid="kb-card-vue"]' >/dev/null
wait_match "drawer for vue" "document.querySelector('[data-testid=\"skill-drawer-edit\"]') !== null" "$T" 20
click '[data-testid="skill-drawer-edit"]' >/dev/null
wait_match "workbench shows" "document.querySelector('[data-testid=\"workbench-host\"]')?.hasAttribute('hidden') === false" "$T" 30
wait_match "SKILL.md opened" "window.__mkbDocs.getState().activePath === 'skills/前端/vue/SKILL.md'" "$T" 30
show_skills
# AC-UX-4: the file is gone before the drawer opens. The index notices a
# deletion within a moment and the card leaves the list, so the "card still
# listed, file already gone" window cannot be hit by timing. Freeze the page's
# rescans (store.refresh / loadScan become no-ops) for this step instead: the
# card stays, the file is really gone, and the drawer has to cope.
w "$VAULT/skills/前端/racer/SKILL.md" "---\nname: racer\ndescription: 抽屉缺失态用\n---\n\n# racer\n"
wait_match "racer card" "$(card racer) !== null" "$T" 60
ev "(() => { const s=window.__mkbSkills; window.__origScan={refresh:s.getState().refresh, loadScan:s.getState().loadScan}; s.setState({refresh: async () => {}, loadScan: async () => {}}); return true; })()" >/dev/null
rm -rf "$VAULT/skills/前端/racer"
sleep 2
check "card is still listed while rescans are frozen" "$(card racer) !== null" "$T"
click '[data-testid="kb-card-racer"]' >/dev/null
wait_match "drawer says the file is missing" "document.querySelector('[data-testid=\"skill-drawer\"]')?.getAttribute('data-state') === 'missing'" "$T" 30
check "missing text and rescan button, no raw error" "(() => { const d=document.querySelector('[data-testid=\"skill-drawer\"]'); return d.textContent.includes('找不到这个技能文件') && !!document.querySelector('[data-testid=\"skill-drawer-rescan\"]') && !d.querySelector('[data-testid=\"skill-drawer-error\"]'); })()" "$T"
ev "(() => { const s=window.__mkbSkills; s.setState(window.__origScan); return true; })()" >/dev/null
click '[data-testid="skill-drawer-rescan"]' >/dev/null
wait_match "rescan drops the deleted skill from the list" "$(card racer) === null" "$T" 40
esc
wait_match "drawer closed" "document.querySelector('[data-testid=\"skill-drawer\"]') === null" "$T" 20

echo "== AC-UX-9: nav filter"
N0=$(eval_value "[...document.querySelectorAll('[data-testid=\"skills-nav\"] .skills-nav-item')].filter((e) => !e.closest('[data-collapsed]')).length")
fill skills-nav-filter "claude"
wait_match "filter keeps only matches" "(() => { const items=[...document.querySelectorAll('[data-testid=\"skills-nav\"] .skills-nav-item')].filter((e) => !e.closest('[data-collapsed]')); return items.length >= 1 && items.every((e) => (e.title || '').toLowerCase().includes('claude')); })()" "$T" 20
check "group heads show hits / total" "/\\d+ \\/ \\d+/.test(document.querySelector('[data-testid=\"skills-nav-head-tools\"]').textContent) && /\\d+ \\/ \\d+/.test(document.querySelector('[data-testid=\"skills-nav-head-kb\"]').textContent)" "$T"
N1=$(eval_value "[...document.querySelectorAll('[data-testid=\"skills-nav\"] .skills-nav-item')].filter((e) => !e.closest('[data-collapsed]')).length")
if [ "${N1:-0}" -lt "${N0:-0}" ] 2>/dev/null; then pass=$((pass+1)); echo "PASS fewer items while filtering ($N0 -> $N1)"; else fail=$((fail+1)); echo "FAIL filter did not narrow ($N0 -> $N1)"; fi
click 'button[aria-label="清除筛选"]' >/dev/null
wait_match "clearing restores the list" "document.querySelector('[data-testid=\"skills-nav-filter\"]').value === ''" "$T" 20
N2=$(eval_value "[...document.querySelectorAll('[data-testid=\"skills-nav\"] .skills-nav-item')].filter((e) => !e.closest('[data-collapsed]')).length")
if [ "$N2" = "$N0" ]; then pass=$((pass+1)); echo "PASS list restored ($N2)"; else fail=$((fail+1)); echo "FAIL list not restored ($N0 -> $N2)"; fi

echo "== AC-UX-10: collapse a group, switch tools, keep it across a restart"
click '[data-testid="skills-nav-toggle-kb"]' >/dev/null
wait_match "knowledge-base group folded" "document.querySelector('[data-testid=\"skills-nav-items-kb\"]')?.hasAttribute('data-collapsed') === true && document.querySelector('[data-testid=\"skills-nav-toggle-kb\"]').getAttribute('aria-expanded') === 'false'" "$T" 20
check "group head sticks to the top of the list" "getComputedStyle(document.querySelector('[data-testid=\"skills-nav-head-kb\"]')).position === 'sticky'" "$T"
click '[data-testid="skills-nav"] button[title="全部工具"]' >/dev/null
wait_match "tool view switches while folded" "document.querySelector('.skills-page .tool-toolbar, .skills-page .tool-skills') !== null && document.querySelector('[data-testid=\"skills-nav-items-kb\"]').hasAttribute('data-collapsed')" "$T" 30
wait_match "setting saved" "window.__mkbApp.getState().settings?.layout?.skillsKbCollapsed === true" "$T" 20
sleep 1
grep -q '"skillsKbCollapsed": true' "$APPD/Mile Knowledge Base/settings.json"; check_file "settings.json holds the fold" "$?"
stop_app

# ===== run 2: restart, reduced motion, narrow window ========================
echo "== AC-UX-10 (restart) and AC-UX-12: reduced motion"
start_app WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--force-prefers-reduced-motion
wait_match "overview after restart" "document.querySelector('[data-testid=\"overview-grid\"]') !== null" "$T" 90
show_skills
wait_match "nav rendered" "document.querySelector('[data-testid=\"skills-nav-toggle-kb\"]') !== null" "$T" 30
check "fold survived the restart" "document.querySelector('[data-testid=\"skills-nav-items-kb\"]').hasAttribute('data-collapsed') && document.querySelector('[data-testid=\"skills-nav-toggle-kb\"]').getAttribute('aria-expanded') === 'false'" "$T"
check "tools group still open" "document.querySelector('[data-testid=\"skills-nav-items-tools\"]').hasAttribute('data-collapsed') === false" "$T"
wait_match "motion query" "matchMedia('(prefers-reduced-motion: reduce)').matches === true" "$T" 20
click '[data-testid="skills-nav-toggle-kb"]' >/dev/null
wait_match "expand again" "document.querySelector('[data-testid=\"skills-nav-items-kb\"]').hasAttribute('data-collapsed') === false" "$T" 20
wait_match "cards after restart" "$(card vue) !== null" "$T" 40
click '[data-testid="kb-card-vue"]' >/dev/null
wait_match "drawer opens" "document.querySelector('[data-testid=\"skill-drawer\"]')?.getAttribute('data-state') === 'ready'" "$T" 30
check "drawer has no real slide-in" "parseFloat(getComputedStyle(document.querySelector('[data-testid=\"skill-drawer\"]')).animationDuration) <= 0.001 && getComputedStyle(document.querySelector('[data-testid=\"skill-drawer\"]')).opacity === '1'" "$T"
esc
wait_match "drawer closed" "document.querySelector('[data-testid=\"skill-drawer\"]') === null" "$T" 20
click '[data-testid="kb-install-top"]' >/dev/null
wait_match "install dialog" "document.querySelector('[data-testid=\"skills-modal\"]') !== null" "$T" 30
check "dialog has no real scale-in" "parseFloat(getComputedStyle(document.querySelector('.skills-modal')).animationDuration) <= 0.001 && parseFloat(getComputedStyle(document.querySelector('[data-testid=\"skills-modal\"]')).animationDuration) <= 0.001 && getComputedStyle(document.querySelector('.skills-modal')).opacity === '1'" "$T"
esc
wait_match "dialog closed" "document.querySelector('[data-testid=\"skills-modal\"]') === null" "$T" 20

echo "== AC-UX-11: 744x640"
foreground_app
resize_window 744 640
sleep 0.6
check "narrow viewport" "window.innerWidth <= 800 && window.innerHeight <= 700" "$T"
echo "width after resize: $(eval_value 'window.innerWidth')x$(eval_value 'window.innerHeight')"
check "nav is an icon rail" "Math.round(document.querySelector('[data-testid=\"skills-page\"] .skills-nav').getBoundingClientRect().width) <= 60" "$T"
check "no horizontal scroll" "document.documentElement.scrollWidth <= window.innerWidth" "$T"
click '[data-testid="skills-nav-toggle-kb"]' >/dev/null
echo "width after the fold click: $(eval_value 'window.innerWidth')"
wait_match "folded group still shows its items in the rail" "getComputedStyle(document.querySelector('[data-testid=\"skills-nav-items-kb\"]')).display !== 'none' && document.querySelector('[data-testid=\"skills-nav-items-kb\"]').querySelector('.skills-nav-item').getBoundingClientRect().height > 0" "$T" 20 || diag "(() => { const g=document.querySelector('[data-testid=skills-nav-items-kb]'); return JSON.stringify({collapsed:g.hasAttribute('data-collapsed'), display:getComputedStyle(g).display, w:window.innerWidth, items:g.querySelectorAll('.skills-nav-item').length, h:g.querySelector('.skills-nav-item')?.getBoundingClientRect().height, exp:document.querySelector('[data-testid=skills-nav-toggle-kb]').getAttribute('aria-expanded')}); })()"
click '[data-testid="skills-nav-toggle-kb"]' >/dev/null
click '[data-testid="kb-card-vue"]' >/dev/null
wait_match "drawer fits" "$(within_viewport '[data-testid="skill-drawer"]')" "$T" 30
check "no horizontal scroll with the drawer" "document.documentElement.scrollWidth <= window.innerWidth" "$T"
esc
wait_match "drawer closed" "document.querySelector('[data-testid=\"skill-drawer\"]') === null" "$T" 20
click '[data-testid="kb-install-top"]' >/dev/null
wait_match "install dialog fits" "$(within_viewport '.skills-modal')" "$T" 30
check "no horizontal scroll with the install dialog" "document.documentElement.scrollWidth <= window.innerWidth" "$T"
esc
wait_match "install dialog closed" "document.querySelector('[data-testid=\"skills-modal\"]') === null" "$T" 20
open_card_button react "分发"
wait_match "distribute dialog fits" "$(within_viewport '.skills-modal')" "$T" 30
check "no horizontal scroll with the distribute dialog" "document.documentElement.scrollWidth <= window.innerWidth" "$T"
esc
wait_match "distribute dialog closed" "document.querySelector('[data-testid=\"skills-modal\"]') === null" "$T" 20
stop_app

echo "RESULT pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
