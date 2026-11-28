#!/usr/bin/env python3
"""M10 T10: real-window material screenshots and screenshot-sampled contrast.

Two jobs, both on the real window (temp APPDATA / LOCALAPPDATA / HOME and a
temp typical vault; nothing real is read or written):

1. Shots for UI设计稿 13.2: work, work-doc, menu, settings, settings-notes and
   welcome, each at blur off / low / std / high in light and dark, written to
   design/r2/shots-m10-real/. Animations are frozen first.
2. AC-MAT-5: contrast of secondary text, accent text and button text on the
   panel, card and strong tiers, 6 accent presets x light/dark x blur
   low/std/high, sampled from real screenshots. For each combination the page
   reports each sample's colour and box, the text is made transparent, a second
   capture is taken, and the effective background is the median pixel in the
   box (DESIGN 15.5 asks for screenshot sampling; computed colour walks cannot
   see a translucent tier over the field). Threshold 4.5. Worst values and any
   failure are printed and written to contrast.json.

Usage (repo root, no other Mile Knowledge Base instance, e2e build in place):
  python -I scripts/visual/capture-m10.py [--only shots|contrast]
"""

import base64
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

from PIL import Image

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
APP = ROOT / "build" / "windows-amd64" / "Mile Knowledge Base.exe"
OUT = ROOT / "design" / "r2" / "shots-m10-real"
PORT = 18581

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

PRESETS = ["orange", "indigo", "blue", "green", "teal", "violet"]
TIERS = ["off", "low", "std", "high"]
THEMES = ["light", "dark"]
STATES = ["work", "work-doc", "menu", "settings", "settings-notes"]
STATE_FILTER = None
CONTRAST_TIERS = ["low", "std", "high"]
LIMIT = 4.5

SAMPLES = [
    # selector, tier, label, kind
    ("header.titlebar .app-title", "panel", "标题栏应用名", "text"),
    (".vault-sidebar-name", "panel", "侧栏库名", "text"),
    (".history-empty", "panel", "历史空态说明（次级）", "text"),
    (".overview-title", "card", "磁贴标题（次级）", "text"),
    (".overview-link", "card", "磁贴强调链接", "text"),
    (".agent-picker-head", "strong", "菜单头（次级）", "text"),
    (".agent-option-name", "strong", "菜单项名", "text"),
    (".agent-option-badge", "strong", "菜单「使用中」（强调）", "text"),
    # AC-MAT-5 names button text too: the accent-filled send button carries
    # --on-accent over the accent fill, which is the weakest button case.
    (".composer-send", "card", "输入区发送按钮（按钮文字）", "button"),
]


def log(message):
    print(message, flush=True)


