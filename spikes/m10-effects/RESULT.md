# M10 T00 基线与真窗口 spike（2026-11-27）

> 结论摘要：
> 1. **1.7.0 最小化 CPU 基线**：300 秒窗口内进程树合计 **0.594 s / 0.625 s**（两次），约单核 0.20%；真实 APPDATA 前后指纹一致。建议 `idleCpuRatio` 以 0.625 s 为基线、门槛 ≤ 1.2 倍（即 300 s 内 ≤ 0.75 s），待用户确认后接入 T09。
> 2. **`kind=typical` 1.7.0 基线**：冷启动 870 ms、检索 p95 1.2 ms、历史首帧 66.1 ms、流式慢帧 0、按键 p90 8.1 ms，`perf-gate --kind typical` 通过（exit 0）。
> 3. **Q-M10-2：当前 DESIGN 15.2 的「强」档在 WebView2 里与「标准」几乎不可区分**（同帧 MAD 0.16–1.7，差异 >8 的像素 0.00%）。原因：原型把 `--field-k` 直接用作 `.field { opacity }`，1.3 被钳到 1，色场强度实际没有提升；模糊半径在柔和色场上也看不出。已实测四个候选，推荐候选 E（色块更强 + 面板更透 + 40px/2.0），work-doc 浅色 MAD 1.82 / 6.15% 像素（约等于 off→std 的可见度），对比度最低仍为 4.53。**按 Q-M10-2 先报告，DESIGN 15.2/15.3 待用户确认后再改。**

## 1. 基线与环境

| 项 | 值 |
| --- | --- |
| 产品代码 | `7307aad`（1.7.0）；此后仅文档提交（`7a5973d`、`cfeee6f`、`43ae4d7`），`git diff 7307aad HEAD -- . ':!docs' ':!design' ':!*.md'` 只有本 spike 的截图脚本 |
| 生产构建 | `bun run build`，产出 `build/windows-amd64/Mile Knowledge Base.exe`，FileVersion **1.7.0** |
| e2e 构建 | `GOFLAGS=-tags=e2e MYGO_INSPECTOR=1 bun run build`（自动化驱动用） |
| 显示器 | 约 200 Hz（M8 spike 实测；本次截图 devicePixelRatio ≈ 1.75） |
| 隔离 | 全部使用临时 APPDATA / LOCALAPPDATA / USERPROFILE / HOME 与临时 genvault；真实 APPDATA 目录前后指纹一致（`REAL_APPDATA_UNCHANGED=yes`，两次） |

## 2. 最小化 5 分钟 CPU 基线（AC-MAT-9 / Q-M10-3）

方法：`cpu/run-cpu.sh` 用生产 1.7.0 + 临时环境启动，等主窗口出现后 `ShowWindow(SW_MINIMIZE)`，稳定 60 s，再对「应用 + 全部 WebView2 子进程」的 `TotalProcessorTime` 求 300 s 增量。

| 轮次 | 窗口 | 进程数 | CPU 合计 | 单核占比 | 文件 |
| --- | --- | --- | --- | --- | --- |
| 1 | 300 s | 7 | **0.594 s** | 0.198% | `cpu/cpu-baseline.json` |
| 2 | 300.02 s | 7 | **0.625 s** | 0.208% | `cpu/cpu-baseline-2.json` |

- 两次相差 5%，构成后续比较的噪声带。
- 进程树含 `Mile Knowledge Base` 1 个 + `msedgewebview2` 6 个（含渲染与 GPU 子进程）。
- 真实 `%APPDATA%\Mile Knowledge Base` 的文件清单、大小与 mtime 在两次运行前后完全一致。

**Q-M10-3 建议**：新增 `idleCpuRatio = idleCpuSeconds / 0.625`，门槛 **≤ 1.2**（300 s 内 ≤ 0.75 s）。取两次中的较大值作分母，容忍基线噪声；连续色场漂动或每帧合成都会远超该值，能覆盖 AC-MAT-9「色场停止」的意图。属新门禁，按计划需用户确认后才写入 `perf-gate.py`。

