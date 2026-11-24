#!/usr/bin/env python3
"""Capture the M8 skills-page states from the real window (T11).

Eight states in light and dark plus the two narrow shots, written into
design/r2/shots-m8/. Also samples the contrast of the muted text on the glass
cards from the actual pixels and prints the ratios.

Usage (from the repo root, with no other Mile Knowledge Base instance):
  python -I scripts/visual/capture-m8.py
"""

import importlib.util
import ctypes
import io
import json
import os
import subprocess
import sys
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

SHOTS = ROOT / "design" / "r2" / "shots-m8"
SHOTS.mkdir(parents=True, exist_ok=True)
PORT = 18461


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
        "$sig='[DllImport(\"user32.dll\")] public static extern bool SetWindowPos(IntPtr hWnd, IntPtr after, int x, int y, int cx, int cy, uint flags);';"
        "$t=Add-Type -MemberDefinition $sig -Name W -Namespace P -PassThru;"
        f"$p=Get-Process -Id {APP.proc.pid};"
        f"if($p.MainWindowHandle -ne 0){{[P.W]::SetWindowPos($p.MainWindowHandle,[IntPtr]::Zero,0,0,{width},{height},0x0006)|Out-Null}}"
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
        "(() => { if (document.getElementById('m8-motion-off')) return true;"
        "const s=document.createElement('style'); s.id='m8-motion-off';"
        "s.textContent='.skills-page *, .skills-page *::before, .skills-page *::after { animation: none !important; transition: none !important; }';"
        "document.head.appendChild(s); return true; })()"
    )


def save(name):
    time.sleep(1.2)
    # Park the pointer outside the window so no card keeps its hover state.
    ctypes.windll.user32.SetCursorPos(0, 0)
    time.sleep(0.2)
    png = APP.driver.capture()
    (SHOTS / f"{name}.png").write_bytes(png)
    log(f"  captured {name}")
    return png


def set_theme(theme):
    APP.js(f"window.__mkbApp.getState().setTheme('{theme}'); true")
    APP.wait(f"document.documentElement.dataset.theme === '{theme}'", 15, settle=0.4)


def goto_skills():
    APP.js("window.__mkbApp.getState().showPage('skills'); true")
    APP.wait("!!document.querySelector('[data-testid=\"skills-page\"]')", 20)
    motion_off()


def nav(title):
    APP.js("(() => { const b=document.querySelector('[data-testid=\"skills-nav\"] button[title=" + json.dumps(title) + "]'); if (b) b.click(); return true; })()")
    time.sleep(0.5)


def card_button(label):
    return (
        "(() => { const card=document.querySelector('[data-testid^=\"kb-card-\"]');"
        f"const b=card && [...card.querySelectorAll('button')].find((x)=>x.textContent.trim()==='{label}');"
        "if (b) b.click(); return !!b; })()"
    )


def esc():
    APP.js("document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); true")
    time.sleep(0.4)


def clear_search():
    APP.js(
        "(() => { const t=document.querySelector('[data-testid=\"kb-skills-search\"]');"
        "if (!t) return true; const set=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set;"
        "set.call(t, ''); t.dispatchEvent(new Event('input',{bubbles:true})); return true; })()"
    )
    time.sleep(0.3)


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
    crop = img.crop((
        max(0, int(box["x"] * scale)),
        max(0, int(box["y"] * scale)),
        min(img.width, int((box["x"] + box["w"]) * scale)),
        min(img.height, int((box["y"] + box["h"]) * scale)),
    ))
    pixels = list(crop.getdata())
    if len(pixels) < 10:
        return None
    lums = sorted(luminance(pixels))
    # The background is the most common luminance; the text is the far tail.
    buckets = {}
    for value in lums:
        key = round(value, 2)
        buckets[key] = buckets.get(key, 0) + 1
    bg = max(buckets.items(), key=lambda item: item[1])[0]
    mean = sum(lums) / len(lums)
    # The crop is text on a plain card, so the extreme pixel is a glyph core.
    text = lums[0] if mean > 0.5 else lums[-1]
    hi, lo = max(bg, text), min(bg, text)
    return round((hi + 0.05) / (lo + 0.05), 2)