class Driver:
    def __init__(self, work, appdata, local, home, vault):
        self.work = work
        self.appdata = appdata
        self.local = local
        self.home = home
        self.vault = vault
        self.proc = None
        self.drive = work / "drive.exe"
        subprocess.run(
            ["go", "build", "-o", str(self.drive), "."],
            cwd=ROOT / "spikes" / "r4-window" / "drive",
            check=True,
        )

    def start(self):
        env = dict(os.environ)
        env.pop("MYGO_AUTOMATION", None)
        env.update(
            APPDATA=str(self.appdata),
            LOCALAPPDATA=str(self.local),
            USERPROFILE=str(self.home),
            HOME=str(self.home),
            MYGO_AUTOMATION="tcp:127.0.0.1:%d" % PORT,
        )
        self.proc = subprocess.Popen([str(APP)], env=env,
                                     stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        for _ in range(160):
            try:
                if self.eval("!!window.__mkbApp && !!window.__mkbApp.getState().settings"):
                    return
            except Exception:
                pass
            time.sleep(0.5)
        raise SystemExit("app did not become ready")

    def stop(self):
        if self.proc:
            subprocess.run(["powershell", "-NoProfile", "-Command",
                            "Stop-Process -Id %d -Force" % self.proc.pid],
                           capture_output=True)
            self.proc = None
        time.sleep(3)

    def _call(self, payload):
        req = self.work / "req.json"
        req.write_text(json.dumps(payload), encoding="utf-8")
        env = dict(os.environ)
        env["MYGO_AUTOMATION"] = "tcp:127.0.0.1:%d" % PORT
        out = subprocess.run([str(self.drive), "-f", str(req)], capture_output=True, timeout=120, env=env)
        line = out.stdout.decode("utf-8", "replace").strip().splitlines()
        for row in line:
            if row.startswith('{"id"'):
                return json.loads(row)["result"]
        raise RuntimeError("no automation reply: " + out.stderr.decode("utf-8", "replace")[:200])

    def eval(self, script):
        return self._call({"id": 1, "method": "eval", "params": {"window": 1, "script": script}}).get("value")

    def capture(self):
        result = self._call({"id": 1, "method": "capture", "params": {"window": 1}})
        return Image.open(__import__("io").BytesIO(base64.b64decode(result["png"]))).convert("RGB")


def seed(appdata, vault, theme="light", blur="std", motion="std"):
    out = Path(appdata) / "Mile Knowledge Base"
    out.mkdir(parents=True, exist_ok=True)
    doc = {
        "version": 1, "theme": theme, "closeBehavior": "quit",
        "effects": {"motion": motion, "blur": blur},
        "layout": {"sidebarWidth": 232, "docWidth": 620, "docCollapsed": False,
                   "sidebarCollapsed": False, "agnesControlsCollapsed": False,
                   "agnesHistoryOpen": True, "agnesComposerHeight": 96},
        "recentVaults": [], "lastVaultId": "",
    }
    if vault is not None:
        import hashlib
        root = os.path.abspath(vault)
        vid = hashlib.sha256(root.replace("/", chr(92)).lower().encode()).hexdigest()[:16]
        doc["recentVaults"] = [{"id": vid, "root": root, "displayName": "m10-shots",
                                "lastOpenedAt": "2026-11-27T09:00:00Z"}]
        doc["lastVaultId"] = vid
    (out / "settings.json").write_text(json.dumps(doc, ensure_ascii=False, indent=2) + "\n",
                                       encoding="utf-8")
    return doc["lastVaultId"]


FREEZE = """
(() => {
  if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
  document.getAnimations().forEach((a) => { try { a.pause(); a.currentTime = 0; a.finish(); } catch {} });
  return true;
})()
"""


def state_script(state, vid):
    open_settings = "window.__mkbApp.getState().openSettings(); true"
    if state == "work":
        return "window.__mkbApp.getState().closeSettings(); window.__mkbApp.getState().showPage('work'); true"
    if state in ("work-doc", "menu"):
        return ("window.__mkbApp.getState().closeSettings();"
                "window.__mkbApp.getState().showPage('work');"
                "window.__mkbDocs.getState().open('%s', 'raws/富文本样例.md').then(() => 'ok')" % vid)
    if state == "settings":
        return open_settings
    if state == "settings-notes":
        return ("window.__mkbApp.setState({ systemReducedMotion: true, effectsDegraded: true });"
                + open_settings)
    return "true"


def open_menu(driver):
    driver.eval("(() => { const b = document.querySelector('[data-testid=route-target]'); "
                "if (b) b.click(); return !!b; })()")
    time.sleep(0.6)


def close_menu(driver):
    driver.eval("document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true})); true")
    time.sleep(0.3)


def set_attrs(driver, accent, theme, blur):
    driver.eval(
        "(() => { const app = window.__mkbApp.getState();"
        " app.setAccent({ preset: '%s' }); app.setTheme('%s'); app.setEffects({ blur: '%s' });"
        " document.documentElement.dataset.theme = '%s'; return true; })()" % (accent, theme, blur, theme)
    )
    time.sleep(0.9)


def take_shots():
    work = Path(tempfile.mkdtemp(prefix=".m10-shots-", dir=ROOT))
    vault = work / "vault"
    subprocess.run(["go", "run", "./cmd/genvault", "-kind", "typical", "-out", str(vault)],
                   cwd=ROOT, check=True, stdout=subprocess.DEVNULL)
    driver = Driver(work, work / "app", work / "local", work / "home", vault)
    written = []
    try:
        vid = seed(work / "app", vault)
        driver.start()
        states = STATE_FILTER if STATE_FILTER else STATES
        for state in states:
            driver.eval(state_script(state, vid))
            time.sleep(1.2)
            if state == "menu":
                open_menu(driver)
            for blur in TIERS:
                for theme in THEMES:
                    set_attrs(driver, "orange", theme, blur)
                    if state == "menu":
                        close_menu(driver)
                        open_menu(driver)
                    driver.eval(FREEZE)
                    time.sleep(0.4)
                    name = "%s-%s-%s.png" % (state, blur, theme)
                    driver.capture().save(OUT / name)
                    written.append(name)
            if state == "menu":
                close_menu(driver)
        driver.stop()
        # welcome needs no vault: a second launch with an empty recent list
        seed(work / "app2", None)
        driver2 = Driver(work, work / "app2", work / "local2", work / "home", None)
        driver2.start()
        try:
            for blur in TIERS:
                for theme in THEMES:
                    set_attrs(driver2, "orange", theme, blur)
                    driver2.eval(FREEZE)
                    time.sleep(0.4)
                    name = "welcome-%s-%s.png" % (blur, theme)
                    driver2.capture().save(OUT / name)
                    written.append(name)
        finally:
            driver2.stop()
    finally:
        try:
            driver.stop()
        except Exception:
            pass
        shutil.rmtree(work, ignore_errors=True)
    log("shots written: %d -> %s" % (len(written), OUT))
    return written


SAMPLE_JS = """
(() => {
  const out = [];
  const specs = %s;
  for (const s of specs) {
    const el = document.querySelector(s[0]);
    if (!el) { out.push({ sel: s[0], tier: s[1], label: s[2], kind: s[3], missing: true }); continue; }
    const r = el.getBoundingClientRect();
    if (r.width < 4 || r.height < 4) { out.push({ sel: s[0], tier: s[1], label: s[2], kind: s[3], missing: true }); continue; }
    out.push({ sel: s[0], tier: s[1], label: s[2], kind: s[3],
               fg: getComputedStyle(el).color,
               rect: [Math.round(r.left), Math.round(r.top), Math.round(r.width), Math.round(r.height)] });
  }
  return JSON.stringify(out);
})()
""" % json.dumps(SAMPLES)

HIDE = """
(() => {
  const specs = %s;
  const style = document.createElement('style');
  style.id = 'm10-contrast-hide';
  style.textContent = specs.map((s) => s[0] + ' { color: transparent !important; }').join('\\n');
  document.head.appendChild(style);
  return true;
})()
""" % json.dumps(SAMPLES)

UNHIDE = "(() => { const s = document.getElementById('m10-contrast-hide'); if (s) s.remove(); return true; })()"


def parse_rgb(value):
    nums = [float(x) for x in value.replace("rgba(", "").replace("rgb(", "").replace(")", "").replace("/", " ").split(",") if x.strip()]
    return nums[:3]


def luminance(rgb):
    def f(v):
        v /= 255.0
        return v / 12.92 if v <= 0.04045 else ((v + 0.055) / 1.055) ** 2.4
    r, g, b = rgb
    return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b)


