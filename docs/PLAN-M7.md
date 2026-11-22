# M7 实施计划：R2 跨机还原分发（分发清单随知识库走）

> 状态：v0.1 草案（2026-11-22），待用户审核；未获批前不实施。
> 本次目标：交付 PRD 第 15 节的 FR-SKM-SYNC（v2.6）。闭环为「机器 A 分发或从 GitHub 直装 → 知识库 `.mile-kb/skill-distributions.json` 自动更新 → 用户自己把知识库同步到机器 B → B 的「工具技能」页提示待还原 → 预览、勾选、还原 → B 的结果与 A 一致（链接指向 B 本机的知识库）」。
> 上一里程碑：docs/PLAN-M6.md（已完成并推送，`92c1ae2`；推送记录 `de3a261`，版本 1.4.0）。下一里程碑：未规划。

## 1. 目标、范围与授权

目标：

1. 清单存储：知识库内 `.mile-kb/skill-distributions.json` 的读入（5 种状态）、逐项校验、按键 upsert / 删除、稳定序列化（数据库设计 3.9）。
2. 清单写入：分发、卸载、GitHub 直装进工具、工具技能更新、收编换链接成功后自动更新清单；清单读不出时停写但操作照常（技术设计 18.3）。
3. 还原判定：按目标键匹配本机内置工具与登记目标，逐项给出新建 / 已一致 / 冲突 / 跳过，算出横幅 N 与「本机有、清单没有」（技术设计 18.4、18.5）。
4. 还原执行：锁内复核后逐项执行；知识库项按清单方式分发，GitHub 项按记录的提交下载安装并写本机来源记录；冲突项只在勾选后备份再覆盖；可取消（技术设计 18.8）。
5. 重建：清单读不出时用本机分发记录与来源记录重写，原文件留 `.corrupt-<时间>` 副本（技术设计 18.6）。
6. 界面：工具页横幅（待还原 / 无法读取）、还原预览子视图、进度、结果、重建确认（UI 稿 9.6、DESIGN 11.7）。
7. 验收：AC-SKM-20–25 自动化（两套临时 APPDATA + 假 HOME 模拟两台机器，本地假 GitHub）；视觉对照；性能门禁。

不做：Git 远端推送 / 拉取、凭据、自动定时同步、zip 安装技能的还原、工具配置文件同步（PRD FR-SKM-SYNC「范围外」）。

授权边界：测试与 e2e 只用临时目录、假 HOME 与本地假 GitHub，不读写真实工具目录、真实知识库与真实应用数据，不访问真实 GitHub；Agent 没有可用工具（PRD 15.4、AC-AI-1）。

## 2. 来源基线与决策点

| 项 | 内容 |
| --- | --- |
| 需求基线 | PRD 第 15 节 FR-SKM-SYNC、AC-SKM-20–25（v2.6，用户 2026-11-22 审核通过）；D-018 |
| 设计基线（本计划一起审） | 技术设计第 18 节、API 第 15 节（M102–M104）、数据库设计 3.9 与 4.6 M7 增补、UI 稿 9.6、DESIGN 11.7 |
| 复用的既有实现 | `skills.ExecuteDistribute`（`internal/skills/distribute.go`）、`GithubClient.DownloadTarball` / `ExtractTarGz` / `StageStore`、`SaveToolSource`、`sources.go` 的知识库文件写法、`opMu`、`logOperation`、E13 进度 |
| 代码基线 | `origin/main` `de3a261`（1.4.0） |
| 已定 | Q-SKM-4 选「清单随知识库走」；登记目标按名称 + 类型匹配；冲突默认跳过；Q-SKM-7–10 按建议（用户 2026-11-22） |

待用户批准时决定（每项都有建议，批准计划即按建议执行，除非另有选择）：

