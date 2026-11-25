#!/usr/bin/env python3
"""Capture the M9 skills-page states from the real window (T08).

Eleven states from UI设计稿 12.2 in light and dark plus the three narrow
shots, written into design/r2/shots-m9-real/. The install / distribute result
states go through the real flows against a local fake GitHub (never the real
network, never real tool directories). Also samples text contrast across the
six accent presets in both themes.

Usage (from the repo root, with no other Mile Knowledge Base instance):
  python -I scripts/visual/capture-m9.py
"""

import importlib.util
import ctypes
import io
import json
import os
import subprocess
import sys
import tarfile
import tempfile
import time
from pathlib import Path

from PIL import Image as PILImage

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
spec = importlib.util.spec_from_file_location("capture_common", HERE / "capture-new.py")
cap = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(cap)

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

SHOTS = ROOT / "design" / "r2" / "shots-m9-real"
SHOTS.mkdir(parents=True, exist_ok=True)
PORT = 18571
GH_PORT = 18572

FAILURES = []


def log(message):
    print(message, flush=True)


def write(rel, text):
    full = Path(cap.vault) / rel
    full.parent.mkdir(parents=True, exist_ok=True)
    full.write_text(text, encoding="utf-8", newline="\n")


def ps(script):
    subprocess.run(["powershell", "-NoProfile", "-Command", script], capture_output=True)


def resize(width, height):
    ps(
        "$sig='[DllImport(\"user32.dll\")] public static extern bool SetWindowPos(IntPtr hWnd, IntPtr after, int x, int y, int cx, int cy, uint flags);"
        "[DllImport(\"user32.dll\")] public static extern bool ShowWindow(IntPtr hWnd, int cmd);';"
        "$t=Add-Type -MemberDefinition $sig -Name W9 -Namespace P -PassThru;"
        f"$p=Get-Process -Id {APP.proc.pid};"
        "if($p.MainWindowHandle -ne 0){[P.W9]::ShowWindow($p.MainWindowHandle,9)|Out-Null;"
        f"[P.W9]::SetWindowPos($p.MainWindowHandle,[IntPtr]::Zero,0,0,{width},{height},0x0006)|Out-Null}}"
    )
    time.sleep(0.8)