def ratio(fg, bg):
    a, b = luminance(fg), luminance(bg)
    hi, lo = max(a, b), min(a, b)
    return (hi + 0.05) / (lo + 0.05)


def median_color(image, rect):
    left, top, width, height = rect
    box = image.crop((max(0, left), max(0, top), max(0, left + width), max(0, top + height)))
    pixels = list(box.getdata())
    if not pixels:
        return None
    channels = list(zip(*pixels))
    return [sorted(c)[len(c) // 2] for c in channels]


def sample_contrast():
    work = Path(tempfile.mkdtemp(prefix=".m10-contrast-", dir=ROOT))
    vault = work / "vault"
    subprocess.run(["go", "run", "./cmd/genvault", "-kind", "typical", "-out", str(vault)],
                   cwd=ROOT, check=True, stdout=subprocess.DEVNULL)
    driver = Driver(work, work / "app", work / "local", work / "home", vault)
    results = []
    worst = {}
    failures = []
    try:
        vid = seed(work / "app", vault)
        driver.start()
        driver.eval(state_script("work-doc", vid))
        time.sleep(1.5)
        open_menu(driver)
        for preset in PRESETS:
            for theme in THEMES:
                for tier in CONTRAST_TIERS:
                    set_attrs(driver, preset, theme, tier)
                    if tier != "std":
                        close_menu(driver)
                        open_menu(driver)
                    driver.eval(FREEZE)
                    time.sleep(0.35)
                    samples = json.loads(driver.eval(SAMPLE_JS))
                    driver.eval(HIDE)
                    time.sleep(0.25)
                    shot = driver.capture()
                    driver.eval(UNHIDE)
                    for s in samples:
                        if s.get("missing"):
                            continue
                        bg = median_color(shot, s["rect"])
                        if bg is None:
                            continue
                        value = ratio(parse_rgb(s["fg"]), bg)
                        row = {"preset": preset, "theme": theme, "blur": tier,
                               "tier": s["tier"], "label": s["label"], "kind": s["kind"],
                               "fg": s["fg"], "bg": "rgb(%d, %d, %d)" % tuple(bg),
                               "ratio": round(value, 2)}
                        results.append(row)
                        key = (s["tier"], s["label"])
                        if key not in worst or value < worst[key]["ratio"]:
                            worst[key] = row
                        if value < LIMIT:
                            failures.append(row)
    finally:
        try:
            driver.stop()
        except Exception:
            pass
        shutil.rmtree(work, ignore_errors=True)

    json.dump({"threshold": LIMIT, "samples": results, "worst": list(worst.values()),
               "failures": failures},
              open(OUT / "contrast.json", "w", encoding="utf-8"), ensure_ascii=False, indent=2)
    log("contrast samples: %d, failures: %d" % (len(results), len(failures)))
    for key in sorted(worst):
        log("  worst %-7s %-24s %5.2f  (%s / %s / %s)" % (
            key[0], key[1], worst[key]["ratio"], worst[key]["preset"], worst[key]["theme"], worst[key]["blur"]))
    for row in failures[:20]:
        log("  FAIL %s %s %s %s %.2f" % (row["label"], row["preset"], row["theme"], row["blur"], row["ratio"]))
    return failures


FOCUS_JS = """
(() => {
  const target = document.querySelector(%s);
  if (!target) return JSON.stringify({ missing: true });
  const old = document.getElementById('m10-focus-probe');
  if (old) old.remove();
  const r = target.getBoundingClientRect();
  const w = 120, h = 36;
  const left = Math.max(8, Math.round(r.left + r.width / 2 - w / 2));
  const top = Math.max(48, Math.round(r.top + r.height / 2 - h / 2));
  const p = document.createElement('div');
  p.id = 'm10-focus-probe';
  // The shipped focus declaration (tokens.css :focus-visible) on a transparent
  // probe, so the ring is measured against whatever material is behind it.
  p.style.cssText = 'position:fixed;left:' + left + 'px;top:' + top + 'px;width:' + w + 'px;height:' + h + 'px;'
    + 'border-radius:8px;background:transparent;outline:2px solid var(--raw-accent);outline-offset:2px;'
    + 'pointer-events:none;z-index:9999';
  document.body.appendChild(p);
  return JSON.stringify({ rect: [left, top, w, h], color: getComputedStyle(p).outlineColor });
})()
"""

FOCUS_TARGETS = [
    ("panel", ".vault-sidebar"),
    ("card", "[data-testid=overview-skills]"),
    ("strong", ".agent-picker"),
]


def focus_checks():
    """N1 (T11 review): the 2px focus ring must stay visible over all four blur
    tiers. A probe carrying the shipped outline declaration is placed on the
    panel, a card and the strong-tier menu; the ring colour is compared with the
    material just outside it, from a real screenshot (WCAG 1.4.11 wants 3.0 for
    non-text UI)."""
    work = Path(tempfile.mkdtemp(prefix=".m10-focus-", dir=ROOT))
    vault = work / "vault"
    subprocess.run(["go", "run", "./cmd/genvault", "-kind", "typical", "-out", str(vault)],
                   cwd=ROOT, check=True, stdout=subprocess.DEVNULL)
    driver = Driver(work, work / "app", work / "local", work / "home", vault)
    results = []
    try:
        vid = seed(work / "app", vault)
        driver.start()
        for tier in TIERS:
            for theme in THEMES:
                set_attrs(driver, "orange", theme, tier)
                # work state: the card target exists; menu: the strong target exists
                driver.eval(state_script("work", vid))
                time.sleep(1.0)
                open_menu(driver)
                driver.eval(FREEZE)
                time.sleep(0.3)
                shot = driver.capture()
                for name, selector in FOCUS_TARGETS:
                    raw = driver.eval(FOCUS_JS % json.dumps(selector))
                    data = json.loads(raw)
                    if data.get("missing"):
                        continue
                    driver.eval(FREEZE)
                    time.sleep(0.2)
                    shot = driver.capture()
                    left, top, width, height = data["rect"]
                    ring = parse_rgb(data["color"])
                    # background samples: a band 8px outside the ring's outer edge
                    band = shot.crop((max(0, left - 10), max(0, top - 10),
                                      min(shot.width, left + width + 10), min(shot.height, top + height + 10)))
                    pixels = list(band.getdata())
                    # pixels outside the probe rect are background; inside is material
                    outer = []
                    for y in range(band.height):
                        for x in range(band.width):
                            gx, gy = band.width, band.height
                            if x < 10 or y < 10 or x >= gx - 10 or y >= gy - 10:
                                outer.append(pixels[y * band.width + x])
                    if not outer:
                        continue
                    channels = list(zip(*outer))
                    bg = [sorted(c)[len(c) // 2] for c in channels]
                    value = ratio(ring, bg)
                    results.append({"tier": name, "blur": tier, "theme": theme,
                                    "ratio": round(value, 2), "ring": data["color"],
                                    "bg": "rgb(%d, %d, %d)" % tuple(bg)})
                    if name == "strong":
                        # keep this one: the saved shot shows the ring on the menu
                        driver.capture().save(OUT / ("focus-%s-%s.png" % (tier, theme)))
                        break
                driver.eval("(() => { const p = document.getElementById('m10-focus-probe'); if (p) p.remove(); return true; })()")
                close_menu(driver)
    finally:
        try:
            driver.stop()
        except Exception:
            pass
        shutil.rmtree(work, ignore_errors=True)
    json.dump(results, open(OUT / "focus.json", "w", encoding="utf-8"), ensure_ascii=False, indent=2)
    worst = min(results, key=lambda r: r["ratio"]) if results else None
    log("focus samples: %d, worst: %s" % (len(results), worst))
    return results


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    only = None
    if "--only" in sys.argv:
        only = sys.argv[sys.argv.index("--only") + 1]
    global STATE_FILTER
    if "--states" in sys.argv:
        STATE_FILTER = sys.argv[sys.argv.index("--states") + 1].split(",")
    if only in (None, "shots"):
        take_shots()
    if only == "focus":
        focus_checks()
    failures = []
    if only in (None, "contrast"):
        failures = sample_contrast()
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