| 编号 | 问题 | 选项 | 建议 |
| --- | --- | --- | --- |
| Q-M7-1 | 收编时勾选「换成链接」要不要写进清单 | (a) 写，作为链接分发项；(b) 不写 | (a)：PRD 说独立技能「先收编再走同一条路」，不写的话这条路在另一台机器上还原不出来 |
| Q-M7-2 | 移除登记（M100）时清单里指向它的项 | (a) 不动；(b) 一起删除 | (a)：移除登记只影响本机，别的机器可能仍在用这个登记 |
| Q-M7-3 | 性能门槛（技术设计 18.9） | (a) 预览 200 项 p95 ≤ 300 ms、清单写入 2000 项 p95 ≤ 50 ms、还原 100 个知识库项 ≤ 3 s，作为阻断门禁；(b) 只记实测不阻断 | (a)：与 M4–M6 一致；实测后若需校准先报告 |
| Q-M7-4 | 还原写日志的操作类型 | (a) 沿用 `distribute` / `install`，detail 写「从清单还原」；(b) 新增 `sync-restore` | (a)：日志页与筛选不用改 |
| Q-M7-5 | 原型与截图 | (a) T00 在 `design/r2/skills-panel.html` 新增 5 个状态、出 10 张原型截图，审核后实施；(b) 只写文字，T08 直接拍真实界面 | (a)：与 M4–M6 一致 |
| B-M7-01 | 版本与安装 | (a) 完成后发 **1.5.0**，打本地安装包并做 1.4.0 → 1.5.0 升级冒烟（安装前单独确认）；(b) 只构建不安装 | (a) |
| B-M7-02 | 整体审查方式 | (a) task-reviewer 独立审查，重点看清单的不可信输入校验、写入点与还原复核；(b) 自审 | (a) |
| B-M7-03 | 真实两台机器的人工测试 | (a) 你有第二台电脑时，在 T11 后亲测（我给出步骤，不由我执行）；(b) 只用 e2e 的两套临时环境模拟 | (b) 作为门禁，(a) 作为可选亲测 |
| Q-M7-6 | 装进工具时，安装结果行显示工具 id（`claude-code → Claude Code：已安装`）而不是技能名；M5 遗留 | (a) 现在修：`SkillOpItem` 增加可选 `name`，服务端装进工具时填候选名，前端优先用它；(b) 记录后放到 T10/T12 审查或后续里程碑 | (a)：纯显示问题、改动小、不涉及行为 | 已确认（2026-11-23）：(a)，T07 补充修复 |

## 3. 任务清单

执行方式：**一次只做一个任务，做完汇报并停下，等用户放行后再做下一个**。状态：待做、进行中、阻塞、待验证、已验证。

