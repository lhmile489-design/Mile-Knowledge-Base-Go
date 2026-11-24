#!/usr/bin/env python3
"""Capture the M7 restore states from the real window (T08).

Runs the e2e build against a temporary vault, a fake HOME and a slow local
fake GitHub (SKILLS_FAKE_GITHUB, e2e builds only), drives the five restore
states in light and dark, and writes them into design/r2/shots-m7/ (replacing
the prototypes; the prototypes stay in git history). Also prints the contrast
ratios of the new text.

Usage (from the repo root, with no other Mile Knowledge Base instance):
  python -I scripts/visual/capture-m7.py
"""

import importlib.util
import io
import json
import os
import shutil
import subprocess
import sys
import tarfile
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
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

SHOTS = ROOT / "design" / "r2" / "shots-m7"
SHOTS.mkdir(parents=True, exist_ok=True)
PORT = cap.PORT
SHA = "1" * 40


def log(message):
    print(message, flush=True)


def write_text(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(data, encoding="utf-8", newline="\n")


def start_slow_github(archive, delay):
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):  # noqa: N802
            if "/tar.gz/" in self.path:
                time.sleep(delay)
                self.send_response(200)
                self.send_header("Content-Type", "application/gzip")
                self.send_header("Content-Length", str(len(archive)))
                self.end_headers()
                self.wfile.write(archive)
                return
            self.send_error(404)

        def log_message(self, *_args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server, server.server_address[1]


def tarball():
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode="w:gz") as tar:
        for rel, data in {
            "skills/pdf/SKILL.md": b"# pdf\n\nrestored from github\n",
            "skills/docx/SKILL.md": b"# docx v1\n",
        }.items():
            info = tarfile.TarInfo("repo/" + rel)
            info.size = len(data)
            tar.addfile(info, io.BytesIO(data))
    return buffer.getvalue()