def scenarios(prefix):
    goto_skills()
    nav("全部技能")
    APP.wait("!!document.querySelector('[data-testid^=\"kb-card-\"]')", 40)
    clear_search()
    time.sleep(0.5)

    png = save(f"kb-{prefix}")
    inner = APP.js("window.innerWidth")
    boxes = rects({
        "desc": ".kb-card .kb-card-desc",
        "cat": ".kb-card .kb-card-cat",
    })
    for key, box in boxes.items():
        log(f"  contrast {key} {prefix}: {contrast_of(png, box, inner)}")

    # batch selection
    APP.js("(() => { const b=document.querySelector('[aria-label^=\"更多操作：\"]'); if (b) b.click(); return true; })()")
    time.sleep(0.2)
    APP.js("(() => { const b=document.querySelector('[data-testid=\"kb-batch-toggle\"]'); if (b) b.click(); return true; })()")
    time.sleep(0.4)
    APP.js(
        "(() => { const boxes=[...document.querySelectorAll('[role=checkbox][aria-label^=\"选择 \"]')].slice(0,2);"
        "boxes.forEach((b)=>b.click()); return boxes.length; })()"
    )
    time.sleep(0.3)
    save(f"kb-select-{prefix}")
    esc()
    nav("全部技能")

    # tools
    nav("全部工具")
    APP.wait("!!document.querySelector('[data-testid^=\"tool-skill-\"]')", 40)
    time.sleep(0.5)
    png = save(f"tools-{prefix}")
    boxes = rects({"path": ".tool-row-path", "name": ".tool-row-name"})
    for key, box in boxes.items():
        log(f"  contrast {key} {prefix}: {contrast_of(png, box, APP.js('window.innerWidth'))}")

    # distribute
    nav("全部技能")
    APP.wait("!!document.querySelector('[data-testid^=\"kb-card-\"]')", 20)
    APP.js(card_button("分发"))
    APP.wait("!!document.querySelector('[data-testid=\"distribute-dialog\"]')", 20, settle=0.4)
    save(f"distribute-{prefix}")
    esc()

    # uninstall confirm from the tool list
    nav("全部工具")
    APP.wait("!!document.querySelector('[data-testid^=\"tool-uninstall-\"]')", 30)
    APP.js("(() => { const b=document.querySelector('[data-testid^=\"tool-uninstall-\"]'); if (b) b.click(); return true; })()")
    APP.wait("!!document.querySelector('[data-testid=\"uninstall-dialog\"]')", 20, settle=0.4)
    save(f"uninstall-{prefix}")
    esc()

    # search with no matches
    nav("全部技能")
    APP.js(
        "(() => { const t=document.querySelector('[data-testid=\"kb-skills-search\"]');"
        "const set=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set;"
        "set.call(t, 'pdf-tools'); t.dispatchEvent(new Event('input',{bubbles:true})); return true; })()"
    )
    APP.wait("!!document.querySelector('[data-testid=\"kb-empty\"]')", 10, settle=0.4)
    save(f"search-empty-{prefix}")
    clear_search()

    # scanning (skeleton cards, no cached payload)
    APP.js(
        "(() => { window.__savedScan = window.__mkbSkills.getState().scan;"
        "window.__mkbSkills.setState({ scan: { cachedAt: new Date().toISOString(), scanning: true, kb: [], tools: [] } });"
        "return true; })()"
    )
    APP.wait("!!document.querySelector('[data-testid=\"kb-skeletons\"]')", 10, settle=0.5)
    save(f"scanning-{prefix}")
    APP.js("window.__mkbSkills.setState({ scan: window.__savedScan }); true")
    nav("全部技能")