| 编号 | 目标 | 修改范围 | 依赖 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| T00 | 原型与契约落盘 | 原型 5 状态与 10 张截图（按 Q-M7-5，`design/r2/shots-m7/`、`scripts/visual/capture-proto-m7.py`）；`internal/contract` 新类型与 M102–M104 签名；`mygo generate` | 用户批准本计划 | 原型截图经用户审核；contract 与绑定生成通过；既有测试不退化 | 待验证（原型 10 张与契约已落盘，等用户审核） |
| T01 | 清单存储 | `internal/skills/manifest.go`（新）、`manifest_test.go` | T00 | 5 种读入状态；逐字段校验（技术设计 18.7）；upsert / 删除按「目标键 + 名称」不区分大小写；稳定排序与序列化；2000 项上限；序列化结果不含绝对路径与用户名（测试断言） | 已验证 |
| T02 | 清单写入钩子 | `internal/service/skills_manifest.go`（新）、`skills_{distribute,uninstall,install_exec,update_exec,adopt}.go`、测试 | T01 | 18.3 每个写入点：成功写、失败不写；目标键从登记表生成（项目用实际子目录）；清单 `unreadable` 时不写但操作成功；`vaultId` 为空或没有打开的知识库时不写；Q-M7-1、Q-M7-2 按批准执行 | 已验证 |
| T03 | 还原判定与预览 | `internal/skills/restore.go`（新）、`internal/service/skills_restore.go`（新，M102）、测试 | T02 | 18.5 判定表逐行有测试；`pending`、`localOnly`、`unmatched`、`warnings` 正确；M102 不联网、不写盘、结果不含绝对路径 | 已验证 |
| T04 | 还原执行 | `skills_restore.go`（M103）、测试（GitHub 用 M5 测试替身） | T03 | 锁内复核（清单变化、已一致、未勾覆盖 → `skipped`）；知识库项链接 / 拷贝 / 联接失败退回拷贝；GitHub 项按 `owner/repo/commit` 分组下载一次、写来源记录；冲突覆盖先备份；取消保留已完成项；日志按 Q-M7-4；清扫描缓存 | 已验证 |
| T05 | 重建清单 | `skills_restore.go`（M104）、`manifest.go`、测试 | T04 | 由本机记录重写；登记已移除的记录跳过并计数；原文件改名 `.corrupt-<时间>`，改名失败不写；`version-unknown` 拒绝 | 已验证 |
| T06 | 前端 | `src/lib/transport.ts`、`src/components/skills/RestoreView.tsx`（新）、`ToolSkillsPage.tsx`、`SkillsPanel.tsx`、`skills-panel.css`、测试 | T05 | 横幅两种形态；预览分组、勾选规则、「需要联网」、本机有清单没有；进度与取消；结果；重建确认；Esc 分层返回；执行后主动重拉 M77 与 M102 | 已验证 |
| T07 | e2e 验收 | `scripts/e2e/ac-m7.sh`（新）、`scripts/test-e2e.sh`（自动收录） | T06 | AC-SKM-20–25；两套临时 APPDATA + 假 HOME；本地假 GitHub；真实工具目录零写入；全量 e2e 回归通过 | 已验证 |
| T08 | 视觉对照 | `design/r2/shots-m7/`、`scripts/visual/capture-m7.py`（新）、UI 稿 9.6.1、DESIGN 11.7 | T06 | 5 状态 × 浅 / 深真实截图替换原型；差异记入 UI 稿；对比度复测 | 已验证 |
| T09 | 性能 | `scripts/e2e/perf-m7.sh`（新）、`scripts/perf-gate.py`（`kind=m7`） | T07 | 按 Q-M7-3 的门槛实测通过；M6 的 `kind=m6` 不退化 | 已验证 |
| T10 | 整体审查（按 B-M7-02） | 审查报告、本计划、受影响代码与测试 | T09 | 阻断 0，需改项已修或经用户接受 | 待做 |
| T11 | 收尾（按 B-M7-01） | README、CLAUDE.md、路线、发布清单、本计划、`mygo.config.ts` | T10 | 1.5.0 安装包与 1.4.0 → 1.5.0 升级冒烟；文档同步；列提交；推送前询问 | 待做 |

依赖顺序 T00 → T11，串行。

## 4. 单任务精确交接（摘要）

> 通用纪律：Go 与 TS 的业务逻辑测试先行；测试只用临时目录、假 HOME 与本地假 GitHub；中文 UI、无 emoji；提交前扫描密钥与真实路径。

- **T00**：contract 里清单的 `target` 用 Go 结构体 + `kind` 字段表达三种形态（mygo 绑定不生成 TS 联合类型时，按 M6 的做法在 `transport.ts` 收窄）；mygo 只为被方法引用的类型生成绑定，M102–M104 的方法签名要同时落盘。
- **T01**：清单在知识库里，按不可信输入处理；复用 `IsValidKbSkillPath`、`IsValidSegment`、`validOrigin`、`ProjectSkillDirs()`、注册表查找。写法照 `sources.go`：`pathguard.Resolve` → 临时文件 → `os.Rename`；`SetEscapeHTML(false)`、2 空格缩进、结尾换行。
- **T02**：M81 卸载与 M96 更新不带 `vaultId`，用 `vault.Registry.Active()`；不改这两个方法的签名。写清单失败不能让原操作返回失败。收编换链接目前不写本机分发记录（扫描能认出联接），M7 也不补写，只写清单。
- **T03**：判定是纯函数，输入是清单项、本机 `DetectedTool` 列表（`skillTools()`）、登记表、分发记录、来源记录与 `Classify` 结果；不联网。未检测到或未启用的内置工具一律 `skip`（M79 本来就拒绝，`skills_distribute.go:124`）。
- **T04**：GitHub 项不调 `Resolve`（清单已有 `commit` 与 `pathCommit`），直接 `DownloadTarball(ref, commit)`；同组共用一个暂存，组结束删暂存。**不依赖后台 goroutine 发出的 E12**（1.3.1 结论），前端执行完主动重拉。
- **T05**：重建只用本机 `state.json` 中 `vaultId` 等于当前知识库的记录与 `skills/sources.json` 中目标位置仍存在的记录；`t-…` 登记 id 要换成「名称 + 类型」，登记已移除的跳过。
- **T06**：结构按 UI 稿 9.6；组件按 DESIGN 11.7；文本输入（如有）用 `.field`；复用 `.skills-subview*`、`.skills-result`、`.install-progress`、`.pill`。
- **T07**：机器 A：临时 APPDATA-A + 假 HOME-A + 知识库 KB-A；分发、GitHub 直装后把 KB-A 整个复制成 KB-B。机器 B：APPDATA-B + HOME-B，登记同名项目；打开 KB-B 还原。冲突场景在 HOME-B 预放内容不同的同名目录；清单损坏场景直接写入冲突标记。