def main():
    if not cap.APP.exists():
        log(f"missing e2e build: {cap.APP}\nrun: GOFLAGS=-tags=e2e MYGO_INSPECTOR=1 bun run build")
        return 2

    appdata = cap.canon(tempfile.mkdtemp(prefix="mkb-m7-appdata-"))
    vault = cap.build_vault()
    vid = cap.vault_id_of(vault)
    cap.write_settings(appdata, "light", vault, vid)
    write_text(Path(vault) / "skills" / "前端" / "vue" / "SKILL.md", "# vue\n\n")
    write_text(Path(vault) / "skills" / "文档" / "docx" / "SKILL.md", "# docx\n\n")

    home = cap.canon(tempfile.mkdtemp(prefix="mkb-m7-home-"))
    write_text(Path(home) / ".local" / "bin" / "claude.cmd", "@echo off")
    write_text(Path(home) / ".claude" / "skills" / "vue" / "SKILL.md", "# local edit\n")
    write_text(Path(home) / ".claude" / "skills" / "docx" / "SKILL.md", "# docx\n")
    registry = Path(home) / "registry.json"
    write_text(registry, json.dumps({"uninstall": []}))

    work = Path(tempfile.mkdtemp(prefix="mkb-m7-fixtures-"))
    project = work / "acme-site"
    (project / ".claude" / "skills").mkdir(parents=True, exist_ok=True)
    queue = work / "pick-queue.txt"
    write_text(queue, str(project) + "\n")

    manifest_path = Path(vault) / ".mile-kb" / "skill-distributions.json"
    state_path = Path(appdata) / "Mile Knowledge Base" / "skills" / "state.json"
    sources_path = Path(appdata) / "Mile Knowledge Base" / "skills" / "sources.json"
    installed = Path(home) / ".claude" / "skills"
    project_link = project / ".claude" / "skills" / "vue"

    def manifest_items():
        return [
            {"source": "github", "target": {"kind": "builtin", "toolId": "claude-code"}, "name": "pdf",
             "owner": "anthropics", "repo": "skills", "ref": "main", "path": "skills/pdf",
             "commit": SHA, "pathCommit": SHA, "updatedAt": "2026-11-23T09:00:00Z"},
            {"source": "kb", "target": {"kind": "project", "name": "acme-site", "subdir": ".claude/skills"},
             "name": "vue", "skillPath": "skills/前端/vue", "mode": "link", "updatedAt": "2026-11-23T09:00:00Z"},
            {"source": "kb", "target": {"kind": "builtin", "toolId": "claude-code"},
             "name": "vue", "skillPath": "skills/前端/vue", "mode": "link", "updatedAt": "2026-11-23T09:00:00Z"},
            {"source": "kb", "target": {"kind": "builtin", "toolId": "claude-code"},
             "name": "docx", "skillPath": "skills/文档/docx", "mode": "copy", "updatedAt": "2026-11-23T09:00:00Z"},
            {"source": "kb", "target": {"kind": "custom", "name": "Droid"},
             "name": "vue", "skillPath": "skills/前端/vue", "mode": "link", "updatedAt": "2026-11-23T09:00:00Z"},
        ]

    def remove_dir(path):
        if not path.exists() and not path.is_symlink():
            return
        subprocess.run(["cmd", "/c", "rmdir", str(path)], capture_output=True, text=True)
        if path.exists() or path.is_symlink():
            shutil.rmtree(path, ignore_errors=True)

    def reset_fixtures():
        remove_dir(installed / "pdf")
        remove_dir(project_link)
        for leftover in (manifest_path.parent).glob("skill-distributions.json.corrupt-*"):
            leftover.unlink()
        if sources_path.exists():
            sources_path.unlink()
        write_text(manifest_path, json.dumps({"version": 1, "updatedAt": "2026-11-23T09:00:00Z", "items": manifest_items()},
                                             ensure_ascii=False, indent=2) + "\n")
        write_text(state_path, json.dumps({"version": 1, "distributions": [{
            "id": "d-111111111111", "vaultId": vid, "skillPath": "skills/文档/docx",
            "toolId": "claude-code", "targetPath": str(installed / "docx"), "mode": "copy",
            "name": "docx", "createdAt": "2026-11-23T09:00:00Z",
            "baseHash": "sha256:" + "a" * 64, "lastVerifiedAt": "2026-11-23T09:00:00Z",
        }]}, ensure_ascii=False, indent=2) + "\n")
        write_text(installed / "vue" / "SKILL.md", "# local edit\n")

    reset_fixtures()
    server, gh_port = start_slow_github(tarball(), 5.0)

    env = {
        **os.environ,
        "APPDATA": appdata,
        "MYGO_AUTOMATION": f"tcp:127.0.0.1:{PORT}",
        "SKILLS_FAKE_HOME": home,
        "SKILLS_FAKE_PATH": str(Path(home) / ".local" / "bin"),
        "SKILLS_FAKE_REGISTRY": str(registry),
        "SKILLS_FAKE_PICK_DIR": "@" + str(queue),
        "SKILLS_FAKE_GITHUB": f"http://127.0.0.1:{gh_port}",
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
        time.sleep(1.0)

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

        def fill(testid, value):
            driver.js(cap.js_set_value(f'[data-testid="{testid}"]', value, "input"))

        def close_panel():
            driver.js("window.__mkbApp.getState().showPage('work'); true")
            time.sleep(0.4)

        def open_tools():
            driver.js("window.__mkbApp.getState().showPage('skills'); true")
            driver.wait("!!document.querySelector('[data-testid=\"skills-page\"]')", 20)
            time.sleep(0.4)
            click_text('[data-testid="skills-nav"]', "全部工具")
            time.sleep(0.6)

        def close_banner():
            driver.js(
                "(() => { const b=document.querySelector('[data-testid=\"skills-banner-close\"]'); if(b) b.click(); return true; })()"
            )
            time.sleep(0.3)

        def register_project_once():
            open_tools()
            click('[data-testid="skills-nav"] button[title="管理目录…"]')
            driver.wait("!!document.querySelector('[data-testid=\"targets-view\"]')", 20)
            click('[data-testid="targets-add-project"]')
            driver.wait("!!document.querySelector('[data-testid=\"target-name\"]')", 20)
            fill("target-name", "acme-site")
            click('[data-testid="target-pick"]')
            driver.wait("!!document.querySelector('[data-testid=\"target-dirs\"]')", 20)
            time.sleep(0.4)
            click('[data-testid="target-install-.claude/skills"]')
            click('[data-testid="target-save"]')
            driver.wait("!document.querySelector('[data-testid=\"target-name\"]')", 40)
            time.sleep(0.8)
            close_banner()
            click('[data-testid="targets-back"]')
            time.sleep(0.4)
            close_panel()

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
          const pair = (key, selector) => { const el = document.querySelector(selector); if (el) out[key] = ratio(getComputedStyle(el).color, bgOf(el)); };
          pair('banner', '.restore-banner-text');
          pair('target', '.restore-target');
          pair('why', '.restore-why');
          pair('source', '.restore-source');
          pair('infoPill', '.pill[data-tone="info"]');
          pair('warnNote', '.restore-note');
          return JSON.stringify(out);
        })()"""

        def scenario(theme):
            reset_fixtures()
            time.sleep(0.6)
            open_tools()
            driver.wait("document.querySelector('[data-testid=\\\"restore-banner\\\"]')?.textContent.includes('3 项')", 40)
            time.sleep(0.4)
            log(f"  contrast banner {theme}: " + str(driver.js(CONTRAST_JS)))
            shot(f"restore-banner-{theme}")

            click('[data-testid="restore-banner-open"]')
            driver.wait("!!document.querySelector('[data-testid=\"restore-view\"]')", 30)
            driver.wait("!!document.querySelector('[data-testid=\"restore-group-create\"]')", 30)
            time.sleep(0.5)
            log(f"  contrast preview {theme}: " + str(driver.js(CONTRAST_JS)))
            shot(f"restore-preview-{theme}")

            click('[data-testid="restore-run"]')
            time.sleep(0.9)
            log(f"  contrast running {theme}: " + str(driver.js(CONTRAST_JS)))
            shot(f"restore-running-{theme}")
            driver.wait("!!document.querySelector('[data-testid=\"restore-result\"]')", 60)
            time.sleep(0.5)
            log(f"  contrast result {theme}: " + str(driver.js(CONTRAST_JS)))
            shot(f"restore-result-{theme}")
            click('[data-testid="restore-done"]')
            time.sleep(0.5)

            # Corrupt the manifest and grab the rebuild confirmation.
            close_panel()
            write_text(manifest_path, "<<<<<<< HEAD\n{\"version\":1}\n=======\n{\"version\":1}\n>>>>>>> other\n")
            open_tools()
            driver.wait("document.querySelector('[data-testid=\\\"restore-banner\\\"]')?.textContent.includes('无法读取')", 40)
            click('[data-testid="restore-banner-open"]')
            driver.wait("!!document.querySelector('[data-testid=\"restore-rebuild\"]')", 20)
            time.sleep(0.3)
            click('[data-testid="restore-rebuild"]')
            driver.wait("!!document.querySelector('[data-testid=\"restore-rebuild-dialog\"]')", 20)
            time.sleep(0.4)
            log(f"  contrast unreadable {theme}: " + str(driver.js(CONTRAST_JS)))
            shot(f"restore-unreadable-{theme}")
            click('[data-testid="restore-rebuild-cancel"]')
            time.sleep(0.3)
            click('[data-testid="restore-back"]')
            time.sleep(0.3)
            close_panel()

        register_project_once()
        scenario("light")
        driver.js("window.__mkbApp.getState().setTheme('dark'); true")
        time.sleep(1.2)
        scenario("dark")
        return 0
    finally:
        server.shutdown()
        try:
            app.terminate()
            app.wait(timeout=8)
        except Exception:  # noqa: BLE001
            subprocess.run(["taskkill", "/F", "/PID", str(app.pid), "/T"], capture_output=True)


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:  # noqa: BLE001
        import traceback

        traceback.print_exc()
        sys.exit(1)