## 3. `kind=typical` 1.7.0 基线

| 指标 | 实测 | 门槛 | 来源 |
| --- | --- | --- | --- |
| coldStartMs | 870 | ≤ 1500 | `perf.sh`（同一方法下 `perf-m1.sh` 另测 801） |
| searchP95Ms | 1.2 | ≤ 100 | `perf-m1.sh` 典型库 50 次 |
| viewerFirstFrameMs | 66.1 | ≤ 100 | `perf-ai.sh` |
| slowFrames（>20 ms） | 0 / 551 帧 | 0 | `perf-ai.sh`，最长单帧 16.7 ms |
| keyLatencyP90Ms | 8.1 | ≤ 16 | `perf-ai.sh`，91 个样本 |

- 指标文件 `perf/typical-1.7.0.json`，`python -I scripts/perf-gate.py --kind typical --input ...` → `perf-gate ok (typical): 5 metrics within target`，exit 0。
- 参考（非门禁）：大库冷启动 663–866 ms；大库检索 p95 48.8 ms（门槛 200）；典型索引就绪 159 ms、大库 6187 ms；1000 次外部写长任务 0；`typical_reader_ms` 沿用脚本既有空值（M0 起未单独计时）。
- 原始日志：`perf/perf-1.7.0.txt`、`perf/perf-m1-1.7.0.txt`、`perf/perf-ai-1.7.0.txt`（`*.log` 被仓库忽略，改存 `.txt`）。

## 4. 真窗口模糊档位与玻璃底对比度（Q-M10-2）

方法：`blur/run.sh` 把已审核原型 `design/r2/global-material.html`（+ `src/styles/tokens.css`）生成到 spike 的 `web/`，在 mygo + WebView2 真窗口（1440×900，dpr ≈ 1.75）打开；`motion=off` 固定色场，逐档设置 `data-theme` / `data-blur`，用 automation `capture` 取 PNG。对比度取样：对每处文字先读计算色与包围盒，再把全部文字设为透明后重拍，取包围盒内像素中位色作为真实有效背景，按 WCAG 计算比值（`blur/sample.py`）。

### 4.1 三档模糊的视觉差异

同一状态、同一主题、同一进程内两两比较（隔像素采样，MAD = 三通道平均绝对差，>8 记为可见差异）：

| 状态 / 主题 | off→low | off→std | low→std | low→high | **std→high** |
| --- | --- | --- | --- | --- | --- |
| work-doc 浅 | 1.34 / 2.54% | 1.94 / 7.96% | 0.79 / 2.69% | 1.26 / 4.83% | **0.48 / 0.00%** |
| work-doc 深 | 3.20 / 20.81% | 4.11 / 20.82% | 0.92 / 2.55% | 0.99 / 4.38% | **0.16 / 0.00%** |
| menu 浅 | 3.28 / 3.83% | 5.72 / 27.16% | 2.63 / 3.46% | 4.29 / 12.91% | **1.70 / 0.00%** |
| menu 深 | 8.90 / 68.02% | 11.73 / 68.49% | 2.87 / 5.86% | 3.33 / 11.54% | **0.70 / 0.00%** |

- **标准与强在任何组合里都没有一个像素的差异超过 8/255**，逐张看截图也分不出（`shots/work-doc-std-light.png` 与 `shots/work-doc-high-light.png`）。
- 根因（原型代码）：`.field { opacity: var(--field-k) }`，`high` 的 `--field-k: 1.3` 被 opacity 上限钳到 1；色块本身不随档位变化。模糊半径 18→28px 作用在已经很柔的色场上，几乎不产生像素差异。这也说明 DESIGN 15.2 表里「色场强度 `--field-k` 1.3」按现原型实现是空操作。

### 4.2 对比度取样（当前档位）

16 个组合（2 状态 × 2 主题 × 4 档）× 每组合 3–8 处文字，全部比值见 `blur/result.json`。最低值：

