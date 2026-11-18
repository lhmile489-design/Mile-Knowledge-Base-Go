# R-1 文件监听方案实测

阶段 07 风险验证 R-1（`docs/技术设计.md` 第 7 节）。验证「大」知识库（1 万 md + 5000 文件代码目录）下三种文件监听方案是否可用。

## 环境

- 机器：Intel Core Ultra 7 155H，31.5 GB RAM，Windows 11 amd64
- 工具链：go1.27.3 windows/amd64
- 依赖：`github.com/egoist/mygo v0.10.0`、`github.com/fsnotify/fsnotify v1.10.1`（模块缓存，无网络下载）
- 样本：本 spike `sample.go` 生成，固定种子 `rand.NewSource(20261118)`，布局可复现：
  - `notes/`：10000 个 `.md`，分布在 300 个目录（深度到 6，约 1/3 目录名含中文）
  - `code-repo/`：`src*` 3000 文件 + `node_modules/` 1000 文件 + `.git/` 1000 文件（这两个子树按 PRD 必须忽略）
  - `.mile-kb/`：库内应用目录
  - 合计 **15001 个文件、360 个目录**（非忽略目录约 311 个）

## 运行命令

```bash
export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"
cd spikes/r1-watch

# 一次性生成样本（生成后删除即可重测；不删则三方案共用同一份）
go run . -mode gen  -vault "%TEMP%/r1-vault"

go run . -mode a -vault "%TEMP%/r1-vault" -quiet   # mygo.WatchFiles 普通模式 MaxEntries=65536
go run . -mode b -vault "%TEMP%/r1-vault" -quiet   # mygo.WatchFiles HintsOnly + Go 侧目录 diff
go run . -mode c -vault "%TEMP%/r1-vault" -quiet   # fsnotify 逐目录 Add
```

`-timeout` 默认 10 分钟总看门狗，进程必退出；mode a/b 在 `App.OnReady` 里跑完后 `App.Quit()`。

## 结果（同一台机、同一样本、各跑一次）

| 指标 | a) mygo 普通模式 MaxEntries=65536 | b) mygo HintsOnly + Go diff | c) fsnotify 逐目录 |
| --- | --- | --- | --- |
| 建立监听耗时 | **7735 ms** | **1.65 ms** | **86.9 ms**（311 个句柄） |
| 建立后 Go heap | 5.4 MiB | 3.3 MiB（+自有快照，见下） | 22.1 MiB |
| 建立后进程 RSS | 56.8 MiB | 19.9 MiB | 33.4 MiB |
| 空闲 30s CPU | 0.05% | 0.05% | 0.00% |
| S1 原子写延迟 p50 / p95 / max（n=50） | 53.7 / 55.5 / 87.4 ms | 51.8 / 54.9 / 70.3 ms | **1.70 / 2.77 / 5.28 ms**（无去抖） |
| S1 漏报 | 0 | 0 | 0 |
| S2 burst 1000 文件全部识别 | 1869 ms，0 丢失 | 1490 ms，0 丢失 | 2055 ms，0 丢失 |
| S3 mkdir+file / 目录改名 / 删目录 | 全部收到 | 全部收到 | 全部收到（含动态 Add 新目录） |
| S4 node_modules 内写文件产生事件 | 0（Ignore 直接不入树） | 0 | 0（逐目录 Add 本就跳过） |

## 读数说明（避免误读）

1. **S1 延迟口径不同**：a/b 的 ~52–55ms 里含 mygo 默认 `Debounce=50ms`（可调）；fsnotify 的 1.7ms 是「第一条原始事件到达」，没有合并去抖。真实产品里 fsnotify 方案也要自己去抖，最终用户感知差距不会到 30 倍。两档都可满足 PRD 「外部改动同步 ≤1s」。
2. **S2 的 1.4–2.1s 里大头是写 1000 个文件本身**（note 里 write 阶段 1.36–2.04s），不是监听瓶颈；三方案都在写完后立刻集齐 1000 条路径，**零丢失**，满足 R-1「外部改 1000 文件 1 秒级同步、最终一致」。
3. **建立耗时**：a 的 7.7s 是 mygo 普通模式**首次全量遍历 + stat 15000 条目建快照**（`watch_scan.go` `start()`），不是 ReadDirectoryChangesW 慢——Windows 端整树只用 1 个句柄（`Subtree`）。这是一次性成本，发生在打开知识库时。
4. **MaxEntries 允许调大**：`MaxEntries=65536` 被接受（只校验非负），普通模式 15001 文件 + 360 目录 ≈ 15361 条目，在 65536 内。默认 8192 确实会爆（`ErrWatchLimit`）。该上限管的是快照条目数，不是句柄数——Windows 单句柄监听整树，所以调大只多花内存和首次遍历时间。
5. **HintsOnly 的 3.3MiB heap 不含「我们的快照」成本**——`vaultSnap` 也在 Go 侧存 15000+ 条 `{path,mtime,size}`，量级与普通模式自家快照相同；HintsOnly 省的是 mygo 内部的重复快照和每次变更的全条目 stat，不是省了「记文件状态」这件事本身。

