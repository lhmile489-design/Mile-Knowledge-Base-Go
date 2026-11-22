#!/usr/bin/env python3
"""Capture the M6 management states from the real window (T10).

Runs the e2e build against a temporary vault, a fake HOME and a folder queue
(SKILLS_FAKE_PICK_DIR=@<file>, e2e builds only), drives the four management
states in light and dark, and writes them into design/r2/shots-m6/ (replacing
the prototype shots; the prototypes stay in git history). Also prints the
contrast ratios of the new text.

Usage (from the repo root, with no other Mile Knowledge Base instance):
  python -I scripts/visual/capture-m6.py
"""

import importlib.util
import io
import json
import os
import subprocess
import sys
import tempfile
import time
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

SHOTS = ROOT / "design" / "r2" / "shots-m6"
SHOTS.mkdir(parents=True, exist_ok=True)
PORT = cap.PORT


def log(message):
    print(message, flush=True)


def write_text(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(data, encoding="utf-8", newline="\n")


def write_skill(path):
    write_text(Path(path) / "SKILL.md", "# demo\n\n")


def set_queue(queue, *paths):
    write_text(Path(queue), "\n".join(paths) + "\n")


def main():
    if not cap.APP.exists():
        log(f"missing e2e build: {cap.APP}\nrun: GOFLAGS=-tags=e2e MYGO_INSPECTOR=1 bun run build")
        return 2

    appdata = cap.canon(tempfile.mkdtemp(prefix="mkb-m6-appdata-"))
    vault = cap.build_vault()
    vid = cap.vault_id_of(vault)
    cap.write_settings(appdata, "light", vault, vid)

    home = cap.canon(tempfile.mkdtemp(prefix="mkb-m6-home-"))
    write_text(Path(home) / ".local" / "bin" / "claude.cmd", "@echo off")
    write_text(Path(home) / ".claude" / "skills" / "other" / "SKILL.md", "# other\n")
    registry = Path(home) / "registry.json"
    write_text(registry, json.dumps({"uninstall": []}))

    work = Path(tempfile.mkdtemp(prefix="mkb-m6-fixtures-"))
    custom = work / "droid-skills"
    custom_b = work / "second-tool"
    project = work / "acme-site"
    gone = work / "gone-tool"
    for root in (custom, custom_b, gone):
        root.mkdir(parents=True, exist_ok=True)
    for index in range(3):
        write_skill(custom / f"skill-{index}")
    write_skill(custom_b / "extra")
    (project / ".claude" / "skills").mkdir(parents=True, exist_ok=True)
    (project / ".agents" / "skills").mkdir(parents=True, exist_ok=True)
    for index in range(2):
        write_skill(project / ".claude" / "skills" / f"web-{index}")
    write_skill(project / ".agents" / "skills" / "shared-one")

    queue = work / "pick-queue.txt"
    set_queue(queue, str(custom))

    env = {
        **os.environ,
        "APPDATA": appdata,
        "MYGO_AUTOMATION": f"tcp:127.0.0.1:{PORT}",
        "SKILLS_FAKE_HOME": home,
        "SKILLS_FAKE_PATH": str(Path(home) / ".local" / "bin"),
        "SKILLS_FAKE_REGISTRY": str(registry),
        "SKILLS_FAKE_PICK_DIR": "@" + str(queue),
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

        def open_targets():
            driver.js("window.__mkbApp.getState().openSkillsPanel(); true")
            driver.wait("!!document.querySelector('[data-testid=\"skills-panel\"]')", 20)
            time.sleep(0.4)
            click_text('[data-testid="skills-panel"]', "工具技能")
            time.sleep(0.4)
            click('[data-testid="manage-targets"]')
            driver.wait("!!document.querySelector('[data-testid=\"targets-view\"]')", 20)
            time.sleep(0.4)

        def register(kind, name, pick, install=None, save=True):
            set_queue(queue, pick)
            click(f'[data-testid="targets-add-{kind}"]')
            driver.wait("!!document.querySelector('[data-testid=\"target-name\"]')", 20)
            fill("target-name", name)
            click('[data-testid="target-pick"]')
            driver.wait("!!document.querySelector('[data-testid=\"target-picked\"]')", 20)
            time.sleep(0.4)
            if install:
                click(f'[data-testid="target-install-{install}"]')
                time.sleep(0.2)
            if save:
                click('[data-testid="target-save"]')
                driver.wait("!document.querySelector('[data-testid=\"target-name\"]')", 40)
                time.sleep(0.8)
                driver.js(
                    "(() => { const b=document.querySelector('[data-testid=\"skills-banner-close\"]');"
                    " if(b) b.click(); return true; })()"
                )
                time.sleep(0.3)

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
          const path = document.querySelector('.targets-path');
          if (path) out.path = ratio(getComputedStyle(path).color, bgOf(path));
          const stat = document.querySelector('.targets-stat');
          if (stat) out.stat = ratio(getComputedStyle(stat).color, bgOf(stat));
          const warnPill = document.querySelector('.targets-row .pill[data-tone="warn"]');
          if (warnPill) out.missingPill = ratio(getComputedStyle(warnPill).color, bgOf(warnPill));
          const fault = document.querySelector('.install-fault');
          if (fault) out.faultText = ratio(getComputedStyle(fault).color, bgOf(fault));
          return JSON.stringify(out);
        })()"""

        # One-time setup: three registrations (the third directory is deleted
        # afterwards so the list shows the missing state).
        open_targets()
        register("custom", "Droid Skills", str(custom))
        register("project", "acme-site", str(project), install=".claude/skills")
        register("custom", "Gone Tool", str(gone))
        subprocess.run(["cmd", "/c", "rmdir", "/s", "/q", str(gone)], check=False)
        time.sleep(0.5)

        def states(theme):
            open_targets()
            driver.js("window.__mkbSkills.getState().refresh(%s); true" % json.dumps(vid))
            time.sleep(1.2)
            log(f"  contrast {theme}: " + str(driver.js(CONTRAST_JS)))
            shot(f"targets-{theme}")

            # New custom form with a rejected directory (targets-custom-*).
            set_queue(queue, str(Path(vault) / "skills"))
            click('[data-testid="targets-add-custom"]')
            driver.wait("!!document.querySelector('[data-testid=\"target-name\"]')", 20)
            fill("target-name", "Vault Tool")
            click('[data-testid="target-pick"]')
            driver.wait("!!document.querySelector('[data-testid=\"target-picked\"]')", 20)
            time.sleep(0.3)
            click('[data-testid="target-save"]')
            driver.wait("!!document.querySelector('[data-testid=\"targets-error\"]')", 30)
            time.sleep(0.6)
            log(f"  contrast targets-custom {theme}: " + str(driver.js(CONTRAST_JS)))
            shot(f"targets-custom-{theme}")
            click('[data-testid="targets-cancel"]')
            time.sleep(0.4)

            # Project form with detected subdirectories (targets-project-*).
            set_queue(queue, str(project))
            click('[data-testid="targets-add-project"]')
            driver.wait("!!document.querySelector('[data-testid=\"target-name\"]')", 20)
            fill("target-name", "another-site")
            click('[data-testid="target-pick"]')
            driver.wait("!!document.querySelector('[data-testid=\"target-dirs\"]')", 20)
            time.sleep(0.4)
            click('[data-testid="target-install-.claude/skills"]')
            time.sleep(0.2)
            shot(f"targets-project-{theme}")
            click('[data-testid="targets-cancel"]')
            time.sleep(0.4)

            # Removal confirmation (targets-remove-*).
            driver.js(
                "(() => { const row=[...document.querySelectorAll('[data-testid^=\"target-row-t-\"]')]"
                ".find((el)=>el.textContent.includes('Droid Skills')); if(!row) throw new Error('missing row');"
                " row.querySelector('[data-testid^=\"target-remove-\"]').click(); return true; })()"
            )
            driver.wait("!!document.querySelector('[data-testid=\"target-remove-dialog\"]')", 20)
            time.sleep(0.5)
            shot(f"targets-remove-{theme}")
            click_text('[data-testid="target-remove-dialog"]', "取消")
            time.sleep(0.3)

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


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:  # noqa: BLE001
        import traceback

        traceback.print_exc()
        sys.exit(1)
