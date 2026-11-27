#!/usr/bin/env python3
"""M10 T09: build a kind=typical metrics file from perf.sh / perf-m1.sh /
perf-ai.sh logs and hand it to scripts/perf-gate.py.

perf.sh and perf-ai.sh are the two scripts that take --effects (AC-MAT-6/7);
perf-m1.sh measures the index, which does not depend on the material tier, so
it is run once and its search p95 is reused for both levels (recorded here so
the metrics file never silently drops a key).

Usage:
  python -I -X utf8 spikes/m10-effects/t09/assemble-typical.py \
      --perf perf-std.txt --m1 perf-m1.txt --ai perf-ai-std.txt --out typical-std.json
"""
import argparse
import json
import re


def one(pattern, text, name, cast=float):
    match = re.search(pattern, text, re.M)
    if not match:
        raise SystemExit(f"missing {name} in the perf logs")
    return cast(match.group(1))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--perf", required=True)
    parser.add_argument("--m1", required=True)
    parser.add_argument("--ai", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    perf = open(args.perf, encoding="utf-8", errors="replace").read()
    m1 = open(args.m1, encoding="utf-8", errors="replace").read()
    ai = open(args.ai, encoding="utf-8", errors="replace").read()

    raw_search = json.loads(re.search(r"^search_typical=(\{.*\})$", m1, re.M).group(1))
    # perf-m1 prints the drive response, whose value is the metric JSON.
    search = json.loads(raw_search["result"]["value"])
    stream = json.loads(one(r"^stream_frames=(\{.*\})$", ai, "stream_frames", str))
    keys = json.loads(one(r"^key_latency=(\{.*\})$", ai, "key_latency", str))

    metrics = {
        "coldStartMs": one(r"^cold_start_typical_ms=(\d+)", perf, "cold start", int),
        "searchP95Ms": search["p95"],
        "viewerFirstFrameMs": one(r"^viewer_first_frame_ms=([0-9.]+)", ai, "viewer first frame"),
        "slowFrames": stream["slow"],
        "keyLatencyP90Ms": keys["p90"],
    }
    json.dump(metrics, open(args.out, "w", encoding="utf-8"))
    print(json.dumps(metrics))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
