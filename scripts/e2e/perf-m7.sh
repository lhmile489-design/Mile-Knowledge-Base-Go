#!/usr/bin/env bash
# M7 T09: restore latency on a real window (技术设计 18.9). One app session:
#   1. preview 200 manifest items (100 kb -> Claude Code, 50 kb -> ten
#      registered projects, 50 GitHub) ten times -> preview200P95Ms
#   2. restore 100 knowledge-base items in one call -> restore100Ms
#   3. rebuild the manifest from 2000 local records ten times ->
#      manifestWriteP95Ms
# Writes metrics.json and runs the G-PERF gate (kind=m7).
set -u
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/windows-amd64/Mile Knowledge Base.exe"
WORK=$(mktemp -d "$ROOT/.perf-m7-XXXXXX")
VAULT="$WORK/vault"; APPD="$WORK/app"; HOME_DIR="$WORK/home"
mkdir -p "$APPD/Mile Knowledge Base" "$HOME_DIR/.local/bin" "$HOME_DIR/.claude/skills"
printf '@echo off' > "$HOME_DIR/.local/bin/claude.cmd"
printf '{"uninstall":[]}' > "$WORK/registry.json"
mkdir -p "$WORK/extra"

python -I - "$WORK" "$VAULT" <<'PY'
import hashlib, json, os, sys
work, vault = sys.argv[1], sys.argv[2]

# 100 vault skills.
for i in range(100):
    d = os.path.join(vault, "skills", "cat-%02d" % (i % 10), "skill-%03d" % i)
    os.makedirs(d, exist_ok=True)
    open(os.path.join(d, "SKILL.md"), "w", encoding="utf-8").write("# s%03d\n\n" % i)

# Ten registered projects, each with the conventional subdirectory.
rels = [".agents/skills", ".claude/skills", ".kiro/skills", ".cline/skills", ".warp/skills"]
targets = []
for p in range(10):
    root = os.path.join(work, "projects", "proj-%02d" % p)
    for rel in rels:
        os.makedirs(os.path.join(root, rel.replace("/", os.sep)), exist_ok=True)
    targets.append({
        "id": "t-%012x" % (p + 1), "kind": "project", "name": "proj-%02d" % p,
        "path": root, "installRel": ".claude/skills",
        "createdAt": "2026-11-23T09:%02d:00Z" % p,
    })
json.dump({"version": 1, "targets": targets},
          open(os.path.join(work, "targets.json"), "w", encoding="utf-8"), ensure_ascii=False, indent=2)

sha = "1" * 40
stamp = "2026-11-23T09:00:00Z"
items = []
for i in range(100):
    items.append({"source": "kb", "target": {"kind": "builtin", "toolId": "claude-code"},
                  "name": "skill-%03d" % i, "skillPath": "skills/cat-%02d/skill-%03d" % (i % 10, i),
                  "mode": "link", "updatedAt": stamp})
for i in range(50):
    items.append({"source": "kb", "target": {"kind": "project", "name": "proj-%02d" % (i % 10), "subdir": ".claude/skills"},
                  "name": "p-skill-%03d" % i, "skillPath": "skills/cat-%02d/skill-%03d" % (i % 10, i),
                  "mode": "copy", "updatedAt": stamp})
for i in range(50):
    items.append({"source": "github", "target": {"kind": "builtin", "toolId": "claude-code"},
                  "name": "gh-%03d" % i, "owner": "anthropics", "repo": "skills", "ref": "main",
                  "path": "skills/gh-%03d" % i, "commit": sha, "pathCommit": sha, "updatedAt": stamp})
os.makedirs(os.path.join(vault, ".mile-kb"), exist_ok=True)
json.dump({"version": 1, "updatedAt": stamp, "items": items},
          open(os.path.join(vault, ".mile-kb", "skill-distributions.json"), "w", encoding="utf-8"),
          ensure_ascii=False, indent=2)
print("fixtures ok")
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
       "recentVaults": [{"id": vid, "root": vault, "displayName": "perf-m7", "lastOpenedAt": "2026-11-23T09:00:00Z"}],
       "lastVaultId": vid}
open(os.path.join(appdir, "settings.json"), "w", encoding="utf-8").write(json.dumps(doc, ensure_ascii=False, indent=2) + "\n")
print(vid)
PY
)
TARGETS_FILE="$APPD/Mile Knowledge Base/skills/targets.json"
mkdir -p "$(dirname "$TARGETS_FILE")"
cp "$WORK/targets.json" "$TARGETS_FILE"

PORT=18547
APP_PID=""
cleanup() {
  if [ -n "${APP_PID:-}" ]; then powershell -NoProfile -Command "Stop-Process -Id $APP_PID -Force" >/dev/null 2>&1; fi
  sleep 2
  if [ "${PERF_KEEP:-0}" != "1" ]; then
    rm -rf "$WORK"
  fi
}
trap cleanup EXIT
if tasklist //FI "IMAGENAME eq Mile Knowledge Base.exe" 2>/dev/null | grep -q "Mile Knowledge"; then
  echo "ABORT: Mile Knowledge Base is already running"; exit 2
fi

APPDATA="$(cygpath -w "$APPD")" MYGO_AUTOMATION="tcp:127.0.0.1:$PORT" \
  SKILLS_FAKE_HOME="$(cygpath -w "$HOME_DIR")" \
  SKILLS_FAKE_PATH="$(cygpath -w "$HOME_DIR/.local/bin")" \
  SKILLS_FAKE_REGISTRY="$(cygpath -w "$WORK/registry.json")" \
  SKILLS_FAKE_PICK_DIR="$(cygpath -w "$WORK/extra")" \
  "$APP" >"$WORK/app.log" 2>&1 &
