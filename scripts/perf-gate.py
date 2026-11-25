#!/usr/bin/env python3
"""G-PERF gate (M3 T12): fail when a measured metrics file misses PRD 10.1.

Usage: python -I scripts/perf-gate.py --kind typical --input metrics.json
Accepted keys: coldStartMs, searchP95Ms, viewerFirstFrameMs, slowFrames,
keyLatencyP90Ms. Unknown keys are ignored. Exit 0 when every present metric is
within its target; exit 1 with the violations printed.
"""
import argparse
import json
import sys

TARGETS = {
    "typical": {
        "coldStartMs": 1500,
        "searchP95Ms": 100,
        "viewerFirstFrameMs": 100,
        "slowFrames": 0,
        "keyLatencyP90Ms": 16,
    },
    "big": {
        "searchP95Ms": 200,
    },
    "skills": {
        "scanP95Ms": 200,
        "opP95Ms": 300,
    },
    "m5": {
        "read50P95Ms": 3000,
        "check20Ms": 2000,
    },
    "m6": {
        "scanBaseP95Ms": 200,
        "scanProjectP95Ms": 300,
        "targetSaveP95Ms": 100,
        "targetRemoveP95Ms": 100,
    },
    "m7": {
        "preview200P95Ms": 300,
        "manifestWriteP95Ms": 50,
        "restore100Ms": 3000,
    },
    "m8": {
        "scrollP95Frames": 1.5,
        "slowFrameRatio": 0.05,
        "enterP95Ms": 300,
        "switchBackMs": 100,
    },
    # M9 (技术设计 20.7). The scroll keys reuse the M8 names and limits: perf-m9
    # measures the same scroll with the detail drawer open.
    "m9": {
        "drawerOpenP95Ms": 150,
        "modalOpenMs": 100,
    },
}

LABELS = {
    "coldStartMs": "冷启动 ms",
    "searchP95Ms": "检索 p95 ms",
    "viewerFirstFrameMs": "历史查看器首帧 ms",
    "slowFrames": "流式 >20ms 帧数",
    "keyLatencyP90Ms": "按键延迟 p90 ms",
    "scanP95Ms": "技能扫描 p95 ms",
    "opP95Ms": "技能操作 p95 ms",
    "read50P95Ms": "50 MB 包读取 p95 ms",
    "check20Ms": "20 组更新检查 ms",
    "scanBaseP95Ms": "M4 基准扫描 p95 ms",
    "scanProjectP95Ms": "10 个项目扫描 p95 ms",
    "targetSaveP95Ms": "登记保存 p95 ms",
    "targetRemoveP95Ms": "登记移除 p95 ms",
    "preview200P95Ms": "还原预览 200 项 p95 ms",
    "manifestWriteP95Ms": "清单写入 2000 项 p95 ms",
    "restore100Ms": "还原 100 个知识库项 ms",
    "scrollP95Frames": "卡片滚动 p95 帧数（相对刷新间隔）",
    "slowFrameRatio": "慢帧比例（> 2R）",
    "enterP95Ms": "进入技能页到卡片可见 p95 ms",
    "switchBackMs": "切回工作台首帧 p95 ms",
    "drawerOpenP95Ms": "打开抽屉到预览可见 p95 ms（渲染缓存未命中）",
    "modalOpenMs": "弹窗打开到第一帧 p95 ms",
}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--kind", choices=sorted(TARGETS), required=True)
    parser.add_argument("--input", required=True)
    args = parser.parse_args()
    with open(args.input, encoding="utf-8") as handle:
        metrics = json.load(handle)
    failures = []
    checked = 0
    for key, limit in TARGETS[args.kind].items():
        if key not in metrics:
            continue
        checked += 1
        value = metrics[key]
        if value > limit:
            failures.append(f"{LABELS[key]}: {value} > {limit}")
    if checked == 0:
        print(f"perf-gate: no known metrics in {args.input}", file=sys.stderr)
        return 1
    if failures:
        print("perf-gate FAILED (" + args.kind + "): " + "; ".join(failures), file=sys.stderr)
        return 1
    print(f"perf-gate ok ({args.kind}): {checked} metrics within target")
    return 0


if __name__ == "__main__":
    sys.exit(main())