## 关键实现事实（源码级）

- mygo Windows 后端 `internal/windows/watch.go`：`ReadDirectoryChangesW` 每目录 1 句柄 + 64KiB 缓冲；**根目录 `Subtree=true` 单句柄即可覆盖整树**（`watch_scan.go:928` / `watch_hints.go:270`）。超限的是 `MaxEntries` 快照条目数，可调。
- mygo 溢出兜底：`raw` 通道满 → `spill`（上限 MaxEntries 条）→ 溢出后 `overflow` → 全量重扫（`watch.go:251-256`），即「宁可重扫也不丢」。S2 零丢失与此一致。
- fsnotify v1.10.1 **公开 API 没有递归**：`path\...` 语法被 `enableRecurse`（`fsnotify.go:503`，仅测试置 true）挡住。本 spike 用 `probe/` 实测：`Add(root\...)` 不报错但只收到 root 直接子项的 WRITE（事件名 `root\...\notes`，bSubtree=false）。因此 c 方案只能逐目录 `Add`（311 个句柄），且**新建目录要自己补 `Add`**（已在事件循环里实现，S3 通过）。
- fsnotify 缓冲默认 64KiB（`WithBufferSize` 可调）；突发时 `ErrEventOverflow` 可能上 `Errors` 通道，本项目只统计到 `Events` 侧无丢失。

## 结论（建议，最终由用户定）

**推荐方案 a：mygo.WatchFiles 普通模式 + `Ignore(node_modules/.git)` + `MaxEntries` 调大到 65536（或 32768×2 余量）。**

理由：
- 精确到文件的事件（`FileEvent{Op,Path,OldPath,IsDir}`），索引/树更新不需要再 diff；
- 自带 `Debounce`（50ms）+ `MaxDelay`（500ms）合并突发，天然满足「批量变更不阻塞」（P-1）；
- `MaxEntries` 可调已实测通过，万级库安全；
- 溢出会自动全量重扫，不丢；
- 单一依赖（已用 mygo），不引第二套监听。

代价：打开知识库时 ~7.7s 首次快照（后台 goroutine 做，不阻塞 UI；PRD 允许「索引在后台建立」）。

**方案 b（HintsOnly）适用场景**：如果 a 的首次 7.7s 快照被证明不可接受、或库远大于 1.5 万条目，HintsOnly 只数目录（~360 < 8192 默认上限），建立几乎零成本，配合 Go 侧按 hint 目录做增量 stat diff 即可。需要我们自己写 diff 逻辑（本 spike `vaultSnap` 已示范，~150 行）。

**方案 c（fsnotify）不建议作为首选**：Windows 无递归 API，要逐目录 `Add`（311 句柄，句柄虽不重但多一层管理），新建目录要补 `Add`，事件无去抖要自己做，且是第二依赖。仅在「不用 mygo」或 a/b 都失败时作兜底。

## 建议参数

- `Recursive: true`，`Ignore`：目录名 `node_modules`/`.git` 返回 true（`.mile-kb` 按 FR-AREA 要显示，监听不排除、索引层再过滤）。
- `MaxEntries`：普通模式按「预期条目 ×4」取，65536 对本样本有 4 倍余量；HintsOnly 保持默认 8192 即可（只数目录）。
- `Debounce`：50ms 默认即可；批量写入期 `MaxDelay` 500ms 已把 S2 收成合理批。

## 剩余风险 / 未测项

- **首次快照 7.7s 在机械硬盘或更大库上可能更长**；样本是 SSD。真实用户库需复测。
- **网络盘 / OneDrive 占位文件**未测：`sameDirectory` 会拒绝 junction/placeholder，行为需另验。
- **S2 期间 UI 响应**：本 spike 无窗口，只验证了「事件不丢、CPU 闲」；「界面主线程 ≤200ms」要在阶段 08 真窗口里复测。
- **`.mile-kb` 内自身写入**（索引落盘）会触发事件回环，需要在业务层忽略（本次未模拟持续回写）。
- fsnotify 的 `ErrEventOverflow` 在更大突发下可能出现，本样本 1000 未触发；如选 c 需自测更大 burst。

## 复现与清理

- 样本目录本次生成于 `C:\Users\LHmile\AppData\Local\Temp\r1-vault`，测后应删除（约 30MB）。
- probe：`go run ./probe -vault <dir>` 单文件验证 fsnotify 递归不可用。
- 代码：`spikes/r1-watch/{main,sample,harness,cpu_windows,mygo_modes,fsnotify_mode}.go`、`probe/probe.go`。
