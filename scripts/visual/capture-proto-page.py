#!/usr/bin/env python3
"""Capture the skills-page prototype states (PRD section 17 draft, 2026-11-24).

Takes design/r2/skills-page.html?state=<name>&theme=<theme>&shot=1&motion=off
in headless Edge (Chrome as fallback) and writes the screenshots into
design/r2/shots-page/. Motion is turned off so the staggered entry animation
never leaves half-drawn cards in a still image; the motion itself is reviewed
in the live prototype.

Usage (from the repo root):
  python -I scripts/visual/capture-proto-page.py
"""

import subprocess
import sys
import tempfile
import time
from pathlib import Path

from PIL import Image

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
PROTO = ROOT / "design" / "r2" / "skills-page.html"
SHOTS = ROOT / "design" / "r2" / "shots-page"
STATES = ["kb", "kb-select", "tools", "distribute", "uninstall", "novault", "scanning", "search-empty"]
THEMES = ["light", "dark"]
NARROW = [("kb", 960, 720), ("uninstall", 960, 720)]
BROWSERS = [
    Path(r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"),
    Path(r"C:\Program Files\Microsoft\Edge\Application\msedge.exe"),
    Path(r"C:\Program Files\Google\Chrome\Application\chrome.exe"),
]

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")


def browser():
    for path in BROWSERS:
        if path.exists():
            return path
    raise RuntimeError("no Edge or Chrome found")


def shoot(exe, profile, query, out, width, height):
    url = f"{PROTO.as_uri()}?{query}&shot=1&motion=off"
    if out.exists():
        out.unlink()
    subprocess.run(
        [
            str(exe),
            "--headless=new",
            "--hide-scrollbars",
            "--no-first-run",
            "--no-default-browser-check",
            f"--user-data-dir={profile}",
            f"--window-size={width},{height}",
            "--virtual-time-budget=1500",
            f"--screenshot={out}",
            url,
        ],
        check=True,
        capture_output=True,
        timeout=120,
    )
    # Edge returns before the renderer writes the file; poll until it settles.
    deadline = time.time() + 60
    last = -1
    while time.time() < deadline:
        if out.exists():
            size_now = out.stat().st_size
            if size_now > 0 and size_now == last:
                break
            last = size_now
        time.sleep(0.3)
    time.sleep(0.3)
    with Image.open(out) as image:
        size = image.size
    if size != (width, height):
        raise RuntimeError(f"{out.name}: unexpected size {size}")
    print(f"  [captured] {out.name} {size[0]}x{size[1]}", flush=True)


def main():
    if not PROTO.exists():
        raise RuntimeError(f"missing prototype: {PROTO}")
    exe = browser()
    SHOTS.mkdir(parents=True, exist_ok=True)
    profile = tempfile.mkdtemp(prefix="mkb-proto-page-")
    for state in STATES:
        for theme in THEMES:
            shoot(exe, profile, f"state={state}&theme={theme}", SHOTS / f"{state}-{theme}.png", 1440, 900)
    for state, width, height in NARROW:
        shoot(exe, profile, f"state={state}&theme=light&w={width}&h={height}", SHOTS / f"{state}-narrow-light.png", width, height)
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:  # noqa: BLE001
        import traceback

        traceback.print_exc()
        sys.exit(1)
