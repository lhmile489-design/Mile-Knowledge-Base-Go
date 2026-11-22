#!/usr/bin/env python3
"""Capture the M7 restore states from the prototype (T00, 2026-11-22).

Takes design/r2/skills-panel.html?state=<name>&theme=<theme>&shot=1 in
headless Edge (Chrome as fallback) at 1440x900 and writes the ten prototype
screenshots into design/r2/shots-m7/. T08 replaces them with real window
screenshots, the same way capture-m6.py did for M6.

Usage (from the repo root):
  python -I scripts/visual/capture-proto-m7.py
"""

import subprocess
import sys
import tempfile
import time
from pathlib import Path

from PIL import Image

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
PROTO = ROOT / "design" / "r2" / "skills-panel.html"
SHOTS = ROOT / "design" / "r2" / "shots-m7"
STATES = [
    "restore-banner",
    "restore-preview",
    "restore-running",
    "restore-result",
    "restore-unreadable",
]
THEMES = ["light", "dark"]
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


def main():
    if not PROTO.exists():
        raise RuntimeError(f"missing prototype: {PROTO}")
    exe = browser()
    SHOTS.mkdir(parents=True, exist_ok=True)
    profile = tempfile.mkdtemp(prefix="mkb-proto-")
    for state in STATES:
        for theme in THEMES:
            url = f"{PROTO.as_uri()}?state={state}&theme={theme}&shot=1"
            out = SHOTS / f"{state}-{theme}.png"
            if out.exists():
                out.unlink()
            subprocess.run(
                [
                    str(exe),
                    "--headless=new",
                    "--disable-gpu",
                    "--hide-scrollbars",
                    "--no-first-run",
                    "--no-default-browser-check",
                    f"--user-data-dir={profile}",
                    "--window-size=1440,900",
                    f"--screenshot={out}",
                    url,
                ],
                check=True,
                capture_output=True,
                timeout=120,
            )
            # Edge returns before the renderer writes the file; poll for it.
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
            if size != (1440, 900):
                raise RuntimeError(f"{out.name}: unexpected size {size}")
            print(f"  [captured] {out.name} {size[0]}x{size[1]}", flush=True)
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:  # noqa: BLE001
        import traceback

        traceback.print_exc()
        sys.exit(1)