class App:
    proc: subprocess.Popen | None = None

    def start(self, appdata):
        env = dict(os.environ)
        env.update({
            "APPDATA": str(Path(appdata)),
            "MYGO_AUTOMATION": f"tcp:127.0.0.1:{PORT}",
            "SKILLS_FAKE_HOME": str(cap.home),
            "SKILLS_FAKE_PATH": str(Path(cap.home) / ".local" / "bin"),
            "SKILLS_FAKE_REGISTRY": str(Path(cap.home) / "registry.json"),
            "SKILLS_FAKE_GITHUB": f"http://127.0.0.1:{GH_PORT}",
        })
        self.proc = subprocess.Popen([str(cap.APP)], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.driver = cap.Driver(PORT)
        self.driver.connect(60)

    def stop(self):
        if self.proc:
            subprocess.run(["taskkill", "/F", "/PID", str(self.proc.pid)], capture_output=True)
            self.proc = None
        time.sleep(1.5)

    @property
    def js(self):
        return self.driver.js

    @property
    def wait(self):
        return self.driver.wait


APP = App()


def motion_off():
    APP.js(
        "(() => { if (document.getElementById('m9-motion-off')) return true;"
        "const s=document.createElement('style'); s.id='m9-motion-off';"
        "s.textContent='.skills-page *, .skills-page *::before, .skills-page *::after { animation: none !important; transition: none !important; }';"
        "document.head.appendChild(s); return true; })()"
    )


def save(name, settle=1.0):
    time.sleep(settle)
    ctypes.windll.user32.SetCursorPos(0, 0)
    time.sleep(0.2)
    try:
        png = APP.driver.capture()
        image = PILImage.open(io.BytesIO(png)).convert("RGB").resize((1440, 900), PILImage.LANCZOS)
        image.save(SHOTS / f"{name}.png")
        log(f"  captured {name}")
    except Exception as error:  # noqa: BLE001 - a failed shot must not stop the run
        FAILURES.append(f"{name}: {error}")
        log(f"  FAILED {name}: {error}")


def save_raw(name):
    """Narrow shots keep the real window size instead of the 1440x900 resize."""
    time.sleep(1.0)
    ctypes.windll.user32.SetCursorPos(0, 0)
    time.sleep(0.2)
    try:
        png = APP.driver.capture()
        (SHOTS / f"{name}.png").write_bytes(png)
        log(f"  captured {name}")
    except Exception as error:  # noqa: BLE001
        FAILURES.append(f"{name}: {error}")
        log(f"  FAILED {name}: {error}")


def set_theme(theme):
    APP.js(f"window.__mkbApp.getState().setTheme('{theme}'); true")
    APP.wait(f"document.documentElement.dataset.theme === '{theme}'", 15, settle=0.4)


def goto_skills():
    APP.js("window.__mkbApp.getState().showPage('skills'); true")
    APP.wait("!!document.querySelector('[data-testid=\"skills-page\"]')", 20)
    motion_off()


def nav(title):
    APP.js(
        "(() => { const b=[...document.querySelectorAll('[data-testid=\"skills-nav\"] button')]"
        ".find((x)=>x.getAttribute('title')===" + json.dumps(title) + "||x.textContent.trim()===" + json.dumps(title) + ");"
        " if (b) b.click(); return !!b; })()"
    )
    time.sleep(0.5)


def esc():
    APP.js("document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); true")
    time.sleep(0.4)


def fill(testid, value):
    APP.js(
        "(() => { const t=document.querySelector('[data-testid=\"" + testid + "\"]');"
        "if (!t) return false; const proto=t.tagName==='TEXTAREA'?HTMLTextAreaElement:HTMLInputElement;"
        "const set=Object.getOwnPropertyDescriptor(proto.prototype,'value').set;"
        "set.call(t, " + json.dumps(value) + "); t.dispatchEvent(new Event('input',{bubbles:true})); return true; })()"
    )
    time.sleep(0.3)


def click_button(selector):
    APP.js(
        "(() => { const b=document.querySelector(" + json.dumps(selector) + "); if (b) b.click(); return !!b; })()"
    )


def rects(selectors):
    out = {}
    for key, selector in selectors.items():
        value = APP.js(
            "(() => { const el=document.querySelector(" + json.dumps(selector) + ");"
            "if (!el) return null; const r=el.getBoundingClientRect();"
            "return JSON.stringify({x:r.left, y:r.top, w:r.width, h:r.height}); })()"
        )
        if value:
            out[key] = json.loads(value)
    return out


def luminance(pixels):
    def f(v):
        v /= 255
        return v / 12.92 if v <= 0.04045 else ((v + 0.055) / 1.055) ** 2.4

    return [0.2126 * f(c[0]) + 0.7152 * f(c[1]) + 0.0722 * f(c[2]) for c in pixels]


def contrast_of(png, box, inner_width):
    img = PILImage.open(io.BytesIO(png)).convert("RGB")
    scale = img.width / inner_width
    x0 = max(0, int(box["x"] * scale))
    y0 = max(0, int(box["y"] * scale))
    x1 = min(img.width, int((box["x"] + box["w"]) * scale))
    y1 = min(img.height, int((box["y"] + box["h"]) * scale))
    if x1 <= x0 or y1 <= y0:
        return None  # element outside the viewport or zero-sized
    crop = img.crop((x0, y0, x1, y1))
    pixels = list(crop.getdata())
    if len(pixels) < 10:
        return None
    lums = sorted(luminance(pixels))
    buckets = {}
    for value in lums:
        key = round(value, 2)
        buckets[key] = buckets.get(key, 0) + 1
    bg = max(buckets.items(), key=lambda item: item[1])[0]
    mean = sum(lums) / len(lums)
    text = lums[0] if mean > 0.5 else lums[-1]
    hi, lo = max(bg, text), min(bg, text)
    return round((hi + 0.05) / (lo + 0.05), 2)


def github_install(repo, pick, finish=True, running_shot=None):
    """Drive the install dialog through the fake GitHub; ends on the result step.

    running_shot, when given, is a capture name: right after submitting, the
    in-flight state (progress table, disabled close) is captured under it.
    """
    click_button('[data-testid="kb-install-top"]')
    APP.wait('document.querySelector(\'[data-testid="skills-modal"] [data-testid="install-github-form"]\') !== null', 20, settle=0.3)
    fill("install-github-input", repo)
    click_button('[data-testid="install-github-read"]')
    APP.wait("document.querySelector('[data-testid=\"install-candidates\"] input[type=checkbox]') !== null", 40, settle=0.3)
    # Only the requested candidate: uncheck everything, then check the target.
    # The data-testid sits on the checkbox input itself.
    APP.js(
        "(() => { document.querySelectorAll('[data-testid=\"install-candidates\"] input[type=checkbox]:checked')"
        ".forEach((c)=>c.click()); return true; })()"
    )
    time.sleep(0.2)
    click_button(f'[data-testid="install-candidate-skills/{pick}"]')
    time.sleep(0.2)
    fill("install-category", "文档")
    click_button('[data-testid="install-submit"]')
    if running_shot:
        # The fixture's ~18 MiB of reference files keep the copy running for
        # a couple of seconds, so a plain poll reliably catches the busy state.
        try:
            APP.wait('!!document.querySelector(\'[data-testid="install-busy"]\')', 6, settle=0.1)
            save(running_shot, settle=0.1)
        except RuntimeError:
            log("  install finished before the running state could be captured")
    APP.wait("document.querySelector('[data-testid=\"install-result\"]') !== null", 90, settle=0.4)
    if finish:
        click_button('[data-testid="install-done"]')
        APP.wait("document.querySelector('[data-testid=\"skills-modal\"]') === null", 15)


def scenarios(prefix):
    goto_skills()
    nav("全部技能")
    APP.wait("document.querySelector('[data-testid=\"kb-card-vue\"]') !== null", 40, settle=0.4)
    time.sleep(0.4)

    # 1. kb list with the new navigation
    save(f"kb-{prefix}")

    # 2. detail drawer
    click_button('[data-testid="kb-card-vue"]')
    APP.wait('document.querySelector(\'[data-testid="skill-drawer"]\')?.getAttribute(\'data-state\') === "ready"', 15, settle=0.5)
    save(f"detail-{prefix}")
    esc()
    APP.wait("document.querySelector('[data-testid=\"skill-drawer\"]') === null", 10)

    # 3. detail missing: delete the file while the page rescan is frozen
    APP.js(
        "(() => { const s=window.__mkbSkills;"
        "window.__m9Saved={refresh:s.getState().refresh,loadScan:s.getState().loadScan};"
        "s.setState({refresh:async()=>{},loadScan:async()=>{}}); return true; })()"
    )
    victim = Path(cap.vault) / "skills" / "后端" / "go-tips" / "SKILL.md"
    backup = victim.read_bytes()
    victim.unlink()
    click_button('[data-testid="kb-card-go-tips"]')
    APP.wait('document.querySelector(\'[data-testid="skill-drawer"]\')?.getAttribute(\'data-state\') === "missing"', 15, settle=0.4)
    save(f"detail-missing-{prefix}")
    esc()
    victim.write_bytes(backup)
    APP.js(
        "(() => { const s=window.__mkbSkills; s.setState(window.__m9Saved); return true; })()"
    )
    time.sleep(0.3)

    # 4-5. nav filter / collapsed
    fill("skills-nav-filter", "vue")
    time.sleep(0.5)
    save(f"nav-filter-{prefix}")
    fill("skills-nav-filter", "")
    click_button('[data-testid="skills-nav-toggle-kb"]')
    time.sleep(0.5)
    save(f"nav-collapsed-{prefix}")
    click_button('[data-testid="skills-nav-toggle-kb"]')
    time.sleep(0.3)

    # 6. install dialog (source step)
    click_button('[data-testid="kb-install-top"]')
    APP.wait('document.querySelector(\'[data-testid="skills-modal"] [data-testid="install-github-form"]\') !== null', 20, settle=0.3)
    save(f"install-{prefix}")
    esc()
    APP.wait("document.querySelector('[data-testid=\"skills-modal\"]') === null", 10)

    # 7. install running + result: real install through the fake GitHub. The
    # light round installs pdf, the dark round docx — a second install of the
    # same skill would be blocked as already existing. docx carries hundreds
    # of small files, so its install stays in flight long enough to capture
    # the running state in the dark round.
    pick = "pdf" if prefix == "light" else "docx"
    github_install("anthropics/skills", pick, finish=False, running_shot=f"install-running-{prefix}")
    save(f"install-result-{prefix}")
    click_button('[data-testid="install-done"]')
    APP.wait("document.querySelector('[data-testid=\"skills-modal\"]') === null", 15)

    # 8. distribute dialog: batch-select two skills (go-tips included, so the
    # overwrite case below has a target) with the batch bar behind the dialog.
    # The batch toggle lives in the card's 更多操作 menu.
    APP.js(
        "(() => { const c=document.querySelector('[data-testid=\"kb-card-vue\"]');"
        " const m=c && c.querySelector('button[aria-label^=\"更多操作\"]'); if (m) m.click(); return !!m; })()"
    )
    time.sleep(0.3)
    click_button('[data-testid="kb-batch-toggle"]')
    time.sleep(0.4)
    APP.js(
        "(() => { const wanted=['vue','go-tips']; let n=0;"
        "for (const name of wanted) { const box=document.querySelector('.kb-check[aria-label=\"选择 ' + name + '\"]');"
        " if (box && box.getAttribute('aria-checked') !== 'true') { box.click(); n++; } } return n; })()"
    )
    time.sleep(0.3)
    click_button('[data-testid="kb-batch-distribute"]')
    APP.wait('document.querySelector(\'[data-testid="skills-modal"] [data-testid="distribute-dialog"]\') !== null', 15, settle=0.4)
    save(f"distribute-{prefix}")
    esc()
    APP.wait("document.querySelector('[data-testid=\"skills-modal\"]') === null", 10)
    # Leave batch selection so the cards show their buttons again.
    click_button('[data-testid="kb-batch-cancel"]')
    time.sleep(0.4)

    # 9b. overwrite confirmation: single-skill distribute of go-tips, which
    # exists in the fake HOME. Only a single-skill dialog runs the preview
    # that marks the target as existing (batch dialogs execute directly).
    nav("全部技能")
    APP.wait("document.querySelector('[data-testid=\"kb-card-go-tips\"]') !== null", 20)
    APP.js(
        "(() => { const card=document.querySelector('[data-testid=\"kb-card-go-tips\"]');"
        "const b=card && [...card.querySelectorAll('button')].find((x)=>x.textContent.trim()==='分发');"
        "if (b) b.click(); return !!b; })()"
    )
    try:
        APP.wait('document.querySelector(\'[data-testid="skills-modal"] [data-testid="distribute-dialog"]\') !== null', 15, settle=0.4)
        click_button('[data-testid="distribute-target-claude-code"]')
        APP.wait('!!document.querySelector(\'.skills-target-overwrite input[type=checkbox]\')', 12, settle=0.3)
        APP.js(
            "(() => { const b=document.querySelector('.skills-target-overwrite input[type=checkbox]');"
            " if (b) b.click(); return !!b; })()"
        )
        APP.wait('document.querySelector(\'[data-testid="distribute-overwrite-confirm"]\') !== null', 8, settle=0.4)
        save(f"distribute-overwrite-{prefix}")
        click_button('[data-testid="distribute-overwrite-keep"]')
        time.sleep(0.3)
    except RuntimeError as error:
        log(f"  distribute-overwrite failed: {error}")
    esc()
    APP.wait("document.querySelector('[data-testid=\"skills-modal\"]') === null", 10)

    # 10. distribute result: single-skill distribute from the card button
    nav("全部技能")
    APP.wait("document.querySelector('[data-testid=\"kb-card-vue\"]') !== null", 20)
    APP.js(
        "(() => { const card=document.querySelector('[data-testid=\"kb-card-vue\"]');"
        "const b=card && [...card.querySelectorAll('button')].find((x)=>x.textContent.trim()==='分发');"
        "if (b) b.click(); return !!b; })()"
    )
    APP.wait('document.querySelector(\'[data-testid="skills-modal"] [data-testid="distribute-dialog"]\') !== null', 15, settle=0.4)
    # Make sure at least one target is selected before submitting.
    APP.js(
        "(() => { const t=document.querySelector('[data-testid=\"distribute-target-claude-code\"]');"
        " if (t && t.getAttribute('aria-checked') !== 'true') t.click(); return true; })()"
    )
    time.sleep(0.2)
    time.sleep(0.3)
    click_button('[data-testid="distribute-submit"]')
    try:
        APP.wait('document.querySelector(\'[data-testid="distribute-result"]\') !== null', 30, settle=0.4)
        save(f"distribute-result-{prefix}")
        click_button('[data-testid="distribute-done"]')
        APP.wait("document.querySelector('[data-testid=\"skills-modal\"]') === null", 10)
    except RuntimeError as error:
        log(f"  skip distribute-result ({error})")
        esc()


def main():
    if not cap.APP.exists():
        log("missing e2e build: GOFLAGS=-tags=e2e MYGO_INSPECTOR=1 bun run build")
        return 2
    if subprocess.run(["tasklist", "/FI", "IMAGENAME eq Mile Knowledge Base.exe"], capture_output=True, text=True).stdout.find("Mile Knowledge") >= 0:
        log("abort: Mile Knowledge Base is already running")
        return 2

    cap.vault = cap.build_vault()
    # Extra fixtures: categories, a scrollable list, and a skill that also
    # exists in the fake HOME (overwrite confirmation).
    write("skills/前端/vue/SKILL.md", "---\nname: vue\ndescription: 组合式 API 与响应式系统；模板里只放表达式。\n---\n\n# vue\n\n组合式 API 的写法。\n\n## 步骤\n\n1. 先读结构。\n2. 再改。\n")
    write("skills/前端/mixa/SKILL.md", "---\nname: mixa\ndescription: 混合示例。\n---\n\n# mixa\n\n混合。\n")
    write("skills/后端/go-tips/SKILL.md", "---\nname: go-tips\ndescription: Go 工程实践。\n---\n\n# go-tips\n\n错误处理与表驱动测试。\n")
    for i in range(30):
        write(f"skills/zz-scroll/scroll-{i:02d}/SKILL.md", f"---\nname: scroll-{i:02d}\ndescription: 滚动回归 {i:02d}。\n---\n\n# scroll-{i:02d}\n")

    vid = cap.vault_id_of(cap.vault)
    appdata = cap.canon(tempfile.mkdtemp(prefix="mkb-m9-appdata-"))
    cap.write_settings(appdata, "light", cap.vault, vid)
    cap.home = cap.canon(tempfile.mkdtemp(prefix="mkb-m9-home-"))
    old = Path(cap.home) / ".claude" / "skills" / "go-tips"
    old.mkdir(parents=True, exist_ok=True)
    (old / "SKILL.md").write_text("# old go-tips\n\n独立的旧版本。\n", encoding="utf-8")
    (Path(cap.home) / ".local" / "bin").mkdir(parents=True, exist_ok=True)
    (Path(cap.home) / ".local" / "bin" / "claude.cmd").write_text("@echo off", encoding="utf-8")
    (Path(cap.home) / "registry.json").write_text('{"uninstall": []}', encoding="utf-8")

    # Local fake GitHub: one repository, two skills. Never touches the network.
    # docx carries many small files so an install stays in flight long enough
    # to capture the running state.
    fix = cap.canon(tempfile.mkdtemp(prefix="mkb-m9-gh-"))
    archive = os.path.join(fix, "v1.tar.gz")
    files = {"skills/pdf/SKILL.md": b"# pdf\n\nversion one\n", "skills/docx/SKILL.md": b"# docx\n\nversion one\n"}
    # Large enough files that the copy takes a couple of seconds — the
    # running state must stay up long enough to be captured.
    blob = (b"reference data for the install-running capture\n") * 2000  # ~92 KiB
    for i in range(200):
        files["skills/docx/ref/r%04d.md" % i] = blob
        files["skills/pdf/ref/r%04d.md" % i] = blob
    with tarfile.open(archive, "w:gz") as tar:
        for rel, data in files.items():
            info = tarfile.TarInfo("repo/" + rel)
            info.size = len(data)
            tar.addfile(info, io.BytesIO(data))
    head = "1" * 40
    state_path = os.path.join(fix, "state.json")
    Path(state_path).write_text(json.dumps({"repos": {"anthropics/skills": {
        "ref": "main", "head": head, "paths": {"skills/pdf": head, "skills/docx": head}, "archives": {head: archive},
    }}}), encoding="utf-8")
    github = subprocess.Popen(
        [sys.executable, "-I", str(ROOT / "scripts" / "e2e" / "fake-github.py"), state_path, str(GH_PORT)],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )

    APP.start(appdata)
    try:
        APP.wait("!!document.querySelector('[data-testid=\"overview-grid\"]')", 90)
        APP.wait("!document.body.innerText.includes('未索引')", 90)
        # Wait for the scan so the cards exist before the first capture.
        APP.js("window.__mkbApp.getState().showPage('skills'); true")
        deadline = time.time() + 40
        while time.time() < deadline:
            APP.js("window.__mkbSkills.getState().loadScan(window.__mkbApp.getState().activeVaultId); true")
            time.sleep(1.0)
            if APP.js("document.querySelector('[data-testid=\"kb-card-vue\"]') !== null"):
                break
        for theme in ["light", "dark"]:
            set_theme(theme)
            log(f"== {theme}")
            scenarios(theme)

        # Accent sweep: six presets x two themes, contrast sampled in memory.
        log("== accents")
        worst = {}
        for theme in ["light", "dark"]:
            set_theme(theme)
            goto_skills()
            nav("全部技能")
            APP.wait("document.querySelector('[data-testid^=\"kb-card-\"]') !== null", 30)
            for preset in ["orange", "indigo", "blue", "green", "teal", "violet"]:
                APP.js(f"window.__mkbApp.getState().setAccent({{preset:'{preset}'}}); true")
                APP.js(f"document.documentElement.dataset.accent='{preset}'; true")
                time.sleep(0.4)
                png = APP.driver.capture()
                inner = APP.js("window.innerWidth")
                for key, selector in {
                    "desc": ".kb-card .kb-card-desc",
                    "active": ".skills-nav-item[aria-current='page'] .skills-nav-label",
                    "curmark": ".skills-nav-curmark",
                }.items():
                    box = rects({key: selector}).get(key)
                    if not box:
                        continue
                    ratio = contrast_of(png, box, inner)
                    if ratio is not None:
                        worst[key] = min(worst.get(key, 99), ratio)
                        log(f"  contrast {key} {preset}/{theme}: {ratio}")
        log(f"  worst across accents: {worst}")
        APP.js("window.__mkbApp.getState().setAccent({preset:'orange'}); document.documentElement.dataset.accent='orange'; true")

        # Narrow window (960x720 like the prototype), light only.
        set_theme("light")
        resize(960, 720)
        deadline = time.time() + 6
        while time.time() < deadline and APP.js("window.innerWidth") > 1020:
            resize(960, 720)
            time.sleep(0.5)
        goto_skills()
        nav("全部技能")
        APP.wait("document.querySelector('[data-testid^=\"kb-card-\"]') !== null", 30, settle=0.5)
        click_button('[data-testid="kb-card-vue"]')
        APP.wait('document.querySelector(\'[data-testid="skill-drawer"]\')?.getAttribute(\'data-state\') === "ready"', 15, settle=0.5)
        save_raw("detail-narrow-light")
        esc()
        click_button('[data-testid="kb-install-top"]')
        APP.wait('document.querySelector(\'[data-testid="skills-modal"] [data-testid="install-github-form"]\') !== null', 20, settle=0.3)
        save_raw("install-narrow-light")
        esc()
        APP.wait("document.querySelector('[data-testid=\"skills-modal\"]') === null", 10)
        nav("全部技能")
        APP.js(
            "(() => { const card=document.querySelector('[data-testid=\"kb-card-vue\"]');"
            "const b=card && [...card.querySelectorAll('button')].find((x)=>x.textContent.trim()==='分发');"
            "if (b) b.click(); return !!b; })()"
        )
        APP.wait('document.querySelector(\'[data-testid="skills-modal"] [data-testid="distribute-dialog"]\') !== null', 15, settle=0.4)
        save_raw("distribute-narrow-light")
        esc()
    finally:
        APP.stop()
        github.terminate()
        subprocess.run(["taskkill", "/F", "/IM", "Mile Knowledge Base.exe"], capture_output=True)

    count = len(list(SHOTS.glob("*.png")))
    log(f"done: {count} screenshots in {SHOTS}")
    if FAILURES:
        log(f"FAILED SHOTS: {len(FAILURES)}")
        for item in FAILURES:
            log(f"  {item}")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
