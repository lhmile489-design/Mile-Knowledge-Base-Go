#!/usr/bin/env python3
"""M10 T00: sample text contrast on glass from real WebView2 screenshots.

Each probe file describes text elements (computed colour + bounding box) for
one state / blur level / theme. The matching .bg.png was captured with all
text made transparent, so the median colour inside a text box is the real
effective background (blur + colour field + every translucent layer). The
ratio is the WCAG contrast between the computed text colour and that
background.

Also reports pairwise image differences between the blur levels, so "can the
three levels be told apart in WebView2" has a number behind it.
"""
import json
import sys
from pathlib import Path

from PIL import Image


def parse_color(value):
    value = value.strip()
    if value.startswith("rgb"):
        inner = value[value.find("(") + 1:value.rfind(")")]
        parts = [p for p in inner.replace("/", " ").replace(",", " ").split() if p]
        return tuple(float(p) for p in parts[:3])
    if value.startswith("color(srgb"):
        inner = value[value.find("srgb") + 4:value.rfind(")")].replace("/", " ").split()
        return tuple(float(p) * 255.0 for p in inner[:3])
    raise ValueError("unsupported colour " + value)


def luminance(rgb):
    def channel(c):
        c = c / 255.0
        return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4

    r, g, b = (channel(v) for v in rgb)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def contrast(fg, bg):
    a, b = luminance(fg), luminance(bg)
    hi, lo = max(a, b), min(a, b)
    return (hi + 0.05) / (lo + 0.05)


def median_patch(img, box):
    x0, y0, x1, y1 = box
    px = img.load()
    values = []
    for y in range(y0, y1):
        for x in range(x0, x1):
            values.append(px[x, y][:3])
    if not values:
        raise ValueError("empty patch")
    n = len(values)
    return tuple(sorted(v[i] for v in values)[n // 2] for i in range(3))


def main():
    work = Path(sys.argv[1])
    shots = Path(sys.argv[2])
    out = Path(sys.argv[3])
    results = []
    for probe_path in sorted(work.glob("*.probe.json")):
        tag = probe_path.name[: -len(".probe.json")]
        probe = json.loads(probe_path.read_text(encoding="utf-8"))
        bg_path = work / (tag + ".bg.png")
        if not bg_path.exists():
            continue
        img = Image.open(bg_path).convert("RGB")
        scale = img.width / float(probe["vw"])
        rows = []
        for sample in probe["samples"]:
            if sample.get("missing"):
                rows.append({"name": sample["name"], "missing": True})
                continue
            rect = sample["rect"]
            x0 = max(1, int(round(rect["x"] * scale)) + 2)
            y0 = max(1, int(round(rect["y"] * scale)) + 2)
            x1 = min(img.width - 1, int(round((rect["x"] + rect["w"]) * scale)) - 2)
            y1 = min(img.height - 1, int(round((rect["y"] + rect["h"]) * scale)) - 2)
            frac = float(sample.get("fracX") or 1.0)
            if frac < 1.0:
                x1 = max(x0 + 1, x0 + int((x1 - x0) * frac))
            if x1 <= x0 or y1 <= y0:
                rows.append({"name": sample["name"], "missing": "empty-box"})
                continue
            bg = median_patch(img, (x0, y0, x1, y1))
            fg = parse_color(sample["color"])
            ratio = contrast(fg, bg)
            rows.append(
                {
                    "name": sample["name"],
                    "tier": sample.get("tier"),
                    "fg": sample["color"],
                    "bg": "#%02x%02x%02x" % tuple(int(round(v)) for v in bg),
                    "ratio": round(ratio, 2),
                }
            )
        results.append(
            {
                "tag": tag,
                "state": probe.get("state"),
                "theme": probe.get("theme"),
                "blur": probe.get("blur"),
                "scale": round(scale, 3),
                "samples": rows,
            }
        )

    # Pairwise differences between blur levels on the same state + theme.
    diffs = []
    for state in ("work-doc", "menu"):
        for theme in ("light", "dark"):
            imgs = {}
            for blur in ("off", "low", "std", "high"):
                path = shots / f"{state}-{blur}-{theme}.png"
                if path.exists():
                    imgs[blur] = Image.open(path).convert("RGB")
            levels = sorted(imgs, key=lambda v: ("off", "low", "std", "high").index(v))
            for i in range(len(levels)):
                for j in range(i + 1, len(levels)):
                    a, b = imgs[levels[i]], imgs[levels[j]]
                    if a.size != b.size:
                        continue
                    pa, pb = a.load(), b.load()
                    total = 0
                    strong = 0
                    n = 0
                    for y in range(0, a.height, 2):
                        for x in range(0, a.width, 2):
                            va, vb = pa[x, y], pb[x, y]
                            d = abs(va[0] - vb[0]) + abs(va[1] - vb[1]) + abs(va[2] - vb[2])
                            total += d
                            if d > 24:
                                strong += 1
                            n += 1
                    diffs.append(
                        {
                            "state": state,
                            "theme": theme,
                            "a": levels[i],
                            "b": levels[j],
                            "mad": round(total / (3.0 * n), 2),
                            "pixels_gt8": round(100.0 * strong / n, 2),
                        }
                    )

    payload = {"samples": results, "blurDiffs": diffs}
    out.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(payload, ensure_ascii=False))


if __name__ == "__main__":
    main()