| 组合 | 最低比值 | 位置 |
| --- | --- | --- |
| menu 深（off/low/std/high） | **4.53** | 强档上的强调色文字（橙） |
| menu 浅（四档） | 4.63 | 同上 |
| work-doc 深 | 6.10–6.60 | 卡片强调色 / 面板次级文字 |
| work-doc 浅 | 5.26–7.12 | 卡片强调色 / 面板次级文字 |
| 内衬（不透明） | 14.20–14.95 | 阅读正文、消息流 |

- 门槛 4.5（AC-MAT-5），当前全部通过；最紧的是深色强档强调色 4.53，与 M9 的最差值一致（M9 T10 记录 4.53）。强档文字压在菜单下的**不透明面板**上，色场不透出，因此这一项不随模糊档位变化。
- 焦点环未取样（T10 的 AC-MAT-5 全矩阵负责）。

### 4.3 候选高强档实测（Q-M10-2 的处置依据）

在同一个真窗口里注入候选 CSS 后重拍、重取样（`blur/tune.sh` / `tune-result.json`，截图 `shots/tune-candE-*`）：

| 候选 | 定义（浅 / 深） | work-doc 浅 MAD / >8 | work-doc 深 | menu 浅 | menu 深 | 最低对比度 |
| --- | --- | --- | --- | --- | --- | --- |
| B | 只增强色块（34→50% 等） | 0.89 / 3.79% | 0.40 / 1.07% | 2.85 / 8.67% | 1.55 / 2.22% | 4.53 |
| C | B + 面板 42/36/66%（深 38/32/62） | 1.09 / 4.30% | 0.42 / 1.36% | 3.54 / 12.40% | 1.67 / 3.70% | 4.53 |
| D | 色块 60/40/28%（深 36/32/17）+ 面板 34/28/**62**（深 30/24/**56**）+ 40px、2.0 | 1.82 / 6.15% | 0.85 / 3.69% | 5.98 / 25.14% | 3.32 / 15.26% | 4.53 |
| **E（推荐）** | 同 D，强档 70% / 68% | **1.82 / 6.15%** | 0.85 / 3.69% | 5.95 / 25.14% | 3.30 / 15.26% | **4.53** |

- E 的可见度约为当前 off→std 的同一量级（1.82 vs 1.94 MAD），切档时可感知；菜单上更明显。
- D 与 E 的差别只在强档透明度（62/56 对 70/68），画面几乎相同（强档下方是不透明面板），因此取 E，保留强档的对比度余量。
- 两个候选的对比度最低值仍是 4.53（深色菜单强调色），未被削弱。
- 截图对比：`shots/work-doc-std-light.png`（标准）对 `shots/tune-candE-work-doc-light.png`（候选强档）。

## 5. 局限

- 原型页面固定 1440×900，而窗口客户区约 1440×871（dpr 1.751），截图底部约 29 CSS px 被裁；取样元素都在可见区内。实施后按真界面重拍（T10）。
- 色场在 `motion=off` 下静止；动态漂动下的观感与性能由 T09/T10 覆盖。
- 对比度只覆盖每档 3–8 处代表性文字；完整 6 套主题色 × 浅 / 深 × 三档的 AC-MAT-5 矩阵在 T10。
- CPU 基线是单个程序的 200 Hz 机器结果；`idleCpuRatio` 的比例口径使其对机器差异不敏感，但 T09 复测时应保持同一台机器、同一 5 分钟口径。
- 候选 E 的数值只在本 spike 验证；改 DESIGN 15.2/15.3 后需在 T10 的真界面截图里复核。

## 6. 复现

```bash
# 1.7.0 生产包（CPU 基线）
bun run build
bash spikes/m10-effects/cpu/run-cpu.sh          # 约 6.5 分钟

# e2e 包（性能与截图）
GOFLAGS="-tags=e2e" MYGO_INSPECTOR=1 bun run build
bash scripts/e2e/perf.sh
bash scripts/e2e/perf-m1.sh
bash scripts/e2e/perf-ai.sh
python -I scripts/perf-gate.py --kind typical --input spikes/m10-effects/perf/typical-1.7.0.json

# 模糊档位与对比度
bash spikes/m10-effects/blur/run.sh
bash spikes/m10-effects/blur/tune.sh
```