## 5. 验证与验收

| 编号 | 给定条件 | 操作 | 预期结果 | 方式 |
| --- | --- | --- | --- | --- |
| V-MAN-1 | 清单不存在 / 正常 / JSON 坏 / 超 1 MiB / `version` = 2 | 读入 | 依次为 `missing` / `ok` / `unreadable` / `too-large` / `version-unknown` | Go |
| V-MAN-2 | 单项 `skillPath` 越界、`name` 含 `..`、`subdir` 不在清单、`toolId` 不在注册表、`commit` 非 40 位十六进制 | 读入 | 该项忽略并记 warning，其余项保留 | Go |
| V-MAN-3 | 分发到内置、自定义、项目各一 | 读清单原文 | 3 项、排序稳定、不含驱动器号、`\Users\`、当前用户名与 `t-` 登记 id | Go |
| V-HOOK-1 | 18.3 每个写入点 | 成功 / 失败各一次 | 成功改清单，失败不改 | Go |
| V-HOOK-2 | 清单为 `unreadable` | 分发 | 分发成功；清单字节不变；M102 `warnings` 提示 | Go + e2e |
| V-JUDGE-1 | 18.5 的每一行 | M102 | 状态、原因、`selectable`、`defaultSelected` 与表一致 | Go |
| V-EXEC-1 | 预览后在目标位置放入同名目录 | M103 选中该项（未勾覆盖） | `skipped`，目录字节不变 | Go |
| V-EXEC-2 | 两个 GitHub 项同 `owner/repo/commit` | M103 | 只下载一次；两项都装好并有来源记录 | Go |
| V-EXEC-3 | 执行中取消 | M103 | 已完成项保留，未开始项「已取消」，暂存删除 | Go |
| V-REB-1 | 清单坏；本机有 3 条分发、1 条来源记录，其中 1 条分发的登记已移除 | M104 | 写入 3 项、跳过 1 项；`.corrupt-<时间>` 副本存在 | Go |
| V-AGENT-1 | 执行模式全自动 | 让 Agent 写清单或触发还原 | 没有可用工具，清单不变 | Go |
| V-SKM-AC | AC-SKM-20–25 | 见 PRD 15.6 | 全部通过 | e2e |

门禁按技术设计第 10、15–18 节；实际命令按 README 第 5 节；每个任务的实际结果记在第 8 节账本。

## 6. 性能目标

| 指标 | 目标（Q-M7-3 建议） | 依据 |
| --- | --- | --- |
| M102 预览，清单 200 项，25 个内置工具 + 10 个登记目标 | p95 ≤ 300 ms | M6 项目扫描 p95 243 ms 的同量级；判定只读文件系统 |
| 清单写入（一次分发后的 upsert），清单 2000 项 | p95 ≤ 50 ms | 一个约 600 KB 的 JSON 读写 |
| M103 还原 100 个知识库项（链接） | 总耗时 ≤ 3 s | 每项一次联接创建 + 记录写入 |
| M6 基准（`kind=m6` 四项） | 不退化 | G-PERF |

GitHub 项受网络影响，不设门槛，只记实测。实测后按 G-PERF 阻断口径校准；超目标先报告再决定。

## 7. 协作、审查与执行方式

- 一次一个任务，汇报后停下；整份计划的批准不等于各任务放行。
- 不用子代理或并行会话执行编号任务；T10 的审查方式按 B-M7-02。
- 每个任务结束提交到本地（不推送）；推送只在 T11 列清单后由用户决定。
- 安装包安装（B-M7-01）执行前单独征得同意；安装前确认没有运行中的实例（托盘里的也要退出）。冒烟必须用 bash 继承的 `APPDATA` 环境变量启动，并在启动前后核对真实 `settings.json`（1.4.0 冒烟的教训，见发布清单第 13 节）。
- 全量 e2e 必须带 `-tags=e2e` 的构建；不要在打过正式包之后用 `E2E_SKIP_BUILD=1` 复用产物（1.4.0 收尾的教训）。

## 8. 恢复入口与执行账本

新会话恢复时：读 CLAUDE.md 第 3 节、本计划第 3、8 节、相关契约；核对 `git log` 与工作区；不重跑已验证的任务，不凭聊天记忆补进度。

| 日期 | 任务 | 结果 | 提交 | 验证 | 下一项与放行状态 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-22 | 计划 | v0.1 草案；PRD v2.6（用户当天审核通过）；D-018；技术设计 §18、API §15、数据库设计 3.9 与 4.6 M7 增补、UI 稿 9.6、DESIGN 11.7 草稿；路线 3.9 | 未提交 | 文档 UTF-8 无 BOM；契约与现有代码核对（分发、卸载、安装、更新、来源记录、`Registry.Active()`、`SKILLS_FAKE_GITHUB`） | 用户 2026-11-22 按建议批准（含 Q-M7-1–5、B-M7-01–03），放行 T00 |
| 2026-11-22 | T00 | 原型 5 状态（`skills-panel.html` 新增 `restore-*`）与 `capture-proto-m7.py`；10 张 1440×900 截图在 `design/r2/shots-m7/`；`internal/contract` 新增 `SkillManifestTarget` / `SkillRestoreItem` / `SkillRestorePreview` / `SkillRestoreSelection` 等 8 个类型，M103 的出参命名 `SkillManifestRestoreResult`（M83 已占用 `SkillRestoreResult`）；`bun run generate` 通过（方法未实现前绑定不出现，同 M6 T00 口径） | `3d02764`（本地，未推送） | 原型首轮两处布局问题已修（`tico('custom')` 抛错导致 5.7KB 空图；横幅挤占主体行）；UI 稿 9.6.1 记录 5 条实现细节；`go build` / `go vet` 通过 | 待用户审核原型与契约；T01 未放行 |
| 2026-11-22 | T01 | `internal/skills/manifest.go`（新）：清单读入 5 种状态（missing / ok / unreadable / too-large / version-unknown）、单项逐字段校验（skillPath、name、target kind、工具 id、项目子目录、GitHub 字段、时间）、重复项保留最后并记 warning、2000 项上限、按目标键排序的稳定序列化（2 空格缩进、无 HTML 转义、结尾换行）、临时文件 + 原子替换；`SaveManifest` 写入前整体校验，任一条不合法即拒绝写入；`manifest_test.go` 11 个测试（含 28 个用例） | `fe88ed8`（本地，未推送） | `go test ./...` 692 项全过；`gofmt` / `go vet` 干净；序列化断言不含驱动器路径、用户名与 `t-` 登记 id | 待放行 T02 |
| 2026-11-22 | T02 | `internal/service/skills_manifest.go`（新）：`updateManifest`（missing 建立、ok 写、unreadable / too-large / version-unknown 停写并置 stale 标志，操作本身不受影响）、`manifestTarget`（内置工具 id；登记目标用名称 + 类型，项目子目录取分发时的实际子目录）、两种条目构造；接入 5 个写入点：M79 分发、M81 卸载删除、M91 装进工具（仅 GitHub 来源）、M96 工具更新刷新提交、收编换链接；`SkillService` 增加 `manifestStale` 标志；`skills_manifest_test.go` 7 个测试（分发 + 卸载、项目子目录、GitHub 直装、更新、收编、unreadable、version-unknown） | `f0aaa63`（本地，未推送） | `go test ./...` 699 项全过；`gofmt` / `go vet` 干净；两份坏清单字节不变且操作照常成功 | 待放行 T03 |
| 2026-11-22 | T03 | `internal/skills/restore.go`（新）：纯函数判定 `JudgeManifest`（`create` / `exists` / `conflict` / `skip` 全表，链接与分发记录认「已一致」，GitHub 项按来源记录认，内置目录拒绝，未匹配工具 / 登记 / 缺失技能给中文原因）、`r-` + 12 位十六进制稳定 id、「本机有、清单没有」（只含当前知识库的分发记录与全部工具来源记录）；`internal/service/skills_restore.go`（新，M102）：组装事实（检测、登记、state.json、sources.json、内置目录）、未匹配登记名去重、pending 计数、坏清单只回状态与 canRebuild、stale 标志进 warnings；`restore_test.go` 5 个测试 + `skills_restore_test.go` 2 个测试 | `208aa61`（本地，未推送） | `go test ./...` 706 项全过；`gofmt` / `go vet` 干净；Vitest 272、`tsc` 通过；M102 绑定已生成；载荷断言不含知识库与 HOME 绝对路径 | 待放行 T04 |
| 2026-11-22 | T04 | `skills_restore.go` 增加 M103 `RestoreManifest`（Go 侧改名规避 M83 的 `Restore`，API §15 已注明）：`opMu` 内重读清单并重判、按选择顺序逐项执行；知识库项按清单方式分发（冲突只在勾选覆盖时先备份），成功写 `state.json`；GitHub 项按 owner/repo/ref/commit 分组共用一次下载（E13 进度、可取消），解包后按拷贝安装并写来源记录；已一致 / 清单变化 / 未勾覆盖 / 已取消逐项写明；有成功项时清扫描缓存并发 E12；日志沿用 `distribute` / `install`，detail 写「从清单还原」；`skills_restore_exec_test.go` 4 个测试（链接与冲突覆盖、同组只下一次、取消与未知 id、坏清单拒绝）；顺带修掉 T03 误带入 `internal/settings/accent_test.go` 的一行空白 | `43b2b3a`（本地，未推送） | `go test ./...` 710 项全过；Vitest 272、`tsc` 通过；`gofmt` / `go vet` 干净 | 待放行 T05 |
| 2026-11-22 | T05 | `skills_restore.go` 增加 M104 `RebuildManifest`：按本机工具来源记录（目标目录仍在）+ 当前知识库的分发记录重建；登记已移除 / 目录已消失 / 目标无法映射的项跳过并计数；原文件先改名为 `skill-distributions.json.corrupt-<UTC 时间>`（改名失败则不写并报 `io-failure`），`version-unknown` 拒绝重建；成功后清 stale 标志；`skills_restore_rebuild_test.go` 3 个测试；技术设计 18.6 的写入口径改为「整体校验、任一条不合法拒绝写入」与 T01 实现对齐 | `8c54272`（本地，未推送） | `go test ./...` 713 项全过；Vitest 272、`tsc` 通过；`gofmt` / `go vet` 干净；future-version 清单字节不变 | 待放行 T06 |
| 2026-11-22 | T06 | `transport.ts` 增加 M102–M104 包装；`RestoreView.tsx`（新）：四组列表、默认勾选、冲突勾选后才覆盖、需要联网胶囊、执行进度（E13 同 requestId）与取消（AbortSignal）、结果页与未匹配提示、本机有清单没有折叠、重建确认与 `onResult` 提示；`ToolSkillsPage` 顶部三种横幅（待还原 / 无法读取 / 更新版本）与打开入口；`SkillsPanel` 子视图接线、Esc 过滤加入 `restore-view`；`skills-panel.css` 新增 `.restore-*`；测试：`RestoreView.test.tsx` 4 个、多个横幅 2 个，`ToolSkillsPage.test.tsx` 与 `SkillsPanel.test.tsx` 的 transport mock 补齐 | `a5ffd7e`（本地，未推送） | Vitest 40 文件 / 278 项全过；`tsc --noEmit` 与 `build:web` 通过；Go 713 项全过 | 待放行 T07 |
| 2026-11-22 | T07 | `scripts/e2e/ac-m7.sh`（新，66 项断言）：两台机器=两套临时 APPDATA + 假 HOME，知识库含清单复制给 B；A 登记自定义与项目、分联接 / 拷贝、GitHub 直装 pdf；清单断言 4 项、含项目子目录与 GitHub 项、不含工作目录 / 用户名 / `t-` / 反斜杠路径；B 横幅 3 项、四组状态、默认选中与「1 项需要联网」、首轮还原（冲突不动、Droid 未匹配、GitHub 项联网装好、清单字节不变）、勾选冲突覆盖带备份、清单损坏后重建与 `.corrupt-*` 副本、pdf 保留 GitHub 来源、真实 `.claude/skills` / `.agents` 前后不变；`scripts/test-e2e.sh` 自动收录 | `7b242e9`（本地，未推送） | ac-m7 66/66；全量 e2e 363 项（15/16/22/47/37/60/54/46/66）EXIT=0；中途三次修正均为脚本夹具与断言（A 机假 HOME 缺 Claude Code 探测标记；M103 只回报请求执行的项，跳过项不在结果列表；安装结果行文案），并新增 Q-M7-6 记录 M5 遗留的显示缺陷 | 待放行 T08 |
| 2026-11-23 | T07 补充（Q-M7-6） | `SkillOpItem` 增加可选 `name`；`skills_install_exec.go` 装进工具时为每个结果项填候选名；`InstallView` 结果行优先用候选名 / `name`，不再退到工具 id；Go 测试与 `InstallView.test.tsx` 各加一条断言；`ac-m7.sh` 恢复严格文案断言（`pdf → Claude Code：已安装`）；API §12.1 注明该字段 | `c1c9433`（本地，未推送） | `go test ./...` 713 项、Vitest 278 项、`tsc` 通过；全量 e2e 363 项（15/16/22/47/37/60/54/46/66）EXIT=0，ac-m7 恢复严格文案断言 | 用户已批准修复 |
| 2026-11-23 | T08 | `scripts/visual/capture-m7.py`（新）：e2e 构建 + 临时 APPDATA + 假 HOME + 慢速假 GitHub（5 秒延迟以拍进行中），5 状态 × 浅深共 10 张真实截图替换原型；UI 稿 9.6.1 逐状态差异、夹具说明与对比度，DESIGN 11.7 追加实测；实现中修三处：还原视图误用模态外壳 `.skills-subview`（改与「管理目录」相同的整页子视图）、底栏从滚动内容移到固定第三行、重建确认卡去掉占位条数；`capture-m7.py` 首次把 `state.json` 写到 APPDATA 根（应为 `APPDATA/Mile Knowledge Base/`），修正后夹具正常 | `caf211e`（本地，未推送） | `tsc` 与 Vitest 40 文件 / 278 项通过；全量 e2e 363 项（15/16/22/47/37/60/54/46/66）EXIT=0；对比度最低 6.08（门槛 4.5） | 待放行 T09 |
| 2026-11-23 | T09 | `perf-gate.py` 新增 `kind=m7`（预览 200 项 ≤300 ms、清单写入 2000 项 ≤50 ms、还原 100 项 ≤3000 ms）；`perf-m7.sh`（新）：100 个知识库技能、10 个登记项目、200 项清单、慢速假 GitHub，清单写入用「2000 项与 1 项清单各 10 次分发的逐对差值」测净开销；热点修复：还原每轮只建一次探测事实（此前每项一次 Detect）、重建每轮只读一次登记表（此前每条一次）；技术设计 18.9 记录方法与实测 | `1070c5f`（本地，未推送） | perf-m7 `perf-gate ok`：预览 p95 60.8 ms、写入净增量 p95 36.0 ms、还原 100 项 76.3 ms（首测 424.7 / 4552.9 后修复）；perf-m6 空载复测 ok：基准 132.9、项目 277.6、保存 35、移除 20.3 ms；三次非空载复测（基准最高 206.5、项目最高 411.4）记录为整机负载，未降门槛；Go 713、Vitest 278、`tsc` 通过 | 待放行 T10 |

## 9. 自查

- FR 覆盖：FR-SKM-SYNC 的清单 → T01、T02；还原 → T03、T04；读不出与重建 → T05；界面 → T06；AC-SKM-20–25 → T07。
- 契约前置：T00 之前不写业务代码；契约草稿与本计划一起审。
- 安全：清单是不可信输入（T01）；还原只写登记目录、备份区、暂存区与本机记录（T04）；不新增外连主机，GitHub 项只请求 codeload（T04）。
- 授权：安装、推送、人工亲测分别单独确认（第 7 节）。
