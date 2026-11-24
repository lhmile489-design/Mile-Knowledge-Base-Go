#!/usr/bin/env python3
"""Capture the M5 install/update states (T13).

Runs the e2e build against a temporary vault, a fake HOME, a local fake GitHub
and a fixed zip (SKILLS_FAKE_* switches, e2e builds only), drives the seven
states in light and dark, and writes them into design/r2/shots-m5/ (replacing
the prototype shots; the prototypes stay in git history). Also prints the
contrast ratios of the new pills and notes.

Usage (from the repo root, with no other Mile Knowledge Base instance):
  python -I scripts/visual/capture-m5.py
"""

import hashlib
import importlib.util
import io
import json
import os
import socket
import subprocess
import sys
import tarfile
import tempfile
import time
import zipfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
spec = importlib.util.spec_from_file_location("capture_common", HERE / "capture-new.py")
cap = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(cap)

from PIL import Image as PILImage  # noqa: E402

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

SHOTS = ROOT / "design" / "r2" / "shots-m5"
SHOTS.mkdir(parents=True, exist_ok=True)
GH_PORT = 18433
PORT = cap.PORT


def log(message):
    print(message, flush=True)


def write_bytes(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data)


def write_text(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(data, encoding="utf-8", newline="\n")


def tree_hash(root):
    """Mirror skills.TreeHash on Windows: backslash-relative paths, sorted."""
    digest = hashlib.sha256()
    for base, _, files in sorted(os.walk(root)):
        for name in sorted(files):
            path = Path(base) / name
            rel = os.path.relpath(path, root)
            digest.update(rel.encode("utf-8"))
            digest.update(b"\0")
            digest.update(path.read_bytes())
            digest.update(b"\0")
    return "sha256:" + digest.hexdigest()


def tar_gz(path, files, evil=False):
    with tarfile.open(path, "w:gz") as archive:
        for rel, data in files.items():
            info = tarfile.TarInfo("repo/" + rel)
            info.size = len(data)
            archive.addfile(info, io.BytesIO(data))
        if evil:
            data = b"evil\n"
            info = tarfile.TarInfo("../evil/SKILL.md")
            info.size = len(data)
            archive.addfile(info, io.BytesIO(data))
    return path


def wait_port(port, timeout=20.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            with socket.create_connection(("127.0.0.1", port), 0.3):
                return
        except OSError:
            time.sleep(0.1)
    raise RuntimeError(f"port {port} never opened")


def main():
    if not cap.APP.exists():
        log(f"missing e2e build: {cap.APP}\nrun: GOFLAGS=-tags=e2e MYGO_INSPECTOR=1 bun run build")
        return 2

    appdata = cap.canon(tempfile.mkdtemp(prefix="mkb-m5-appdata-"))
    vault = cap.build_vault()
    vid = cap.vault_id_of(vault)
    cap.write_settings(appdata, "light", vault, vid)
    data_dir = Path(appdata) / "Mile Knowledge Base"

    home = cap.canon(tempfile.mkdtemp(prefix="mkb-m5-home-"))
    write_text(Path(home) / ".local" / "bin" / "claude.cmd", "@echo off")
    write_text(Path(home) / ".local" / "bin" / "cursor.cmd", "@echo off")
    write_text(Path(home) / ".claude" / "skills" / "other" / "SKILL.md", "# other\n")
    registry = Path(home) / "registry.json"
    write_text(registry, json.dumps({"uninstall": []}))

    # --- local v1 skill and its tool copy -----------------------------------
    v1 = {
        "SKILL.md": b"# pdf\n\nversion one\n",
        "reference/old.md": b"old\n",
        "assets/icon.bin": bytes([255, 1]),
    }
    kb_skill = Path(vault) / "skills" / "文档" / "pdf"
    tool_skill = Path(home) / ".claude" / "skills" / "pdf"
    for rel, data in v1.items():
        write_bytes(kb_skill / rel, data)
        write_bytes(tool_skill / rel, data)
    base_hash = tree_hash(kb_skill)
    now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    commit_old = "1" * 40
    commit_new = "2" * 40

    write_text(
        Path(vault) / ".mile-kb" / "skill-sources.json",
        json.dumps({
            "version": 1,
            "records": [{
                "skillPath": "skills/文档/pdf",
                "owner": "acme", "repo": "skills", "ref": "main", "path": "skills/pdf",
                "commit": commit_old, "pathCommit": commit_old,
                "baseHash": base_hash, "installedAt": now,
            }],
        }, ensure_ascii=False, indent=2) + "\n",
    )
    write_text(
        data_dir / "skills" / "sources.json",
        json.dumps({
            "version": 1,
            "records": [{
                "id": "g-aaaaaaaaaaaa", "toolId": "claude-code",
                "targetPath": str(tool_skill),
                "owner": "acme", "repo": "skills", "ref": "main", "path": "skills/pdf",
                "commit": commit_old, "pathCommit": commit_old,
                "baseHash": base_hash, "installedAt": now,
            }],
        }, ensure_ascii=False, indent=2) + "\n",
    )
    write_text(
        data_dir / "skills" / "update-cache.json",
        json.dumps({
            "version": 1,
            "checkedAt": now,
            "results": [{
                "owner": "acme", "repo": "skills", "ref": "main", "path": "skills/pdf",
                "latestPathCommit": commit_new, "checkedAt": now,
            }],
        }, ensure_ascii=False, indent=2) + "\n",
    )

    # --- fake GitHub and zip fixtures ---------------------------------------
    fix = Path(tempfile.mkdtemp(prefix="mkb-m5-fix-"))
    demo = tar_gz(fix / "demo.tar.gz", {
        "skills/alpha/SKILL.md": b"# alpha\n\nAlpha skill.\n",
        "skills/beta/SKILL.md": b"# beta\n\nBeta skill.\n",
    })
    update = tar_gz(fix / "update.tar.gz", {
        "skills/pdf/SKILL.md": b"# pdf\n\nversion two\n",
        "skills/pdf/reference/new.md": b"new\n",
        "skills/pdf/assets/icon.bin": bytes([255, 2]),
    })
    evil = tar_gz(fix / "evil.tar.gz", {"SKILL.md": b"# evil\n"}, evil=True)
    zip_path = fix / "demo.zip"
    with zipfile.ZipFile(zip_path, "w") as archive:
        archive.writestr("skills/gamma/SKILL.md", "# gamma\n\nGamma skill.\n")
    state = fix / "state.json"
    write_text(state, json.dumps({"repos": {
        "acme/demo": {"ref": "main", "head": "d" * 40, "paths": {"": "d" * 40}, "archives": {"d" * 40: str(demo)}},
        "acme/skills": {"ref": "main", "head": commit_new, "paths": {"skills/pdf": commit_new}, "archives": {commit_new: str(update)}},
        "acme/evil": {"ref": "main", "head": "e" * 40, "paths": {"": "e" * 40}, "archives": {"e" * 40: str(evil)}},
    }}))

    server = subprocess.Popen(
        [sys.executable, "-I", str(ROOT / "scripts" / "e2e" / "fake-github.py"), str(state), str(GH_PORT)],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    wait_port(GH_PORT)
    provider = subprocess.Popen(
        [sys.executable, "-I", str(ROOT / "scripts" / "e2e" / "fake-openai.py"), "--port", str(cap.PROVIDER_PORT)],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    env = {
        **os.environ,
        "APPDATA": appdata,
        "MYGO_AUTOMATION": f"tcp:127.0.0.1:{PORT}",
        "SKILLS_FAKE_HOME": home,
        "SKILLS_FAKE_PATH": str(Path(home) / ".local" / "bin"),
        "SKILLS_FAKE_REGISTRY": str(registry),
        "SKILLS_FAKE_GITHUB": f"http://127.0.0.1:{GH_PORT}",
        "SKILLS_FAKE_ZIP": str(zip_path),
    }
    app = subprocess.Popen([str(cap.APP)], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        driver = cap.Driver(PORT)
        driver.connect(60)
        driver.wait("!!document.querySelector('[data-testid=\"overview-grid\"]')", 90)
        try:
            driver.wait("!document.body.innerText.includes('未索引')", 90)
        except RuntimeError:
            log("index still running; continuing")
        time.sleep(1.5)

        def click(selector):
            driver.js(cap.js_click(selector))

        def click_text(scope, text):
            driver.js(
                "(() => { const scope=document.querySelector(%s); if(!scope) throw new Error('missing scope');"
                " const b=[...scope.querySelectorAll('button')].find((x)=>x.textContent.includes(%s));"
                " if(!b) throw new Error('missing button'); b.click(); return true; })()"
                % (json.dumps(scope), json.dumps(text))
            )

        def shot(name):
            image = PILImage.open(io.BytesIO(driver.capture())).convert("RGB").resize((1440, 900), PILImage.LANCZOS)
            image.save(SHOTS / f"{name}.png")
            log(f"  [captured] {name}.png")

        CONTRAST_JS = """(() => {
          const parse = (s) => {
            const mixed = s.match(/color\\(srgb\\s+([\\d.]+)\\s+([\\d.]+)\\s+([\\d.]+)/);
            if (mixed) return [mixed[1], mixed[2], mixed[3]].map((v) => Number(v) * 255);
            return (s.match(/[\\d.]+/g) || []).slice(0, 3).map(Number);
          };
          const lum = (c) => { const f = (v) => { v /= 255; return v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); }; return 0.2126*f(c[0])+0.7152*f(c[1])+0.0722*f(c[2]); };
          const ratio = (a, b) => { const x = lum(parse(a)), y = lum(parse(b)); const hi = Math.max(x, y), lo = Math.min(x, y); return +((hi + 0.05) / (lo + 0.05)).toFixed(2); };
          const bgOf = (el) => { let n = el; while (n && n !== document.documentElement) { const bg = getComputedStyle(n).backgroundColor; if (bg && bg !== 'transparent' && bg !== 'rgba(0, 0, 0, 0)') return bg; n = n.parentElement; } return getComputedStyle(document.body).backgroundColor; };
          const out = {};
          const pill = document.querySelector('[data-testid="kb-card-pdf"] .pill[data-tone="warn"]');
          if (pill) out.updatePill = ratio(getComputedStyle(pill).color, bgOf(pill.closest('.kb-card') || pill.parentElement));
          const src = document.querySelector('.kb-card-src');
          if (src) out.sourceLine = ratio(getComputedStyle(src).color, bgOf(src.closest('.kb-card') || src));
          const note = document.querySelector('.install-note span');
          if (note) out.noteText = ratio(getComputedStyle(note).color, bgOf(note));
          const errorText = document.querySelector('.install-note-body p');
          if (errorText) out.errorText = ratio(getComputedStyle(errorText).color, bgOf(errorText));
          const errorTitle = document.querySelector('.install-note-body b');
          if (errorTitle) out.errorTitle = ratio(getComputedStyle(errorTitle).color, bgOf(errorTitle));
          const row = document.querySelector('.update-diff-row code');
          if (row) out.diffPath = ratio(getComputedStyle(row).color, bgOf(row));
          return JSON.stringify(out);
        })()"""

        def states(theme):
            driver.js("window.__mkbApp.getState().showPage('skills'); true")
            driver.wait("!!document.querySelector('[data-testid=\"skills-page\"]')", 20)
            time.sleep(0.4)
            # The panel remembers its last tab; both passes start on the KB page.
            click_text('[data-testid="skills-nav"]', "全部技能")
            time.sleep(0.4)
            driver.js("document.querySelector('[data-testid=\"kb-install-top\"]').click(); true")
            driver.wait("!!document.querySelector('[data-testid=\"install-github\"]')", 20)

            click('[data-testid="install-github"]')
            driver.wait("!!document.querySelector('[data-testid=\"install-github-form\"]')", 10)
            driver.js(cap.js_set_value('[data-testid="install-github-input"]', "acme/demo", "input"))
            click('[data-testid="install-github-read"]')
            driver.wait(
                "!!document.querySelector('[data-testid=\"install-candidate-skills/alpha\"]')"
                " && !!document.querySelector('[data-testid=\"install-candidate-skills/beta\"]')",
                60,
            )
            driver.js(cap.js_set_value('[data-testid="install-category"]', "文档", "input"))
            time.sleep(0.6)
            shot(f"install-github-{theme}")

            click('[data-testid="install-target-tools"]')
            driver.wait("!!document.querySelector('[data-testid=\"install-tool-claude-code\"]')", 20)
            click('[data-testid="install-tool-claude-code"]')
            time.sleep(1.4)
            shot(f"install-tools-{theme}")

            click_text('.install-foot', "取消")
            driver.wait("!!document.querySelector('[data-testid=\"install-github\"]')", 10)
            click('[data-testid="install-zip"]')
            driver.wait(
                "!!document.querySelector('[data-testid=\"install-stage\"]')"
                " && document.body.innerText.includes('demo.zip')",
                40,
            )
            time.sleep(0.8)
            shot(f"install-zip-{theme}")

            click('[data-testid="install-stage-back"]')
            driver.wait("!!document.querySelector('[data-testid=\"install-github\"]')", 10)
            click('[data-testid="install-github"]')
            driver.wait("!!document.querySelector('[data-testid=\"install-github-form\"]')", 10)
            driver.js(cap.js_set_value('[data-testid="install-github-input"]', "acme/evil", "input"))
            click('[data-testid="install-github-read"]')
            driver.wait("!!document.querySelector('[data-testid=\"install-error\"]')", 60)
            time.sleep(0.6)
            log(f"  contrast install-error {theme}: " + str(driver.js(CONTRAST_JS)))
            shot(f"install-error-{theme}")

            click('[aria-label="返回技能列表"]')
            driver.wait("!!document.querySelector('[data-testid=\"kb-card-pdf\"]')", 30)
            driver.js(
                "document.querySelector('[data-testid=\"kb-card-pdf\"]')?.scrollIntoView({ block: 'center' }); true"
            )
            time.sleep(0.9)
            log(f"  contrast {theme}: " + str(driver.js(CONTRAST_JS)))
            shot(f"kb-update-{theme}")

            click('[data-testid="kb-update-pdf"]')
            driver.wait("!!document.querySelector('[data-testid=\"update-files\"]')", 60)
            time.sleep(0.8)
            log(f"  contrast update-preview {theme}: " + str(driver.js(CONTRAST_JS)))
            shot(f"update-preview-{theme}")
            click('[data-testid="update-cancel"]')
            time.sleep(0.4)

            click_text('[data-testid="skills-nav"]', "全部工具")
            driver.wait("!!document.querySelector('[data-testid=\"tool-skill-pdf-claude-code\"]')", 20)
            click('[data-testid="tool-update-pdf-claude-code"]')
            driver.wait("!!document.querySelector('[data-testid=\"tool-update-dialog\"]')", 10)
            driver.wait(
                "document.querySelector('[data-testid=\"tool-update-dialog\"]').innerText.includes('原目录先备份')",
                60,
            )
            time.sleep(0.8)
            shot(f"tools-update-{theme}")
            click_text('[data-testid="tool-update-dialog"]', "取消")
            time.sleep(0.4)
            driver.js("window.__mkbApp.getState().closeSkillsPanel(); true")
            time.sleep(0.4)

        states("light")
        driver.js("window.__mkbApp.getState().setTheme('dark'); true")
        time.sleep(1.0)
        states("dark")
        return 0
    finally:
        try:
            app.terminate()
            app.wait(timeout=8)
        except Exception:  # noqa: BLE001
            subprocess.run(["taskkill", "/F", "/PID", str(app.pid), "/T"], capture_output=True)
        server.terminate()
        provider.terminate()


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:  # noqa: BLE001
        import traceback
        traceback.print_exc()
        sys.exit(1)