def main():
    if not cap.APP.exists():
        log("missing e2e build: GOFLAGS=-tags=e2e MYGO_INSPECTOR=1 bun run build")
        return 2
    if subprocess.run(["tasklist", "/FI", "IMAGENAME eq Mile Knowledge Base.exe"], capture_output=True, text=True).stdout.find("Mile Knowledge") >= 0:
        log("abort: Mile Knowledge Base is already running")
        return 2

    cap.vault = cap.build_vault()
    for rel, name, desc in [
        ("react", "React 技巧", "受控组件与状态提升；用组合而不是继承，把副作用放进 Effect。"),
        ("web-vue", "Vue", "组合式 API 与响应式系统；模板里只放表达式，逻辑留在 setup。"),
        ("nodejs", "Node", "事件循环与流；错误优先回调改成 Promise，背压用 pipeline。"),
        ("harmonyos", "鸿蒙", "ArkTS 状态管理；页面生命周期与组件复用。"),
    ]:
        write(f"skills/{rel}/SKILL.md", f"---\nname: {name}\ndescription: {desc}\n---\n\n# {name}\n")
    vid = cap.vault_id_of(cap.vault)
    appdata = cap.canon(tempfile.mkdtemp(prefix="mkb-m8-appdata-"))
    cap.write_settings(appdata, "light", cap.vault, vid)
    cap.home = cap.canon(tempfile.mkdtemp(prefix="mkb-m8-home-"))
    for name in ["alpha", "beta", "gamma"]:
        path = Path(cap.home) / ".claude" / "skills" / name
        path.mkdir(parents=True, exist_ok=True)
        (path / "SKILL.md").write_text(f"# {name}\n\n独立技能。\n", encoding="utf-8")
    (Path(cap.home) / "registry.json").write_text('{"uninstall": []}', encoding="utf-8")

    APP.start(appdata)
    try:
        APP.wait("!!document.querySelector('[data-testid=\"overview-grid\"]')", 90)
        APP.wait("!document.body.innerText.includes('未索引')", 90)
        for theme in ["light", "dark"]:
            set_theme(theme)
            goto_skills()
            log(f"== {theme}")
            scenarios(theme)

        # accent sweep: six presets × two themes, sampled in memory
        log("== accents")
        worst = {}
        for theme in ["light", "dark"]:
            set_theme(theme)
            goto_skills()
            nav("全部技能")
            APP.wait("!!document.querySelector('[data-testid^=\"kb-card-\"]')", 30)
            for preset in ["orange", "indigo", "blue", "green", "teal", "violet"]:
                APP.js(f"window.__mkbApp.getState().setAccent({{preset:'{preset}'}}); true")
                APP.js(f"document.documentElement.dataset.accent='{preset}'; true")
                time.sleep(0.4)
                png = APP.driver.capture()
                inner = APP.js("window.innerWidth")
                for key, selector in {
                    "desc": ".kb-card .kb-card-desc",
                    "active": ".skills-nav-item[aria-current='page'] .skills-nav-label",
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

        # narrow window, light only
        set_theme("light")
        resize(744, 640)
        goto_skills()
        nav("全部技能")
        APP.wait("!!document.querySelector('[data-testid^=\"kb-card-\"]')", 30, settle=0.5)
        save("kb-narrow-light")
        nav("全部工具")
        APP.wait("!!document.querySelector('[data-testid^=\"tool-uninstall-\"]')", 30)
        APP.js("(() => { const b=document.querySelector('[data-testid^=\"tool-uninstall-\"]'); if (b) b.click(); return true; })()")
        APP.wait("!!document.querySelector('[data-testid=\"uninstall-dialog\"]')", 20, settle=0.4)
        save("uninstall-narrow-light")
        esc()
    finally:
        APP.stop()

    # no vault
    appdata2 = cap.canon(tempfile.mkdtemp(prefix="mkb-m8-novault-"))
    (Path(appdata2) / "Mile Knowledge Base").mkdir(parents=True, exist_ok=True)
    APP.start(appdata2)
    try:
        APP.wait("!!document.querySelector('.welcome-shell')", 60)
        goto_skills()
        APP.wait("!!document.querySelector('[data-testid=\"skills-no-vault\"]')", 30)
        for theme in ["light", "dark"]:
            set_theme(theme)
            time.sleep(0.5)
            save(f"novault-{theme}")
    finally:
        APP.stop()

    log(f"done: {len(list(SHOTS.glob('*.png')))} screenshots in {SHOTS}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