sleep 7
APP_PID=$(powershell -NoProfile -Command "(Get-Process -Name 'Mile Knowledge Base' | Sort-Object StartTime -Descending | Select-Object -First 1).Id" | tr -d '\r')

python -I - "$PORT" "$VID" "$(cygpath -w "$WORK")" "$(cygpath -w "$VAULT")" "$(cygpath -w "$APPD/Mile Knowledge Base")" "$(cygpath -w "$HOME_DIR")" "$(cygpath -w "$WORK/metrics.json")" <<'PY'
import json, os, socket, sys, time

port, vid, work, vault, data_dir, home_dir, out_path = (
    int(sys.argv[1]), sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5], sys.argv[6], sys.argv[7])

def call(method, **params):
    with socket.create_connection(("127.0.0.1", port), timeout=60) as s:
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
        time.sleep(0.5)
    raise SystemExit("wait timeout: " + script[:80])

wait("!!window.__mkbApi && !!window.__mkbApp.getState().settings")
wait("document.querySelector('[data-testid=\"overview-grid\"]') !== null")

def p95(values):
    ordered = sorted(values)
    return ordered[max(0, int(round(0.95 * len(ordered))) - 1)]

# 1) Preview 200 items ten times.
preview_ms = []
for _ in range(10):
    t0 = time.perf_counter()
    js("window.__mkbApi.skills.restorePreview(%s)" % json.dumps(vid))
    preview_ms.append((time.perf_counter() - t0) * 1000)

# 2) Restore the 100 knowledge-base items in one call.
PREVIEW = """
(async () => {
  const p = await window.__mkbApi.skills.restorePreview(%s);
  return p.preview.items.filter((i) => i.source === 'kb' && i.target.kind === 'builtin').map((i) => i.id);
})()""" % json.dumps(vid)
ids = js(PREVIEW)
if len(ids) != 100:
    raise SystemExit("expected 100 kb items, got %d" % len(ids))
RESTORE = """
(async () => {
  const ids = %s;
  const t0 = performance.now();
  const r = await window.__mkbApi.skills.restoreManifest(%s, ids.map((id) => ({ id })), 'perf-m7');
  return [performance.now() - t0, r.items.filter((i) => i.result === 'linked' || i.result === 'copied' || i.result === 'installed').length];
})()""" % (json.dumps(ids), json.dumps(vid))
restore_ms, restored = js(RESTORE)

# 3) Manifest write: the net cost of the upsert. Ten distributions with a
#    2000-item manifest minus ten with a 1-item manifest; the difference is
#    the manifest load + validate + sort + write itself.
stamp = "2026-11-23T09:00:00Z"

REG = """
(async () => {
  const api = window.__mkbApi.skills;
  const pick = await api.pickTargetFolder();
  if (pick.cancelled) throw new Error('pick cancelled');
  const saved = await api.saveTarget({ kind: 'custom', name: 'Perf Tool', token: pick.token });
  return saved.target.id;
})()"""
CUSTOM_ID = js(REG)

manifest_file = os.path.join(vault, ".mile-kb", "skill-distributions.json")


def seed_manifest(count):
    items = []
    for i in range(count):
        items.append({"source": "kb", "target": {"kind": "builtin", "toolId": "claude-code"},
                      "name": "big-%04d" % i, "skillPath": "skills/cat-00/skill-000",
                      "mode": "copy", "updatedAt": stamp})
    json.dump({"version": 1, "updatedAt": stamp, "items": items},
              open(manifest_file, "w", encoding="utf-8"), ensure_ascii=False)


def distribute_round(name):
    skill_dir = os.path.join(vault, "skills", "perf", name)
    os.makedirs(skill_dir, exist_ok=True)
    open(os.path.join(skill_dir, "SKILL.md"), "w", encoding="utf-8").write("# r" + chr(10))
    t0 = time.perf_counter()
    js("window.__mkbApi.skills.distribute(%s, %s, [{targetId: %s}], 'copy')"
       % (json.dumps(vid), json.dumps("skills/perf/" + name), json.dumps(CUSTOM_ID)))
    return (time.perf_counter() - t0) * 1000


seed_manifest(1)
base_ms = [distribute_round("base-%02d" % i) for i in range(10)]
seed_manifest(2000)
big_ms = [distribute_round("big-%02d" % i) for i in range(10)]
write_ms = [max(0.0, big - base) for big, base in zip(big_ms, base_ms)]
print("base_distribute_ms:", " ".join("%.1f" % v for v in base_ms))
print("big_distribute_ms: ", " ".join("%.1f" % v for v in big_ms))
print("write_delta_ms:    ", " ".join("%.1f" % v for v in write_ms))

metrics = {
    "preview200P95Ms": round(p95(preview_ms), 1),
    "manifestWriteP95Ms": round(p95(write_ms), 1),
    "restore100Ms": round(restore_ms, 1),
}
json.dump(metrics, open(out_path, "w", encoding="utf-8"))
print("preview_ms:", " ".join("%.0f" % v for v in preview_ms))
print("restore_ms:", round(restore_ms, 1), "restored:", restored)
print("metrics:", json.dumps(metrics))
PY

python -I "$ROOT/scripts/perf-gate.py" --kind m7 --input "$(cygpath -w "$WORK/metrics.json")"
STATUS=$?
stop_pid=$APP_PID
powershell -NoProfile -Command "Stop-Process -Id $stop_pid -Force" >/dev/null 2>&1
APP_PID=""
exit $STATUS
