# M5 实施计划：R2 安装来源与更新

> 状态：v0.1 已获用户批准（2026-11-21），按「一次一个任务」执行完毕；T00–T16 全部交付。B-M5-01–03 与 Q-M5-1/2 按建议执行（B-M5-02 真实 GitHub 冒烟经用户同意后执行）。
> 本次目标：交付 PRD 第 15 节的 M5 范围（FR-SKM-SRC、FR-SKM-UPD）。闭环为「粘贴 GitHub 地址或 `owner/repo`，或选一个 zip → 列出候选并勾选 → 装进知识库或直接装进工具 → 打开浮层看到 GitHub 来源的技能是否有更新 → 安全地更新（知识库经变更计划，工具先备份）」。
> 上一里程碑：docs/PLAN-M4.md（已完成并推送，`d9c854b`）。下一里程碑：M6（自定义工具目录、项目源、Git 跨机同步）。

## 1. 目标、范围与授权

目标：

1. 地址解析：3 种输入（仓库地址、带 `/tree/<ref>/<path>` 的地址、`owner/repo`），其他主机与格式拒绝。
2. GitHub 客户端：只匿名访问（Q-SKM-1）；取默认分支、头部提交与子目录最近提交；codeload 下载钉在头部提交上；超时、额度、重定向主机白名单。
3. 安全解包：tar.gz 与 zip 解到暂存区；整包 200 MiB 上限（Q-SKM-5）；不安全条目整包拒绝且不留残留；跳过链接与特殊文件。
4. 候选与安装：向下 3 层找候选；装进知识库（复用 FR-SKILL，不覆盖）或直接装进工具（同分发的拷贝规则）；来源记录（知识库 `.mile-kb/skill-sources.json`、应用数据 `skills/sources.json`）。
5. 更新检测：打开浮层自动查，60 分钟内用缓存，另有手动「检查更新」（Q-SKM-2）；5 种状态；角标不变（Q-M5-1）。
6. 更新执行：知识库技能生成变更计划，新增 `importFile` 动作承载非文本文件；工具技能先备份再替换。
7. 界面：安装视图启用 GitHub 与 zip、候选勾选、目标二选一、进度与取消、错误卡；卡片与工具行的 GitHub 状态、更新预览、工具更新确认（UI 稿 9.4）。
8. 验收：AC-SKM-12–16 自动化（本地假 GitHub）；视觉对照；性能实测接入 G-PERF。

不做（留给 M6 或以后）：GitHub token、私有仓库、GitHub Enterprise、Git LFS、后台定时检查、zip 来源的更新、自定义工具目录、项目源、Git 跨机同步。

授权边界：外连只连 `github.com`、`api.github.com`、`codeload.github.com`，只在用户安装、更新或打开浮层时发出；测试与 e2e 用本地假 GitHub（`SKILLS_FAKE_GITHUB`，只在 e2e 构建读取）与假 HOME，不连真实网络、不写真实工具目录与真实应用数据；真实 GitHub 冒烟是人工项，执行前单独征得同意。

## 2. 来源基线与决策点

| 项 | 内容 |
| --- | --- |
| 需求与设计基线 | PRD 第 15 节（v2.4，Q-SKM-1/2/5 已关闭）、技术设计第 16 节草稿、API 第 13 节草稿、数据库设计 3.8 与 4.6 增补草稿、UI 稿 9.4 与 DESIGN 11.5 草稿、原型 `design/r2/skills-panel.html` 新增 7 个状态与 `design/r2/shots-m5/` 14 张截图、D-016 |
| 代码基线 | 本地 `main` `c4a8c76`（= `origin/main`） |
| 本计划 | v0.1，2026-11-21 草案；同日经用户批准 |

待用户批准时决定：

| 编号 | 问题 | 选项 | 建议 |
| --- | --- | --- | --- |
| B-M5-01 | 版本号 | (a) M5 完成发 1.3.0，打本地安装包并做 1.2.0 → 1.3.0 升级冒烟（安装前先结束残留实例，见发布清单第 9 节）；(b) 等 M6 完成再发 | 已确认（2026-11-21）：(a) |
| B-M5-02 | 真实 GitHub 冒烟 | (a) 列入 T14 人工项：用一个公开仓库读取并装进临时知识库与假 HOME，执行前单独确认；(b) 不做，只靠本地假服务 | 已确认（2026-11-21）：(a)，执行前再单独确认 |
| B-M5-03 | 整体审查方式 | (a) task-reviewer 独立审查，重点看解包安全与 `importFile`；(b) 自审 | 已确认（2026-11-21）：(a) |
| Q-M5-1 | GitHub 更新是否计入角标 | (a) 不计，只在底栏显示（PRD 口径不变）；(b) 计入，PRD FR-SKM-PANEL 改写 | 已确认（2026-11-21）：(a) |
| Q-M5-2 | 「有更新」的判定口径 | (a) 技能子目录的最近提交；(b) 整个仓库的头部提交 | 已确认（2026-11-21）：(a) |

## 3. 任务清单

执行方式：**一次只做一个任务，做完汇报并停下，等用户放行后再做下一个**。状态：待做、进行中、阻塞、待验证、已验证。

| 编号 | 目标 | 修改范围 | 依赖 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| T00 | 契约落盘 | 技术设计 §16、API §13、数据库设计 3.8/4.6、UI 稿 9.4、DESIGN 11.5、本计划；`internal/contract` | 用户批准本计划 | 按批准意见定稿、去掉草稿标记；新类型、3 个错误码、E13、`importFile` 常量与 `SKILLS_FAKE_GITHUB` 进入 contract | 待验证（门禁已过，待用户验收） |
| T01 | 地址解析与 GitHub 客户端 | `internal/skills/{ghref,github}.go`、测试 | T00 | 3 种输入与非法输入；默认分支 / 头部 / 子目录提交 / 下载；404、额度、超时、跨主机重定向全部用 `httptest` 覆盖 | 待验证（门禁已过，待用户验收） |
| T02 | 安全解包与暂存区 | `internal/skills/{unpack,stage}.go`、测试 | T01 | tar.gz 与 zip；技术设计 16.3 七条不变量各有测试；违规整包拒绝后暂存目录不存在；过期、上限 3 个、启动清空 | 待验证（门禁已过，待用户验收） |
| T03 | 候选定位 | `internal/skills/candidates.go`、`internal/vault/skillimport.go`（扩展任意深度）、测试 | T02 | 3 层、跳过点号与 node_modules、50 个上限、重复名、过大、非法名、已存在 | 待验证（门禁已过，待用户验收） |
| T04 | 来源记录与扫描合并 | `internal/skills/sources.go`、`scan.go`、测试 | T03 | A 区稳定序列化（与 provenance 同规则）、B 区读写、坏条目丢弃；扫描结果带 `remote` | 待验证（门禁已过，待用户验收） |
| T05 | 读取与暂存服务 | `internal/service/skills_install.go`（新）、M88/M89/M92、E13、transport | T04 | 读 GitHub、选 zip、取消即删暂存、进度事件节流；`mygo generate` | 待验证（门禁已过，待用户验收） |
| T06 | 安装服务 | 同上、M90/M91 | T05 | 装进知识库不覆盖；装进工具按分发规则与备份；来源记录、日志、E12 | 待验证（门禁已过，待用户验收） |
| T07 | 变更计划 `importFile` | `internal/mutation/{plan,execute}.go`、`internal/policy`、测试 | T04 | 只允许 user；Agent 生成与执行两处拒绝；暂存解析与哈希校验；替换进事务且可恢复 | 待验证（门禁已过，待用户验收） |
| T08 | 更新检测 | `internal/skills/update.go`、`internal/service/skills_update.go`（新）、M93 | T04、T05 | 60 分钟缓存与 force；分组去重、单轮 40 组上限、额度用完停下；5 种状态；完成发 E12 | 待验证（门禁已过，待用户验收） |
| T09 | 更新执行 | 同上、M94/M95/M96 | T07、T08 | 知识库：差异 → 计划（含 `importFile`、`trash`）→ M95 哈希一致才写记录；工具：备份替换与失败移回；本地改过拒绝 | 待验证（门禁已过，待用户验收） |
| T10 | 安装视图前端 | `src/components/skills/InstallView.tsx` 等、transport、store | T06 | 3 种来源可用；地址校验提示；候选勾选与禁用原因；目标二选一；进度与取消；错误卡 | 待验证（门禁已过，待用户验收） |
| T11 | 更新状态前端 | `KbSkillsPage`、`ToolSkillsPage`、更新预览子视图、工具更新确认卡 | T09、T10 | 胶囊与按钮按 UI 稿 9.4；打开浮层触发 M93；底栏检查时间与按钮；计划卡衔接与 M95 回调 | 待验证（门禁已过，待用户验收） |
| T12 | e2e 验收 | `scripts/e2e/ac-m5.sh`（新）、本地假 GitHub 服务、`scripts/test-e2e.sh` | T05–T11 | AC-SKM-12–16 自动化；断网用关闭端口；真实网络零请求、真实工具目录零写入；全量回归通过 | 待验证（门禁已过，待用户验收） |
| T13 | 视觉对照 | `design/r2/shots-m5/`、`scripts/visual/capture-m5.py`（新） | T11 | 7 状态 × 浅 / 深真实截图；与原型差异记入 UI 稿；对比度复测 | 待验证（门禁已过，待用户验收） |
| T14 | 性能与人工项 | `scripts/e2e/perf-m5.sh`（新）、G-PERF、真实 GitHub 冒烟（按 B-M5-02） | T12 | 50 MB 包读取 p95 ≤ 3 s、检查 20 组 ≤ 2 s（本地）；真实冒烟经同意后执行并记录 | 待验证（门禁已过，待用户验收） |
| T15 | 整体审查（按 B-M5-03） | 审查报告、本计划 | T14 | 阻断 0，需改项已修或经用户接受 | 待验证（阻断 0、需改已修） |
| T16 | 收尾（按 B-M5-01） | README、CLAUDE.md、路线、发布清单、本计划 | T15 | 版本与安装包；文档同步；列提交；推送前询问 | 待验证（文档已同步，推送待用户确认） |

依赖顺序 T00 → T16，串行。T07 与 T05/T06 互不依赖，T10 与 T07–T09 也不依赖，但都按「一次一个」分开放行。

## 4. 单任务精确交接（摘要）

> 通用纪律：Go 与 TS 的业务逻辑测试先行；样式与视觉按验收脚本与截图验证；测试只用临时目录、假 HOME 与本地假 GitHub，不连真实网络，不读写真实工具目录与真实应用数据；中文 UI、无 emoji；提交前扫描密钥与真实路径。

- **T01**：HTTP 客户端可注入基础地址（单测用 `httptest`，e2e 构建读 `SKILLS_FAKE_GITHUB`）；生产只认 3 个主机；`User-Agent` 带版本；`Accept: application/vnd.github.sha` 取 SHA；额度头解析；网络错误只重试 1 次；不做镜像回退。
- **T02**：tar 去掉第一层目录；zip 先验后写；按实际字节计数；违规时删暂存目录，测试断言目录不存在；200 MiB 边界用计数测试而不是真文件。
- **T03**：`vault.ImportSkills` 现在只认根或一层子目录（`skillimport.go:94-137`），扩展为接受暂存候选的任意相对路径，FR-SKILL 本地文件夹向导的行为不变（回归测试）。
- **T04**：`skill-sources.json` 的序列化逐字节对齐 provenance.json 的规则（数据库设计 §3.4）；写入前重读合并，原子替换。
- **T05**：长任务接受 ctx 取消（mygo 绑定的 AbortSignal 会取消 ctx，见 mygo `docs/bindings.md` Canceling a call）；E13 每 200 ms 最多一帧。
- **T07**：`importFile` 走与 `permanentDelete` 相同层级的 Agent 拒绝；执行器复用 `copyFileVerified`；事务 pre-image 已按 sha256 存任意字节，格式不变。
- **T08**：结果写 `skills/update-cache.json`；扫描合并时由 Go 计算 `updateState`（需要本地哈希），前端只渲染。
- **T09**：差异按相对路径与内容哈希；变动超过 500 项不生成计划；M95 必须重新算哈希。
- **T10–T11**：结构与状态按 UI 稿 9.4；新组件按 DESIGN 11.5；更新计划交给 `useMutationStore.propose`，报告 `completed` 后调 M95。
- **T12**：假 GitHub 服务从夹具目录现场打 tar.gz（含 `../evil` 恶意包），能模拟 404、额度为 0、慢响应；AC-SKM-14 关闭端口；断言整个用例期间没有对外连接（e2e 构建里客户端只认假地址）。
- **T14**：真实 GitHub 冒烟只读一个公开仓库，装进临时知识库与假 HOME；完成后删掉临时目录。

### T15 整体独立审查（task-reviewer，2026-11-21）

- 范围：基线 `c4a8c76` → `2adcfb4`（T00–T14），按 B-M5-03 交给 task-reviewer；审查代理只读、未改仓库，复核时 `go test -count=1 ./...` 639 项、Vitest 263 项与 `tsc` 均通过，未跑 e2e、未访问真实网络。
- 结论：**阻断 0，需改 3，建议 4**。解包七条不变量、`importFile` 双层拒绝、GitHub 客户端约束、更新状态机、e2e 断言真实性（落到文件系统而非空跑）与数据卫生均核实；可以修复需改项后进入 T16。
- 需改项与修复：
  1. 安装候选校验：`stageSelection` 原来接受 `state=exists`，与 API §13.3 不符，tools 目标可被覆盖。现在按**当前知识库**重算同名候选（`existingSkillNames`，与读取时同一视图），并用 `pickInstallCandidates` 只放行 `state=ok`；新增单元测试（`TestPickInstallCandidatesRequiresOK`）。
  2. 启动清空暂存：`StageStore.CleanAll` 无调用者，与数据库设计 4.6 不符。`service.New` 现在启动即清空 `skills/tmp/`（M4 的 `tmp/<opId>/` 一并清理）；新增 `TestServiceNewClearsSkillStaging`。
  3. 来源记录校验：`SaveKbSource`/`SaveToolSource` 原来不校验 `BaseHash`/`InstalledAt`，空哈希会被当成「本地没改过」使更新检测静默失效。写入与读取现在都要求 `sha256:<64 hex>` 与 RFC3339（坏条目丢弃并记 warning）；安装流程算不出哈希时不写记录并在结果里说明；数据库设计 3.8 已同步。新增 `TestSourceRecordsRequireBaseHashAndTime`；性能夹具补合法哈希。
- 建议项处理：S-1（`CheckUpdates` 先读后加锁的并发窗口）已改为同一次持锁读取与置位；S-2（错误条目的 `checkedAt` 语义）、S-3（e2e fake 的重定向白名单）、S-4（`Resolve` 对不存在段的短路）记录保留，均无实际风险。
- 修复后门禁：Go **642**、全量 e2e **250 项全过**（15/16/22/47/37/60/53）、Vitest 263、`tsc`、`build:web`。审查者未复现 e2e 与并发场景，相关结论基于代码通读；修复后的 e2e 由本轮重新跑过。

## 5. 验证与验收

| 编号 | 给定条件 | 操作 | 预期结果 | 方式 |
| --- | --- | --- | --- | --- |
| V-SRC-REF-1 | 3 种合法输入 | 解析 | owner / repo / ref / path 正确 | Go |
| V-SRC-REF-2 | 其他主机、`http://`、查询串、`..`、超长 | 解析 | `invalid-input` | Go |
| V-SRC-NET-1 | 假 GitHub 返回 404 / 额度为 0 / 超时 / 跨主机重定向 | 读取 | `not-found` / `rate-limited` / `network-failure` / 拒绝；无写入 | Go |
| V-SRC-ARC-1 | 含 `../evil`、绝对路径、盘符、保留名、大小写冲突的包 | 读取 | `unsafe-archive`，暂存目录不存在 | Go |
| V-SRC-ARC-2 | 超过 200 MiB（压缩或解开后）、zip 头部谎报大小 | 读取 | `too-large`，暂存目录不存在 | Go |
| V-SRC-ARC-3 | 含符号链接的包 | 读取 | 链接跳过并列在 `skipped` | Go |
| V-SRC-CAND-1 | 单技能、`skills/<name>`、3 层深、第 4 层、重复名、过大 | 定位 | 候选与状态正确 | Go |
| V-SRC-INST-1 | 装进知识库，目标已存在 | 安装 | 跳过，不覆盖；其余照装；来源记录写入 | Go |
| V-SRC-INST-2 | 装进工具，目标已存在 / 选覆盖 / 内置 | 安装 | 跳过 / 备份后覆盖 / `skill-protected` | Go |
| V-SRC-REC-1 | 写两次 `skill-sources.json` | 比较字节 | 稳定序列化，无无意义 diff | Go |
| V-UPD-1 | 远端变 / 本地变 / 两边都变 / 都没变 / 查询失败 | 检查 | 5 种状态正确 | Go |
| V-UPD-2 | 60 分钟内再次打开 | 检查 | 用缓存，不发请求；`force=true` 时发请求 | Go |
| V-UPD-3 | 额度在检查中途用完 | 检查 | 停在该组，其余标未检查，返回重置时间 | Go |
| V-UPD-KB-1 | 远端改了文本与图片、删了一个文件 | 更新知识库技能 | 计划含 `edit` / `importFile` / `trash`；执行后内容等于远端；可从操作日志恢复 | Go + e2e |
| V-UPD-KB-2 | 计划部分失败 | 调 M95 | `updated=false`，状态仍是有更新 | Go |
| V-UPD-TOOL-1 | 工具里的 GitHub 技能，本地没改 | 更新 | 原目录进备份，新内容生效，可恢复 | Go |
| V-MUT-IMP-1 | Agent 计划含 `importFile` | 生成 / 执行 | 两处都 `permission-denied` | Go |
| V-MUT-IMP-2 | 暂存文件被改或已过期 | 执行 | 该项失败，后续跳过；知识库不变 | Go |
| V-SKM-AC | AC-SKM-12–16 | 见 PRD 15.6 | 全部通过 | e2e |
| V-SKM-NET-0 | 全量 e2e | 运行期间 | 没有连到真实 GitHub | e2e |

门禁按技术设计第 10 节与第 16 节；实际命令按 README 第 5 节；每个任务的实际结果记在第 8 节账本。

## 6. 性能目标

| 指标 | 目标 | 依据 |
| --- | --- | --- |
| 50 MB tar.gz 从读取到列出候选（本地假服务） | p95 ≤ 3 s | 解包与哈希都是顺序 IO，本机 SSD 估算 |
| 检查 20 组更新（本地假服务） | ≤ 2 s | 每组一次请求，并发 4 |
| 浮层首屏 | 不变（≤ 50 ms，缓存命中） | 更新检查在后台 |

实测后按 G-PERF 阻断口径校准；超目标先报告再决定。

## 7. 协作、审查与执行方式

- 一次一个任务，汇报后停下；整份计划的批准不等于各任务放行。
- 不用子代理或并行会话执行编号任务；T15 的审查方式按 B-M5-03。
- 每个任务结束提交到本地（不推送）；推送只在 T16 列清单后由用户决定。
- 真实 GitHub 冒烟（B-M5-02）与安装包安装（B-M5-01）执行前分别单独征得同意。

## 8. 恢复入口与执行账本

新会话恢复时：读 CLAUDE.md 第 3 节、本计划第 3、8 节、相关契约；核对 `git log` 与工作区；不重跑已验证的任务，不凭聊天记忆补进度。

| 日期 | 任务 | 结果 | 提交 | 验证 | 下一项与放行状态 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-21 | T16 | 完成：`mygo.config.ts` 升到 1.3.0，生产构建与 NSIS 安装包（1.3.0，exe 20.3MB / 安装包 6.8MB）；静默安装 `/S` 退出码 0，安装目录版本 1.3.0；临时 APPDATA 种 1.2.0 格式（深色、sidebar 260、绿色强调色），安装后启动设置逐字段保留、界面为深色 + 绿色 + 欢迎页（截图 `E:/tmp/t16-1.3.0.png`）；真实 `%APPDATA%` 的 settings.json mtime / 长度未变，无后台实例。README、CLAUDE.md、PLAN-M0、发布清单（第 10 节）与本计划同步；M5 共 18 个提交（此前 17 个 + 收尾），列出等待推送 | 见下 | 版本与安装包、升级冒烟、文档同步 | 推送待用户确认 |
| 2026-11-21 | T15 | 完成：task-reviewer 独立审查（c4a8c76→2adcfb4，T00–T14），阻断 0、需改 3、建议 4；需改已修：安装候选只放行 `state=ok`（按当前知识库重算 exists，`pickInstallCandidates` + 单测）、`service.New` 启动清空暂存（+测试）、来源记录 `BaseHash`/`InstalledAt` 写入与读取双层校验（坏条目丢弃，算不出哈希时不写记录，+测试，数据库设计 3.8 同步）；建议 S-1 并发窗口已改为同锁读取与置位，S-2–S-4 记录保留。修复后 Go 642、全量 e2e 250 项全过、Vitest 263、tsc/build:web 通过 | 见下 | 全量 `scripts/test-e2e.sh`；`go test -count=1 ./...`（642）；Vitest 263；typecheck；build:web | 等待放行 T16 |
| 2026-11-21 | T14 | 完成：`scripts/e2e/perf-m5.sh`（本地假 GitHub：50.0 MiB tar.gz 读取 5 次、20 组更新检查；写 metrics.json 并跑 G-PERF）；`scripts/perf-gate.py` 新增 `kind=m5`（read50P95Ms 3000、check20Ms 2000，阻断）；README 与 技术设计 16.8 记录实测与命令。实测：50 MB 读取 190/178/182/204/192 ms，p95 **204 ms**；20 组检查 **69 ms**（groups=20、unchecked=0），门禁通过。B-M5-02 真实 GitHub 冒烟（用户 2026-11-21 同意后执行）：匿名读取 anthropics/skills 成功，20 个候选，提交 e4bb5d96…，31.2 s（含下载）；把 academy-guide 装进临时知识库「文档」，来源记录 commit / pathCommit / baseHash 一致；临时目录已删除。`scripts/e2e/manual-github-smoke.sh` 作为可复跑的手动项保留（不进入 test-e2e） | 见下 | `bash scripts/e2e/perf-m5.sh`（门禁通过）；真实冒烟证据；脚本与文档改动，无 Go/Vitest 代码改动 | 等待放行 T15 |
| 2026-11-21 | T13 | 完成：新增 `scripts/visual/capture-m5.py`（e2e 构建、假 HOME、本地假 GitHub、`SKILLS_FAKE_ZIP` 固定 zip；真实窗口驱动 7 状态 × 浅 / 深，写入 `design/r2/shots-m5/`，原型保留在 git 历史）；`internal/service` 增加 e2e 构建专用的 `autoOpenDialog`（生产不变）与 `SKILLS_FAKE_ZIP` 契约常量，附 e2e 构建测试；为对齐原型修了安装底栏文案、工具目标来源条显示已选名单、更新预览来源条补 owner/repo；UI 稿新增 9.4.1 逐状态差异。对比度实测（浅 / 深）：更新胶囊 9.61 / 11.12、来源行 6.22 / 6.22、错误标题 16.22 / 13.99、错误正文 7.32 / 6.61、信息正文 13.47 / 14.50、预览路径 14.95 / 13.50，全部达标 | 见下 | 14 张截图；对比度实测；ac-m5 回归；typecheck / Vitest 263；Go 639 + e2e 构建测试 | 等待放行 T14 |
| 2026-11-21 | T12 | 完成：`scripts/e2e/fake-github.py`（本地假 GitHub：默认分支 / 头部提交 / 子目录提交 / tar.gz，状态文件热切换；禁止双实例占端口）；`scripts/e2e/ac-m5.sh`（AC-SKM-12：多技能仓库只装勾选；13：`../evil` 整包拒绝、知识库与暂存零残留；14：关端口读取失败、零写入；15：v2 更新 → 预览（文本 / 二进制 / 新增 / 删除）→ 计划卡含 importFile 与 trash → 执行 → 内容等于远端、来源记录推进、rewindPrepare 可恢复；16：本地改过显示两边都改了、无更新按钮、updatePreview 拒绝）；`scripts/test-e2e.sh` 自动纳入。顺带修复：`vault.Write` 对 Windows 共享冲突的替换重试（新增 `replace_windows.go` / `replace_other.go` 与 Windows 回归测试），解决计划写入偶发 io-failure；工具行卸载 testid 带 toolId，ac-m4 选择器同步（T11 的 E12 自动刷新暴露了旧模糊选择器）；计划卡补 importFile 标签；对比度采样加一次重试。证据：ac-m5 53 项全过；全量 e2e 250 项（15/16/22/47/37/60/53）全过；e2e 构建通过 | 见下 | 全量 `scripts/test-e2e.sh`；`go test -count=1 ./...`（639）、vet/gofmt、typecheck、bindings、Vitest 263、build:web | 等待放行 T13 |
| 2026-11-21 | T11 | 完成：`skillRemote.ts`（六种状态胶囊、来源、检查时间、字节与变更摘要）；`KbSkillsPage` 卡片显示 GitHub 来源与状态，`update-available` 时按钮变「更新」、`both-changed` 只在菜单里给禁用的说明；`ToolSkillsPage` 行内显示 GitHub 胶囊与来源、更新按钮；新增 `UpdateView`（M94 预览：SHA 旧→新、逐文件修改 / 新增 / 删除、取消丢弃暂存、生成变更计划交给 `useMutationStore.propose`）；新增 `ToolUpdateDialog`（工具更新确认：变更数量与备份位置，M96 执行并汇总结果）；`SkillsPanel` 打开时调 M93、订阅 E12 刷新扫描与检查状态、底栏显示检查时间 / 未检查来源 / 额度重置与「检查更新」force 按钮、监听计划报告后调 M95 并刷新，取消计划卡时丢弃暂存。测试 8 项新增（卡片状态与更新入口、工具行更新与确认、更新预览列表 / 取消 / 冲突 / 无变化、面板 M93 与底栏、计划 → 报告 → M95） | 见下 | `bun run typecheck`；Vitest 263（37 文件）；`bun run build:web` | 等待放行 T12 |
| 2026-11-21 | T10 | 完成：`InstallView` 重写（M88–M92 前端）：3 种来源（本地文件夹仍用原向导、GitHub 表单、zip 原生对话框）；GitHub 地址内联校验与读取；候选勾选与禁用原因（已存在 / 过大 / 重复 / 名字不合法）与跳过说明；装到知识库（分类校验与提示）/ 直接装进工具（目标格、覆盖勾选、安装预览）二选一；E13 进度条与取消（AbortSignal）；整包拒绝 / 网络 / 额度错误卡与重试或换文件；安装结果行、成功后回来源页并刷新；返回或离开时丢弃暂存；保留最近记录与恢复。`SkillsPanel` 传工具列表；`transport` 支持 `readGithub` 的 AbortSignal 与把 failure.detail 放进 `TransportError`；新增安装样式。zip 读取没有 E13（后端只发 GitHub 下载进度），显示不确定进度条、等原生对话框返回。测试 11 项（来源、空地址、非法地址、候选与知识库安装、分类禁用、工具覆盖预览、返回丢弃暂存、压缩包错误、取消读取、zip 读取、本地向导） | 见下 | `bun run typecheck`；Vitest 255（36 文件）；`bun run build:web` | 等待放行 T11 |
| 2026-11-21 | T09 | 完成：`skills/update.go` 差异引擎（`DiffSkillTrees` 按相对路径与文件哈希给出 added/modified/removed、字节与二进制标记；文本判定为 UTF-8、无 BOM、LF、≤2 MiB；`FileSHA`）；`StageMeta.update`（`{kind, skillPath?, toolId?, targetPath?, sourcePath}`，M95/M96 读目标）；M94 `updatePreview`（校验来源与本地基线、Resolve 远端、无变化不建暂存、下载解包到暂存、比较并生成知识库计划；工具返回备份预览）；知识库计划：文本 create/edit（带 expectedTargetRevision）、二进制/BOM/CRLF/无扩展名/新目录文件走 importFile、远端删除走 trash；M95 `confirmKbUpdate`（执行后哈希等于暂存才移动来源记录）；M96 `updateTool`（本地改过拒绝、原目录备份 kind=update、拷贝失败移回、写来源记录与 update 日志）；`mutation/staged.go` 放宽暂存路径（允许点号开头与任意深度，仍拒绝越出形式）；transport 暴露三个方法。测试 7 项新增（差异、暂存 update 元数据、深路径 importFile、知识库预览+执行+确认、冲突与哈希不符、无变化、工具更新与本地改动拒绝） | 见下 | `go test ./internal/skills/ ./internal/mutation/ ./internal/service/`；`go test -count=1 ./...`（638）、race（213）、vet/gofmt、generate、tsc、bindings、Vitest 247、e2e 构建 | 等待放行 T10 |
| 2026-11-21 | T08 | 完成：`skills/update.go`（`skills/update-cache.json` version 1 的读写，损坏或版本不符即删；60 分钟 TTL；`CollectUpdateGroups` 按 (owner, repo, ref, path) 去重排序；`RemoteUpdateState` 给出 up-to-date / update-available / both-changed / local-modified 与 unchecked / error）；`GithubClient.LatestPathCommit`（仓库根取 ref 头部、子路径取 `commits?path=`，每组 1 次请求；空历史报 not-found）与 `RateLimitResetAt`；M93 `CheckUpdates`（检查中不重开一轮；缓存命中不发请求；后台一轮，40 组上限，额度用完停下并记重置时间；写缓存后删扫描缓存并发 E12）；扫描合并（A 区 `AttachKbRemotes`、B 区 `toolRemoteFor`）带 `latestPathCommit` / `checkedAt` / `error`；transport 暴露 `checkUpdates`。测试 11 项（缓存往返与损坏、分组去重、5 状态、A/B 区合并、轮次与缓存、60 分钟过期、额度停止、40 组上限、并发只开一轮） | 见下 | `go test ./internal/skills/ ./internal/service/`；`go test -count=1 ./...`（630 项）、vet/gofmt、generate、tsc、bindings、Vitest 247、e2e 构建 | 等待放行 T09 |
| 2026-11-21 | T07 | 完成：`mutation/staged.go`（stageId `s-<12 hex>`、stagePath ≤8 段、路径解析到 `skills/tmp/<id>/root` 内、≤50 MiB、sha256 校验）；`Builder.StagingRoot` 与 `validateImportFile`（只允许 user；无目标创建、有目标必须带 expectedTargetRevision；目标为目录拒绝；哈希不符 revision-mismatch）；`Executor.StageRoot` / `StatRevision` 与 `doImportFile`（执行时复核哈希与版本；替换先 Capture 进事务，`copyFileVerified` 写入；`needsTransaction` 覆盖替换；风险：新增 low、替换 medium）；Agent 在计划与执行两处显式拒绝。服务注入暂存区与 `vault.StatRevision`。测试 4 项（规则、创建/替换/过期版本、暂存被改、Agent 执行拒绝） | 见下 | `go test ./internal/mutation/`、`go test -count=1 ./...`、vet/gofmt、tsc/bindings | 等待放行 T08 |
| 2026-11-21 | T06 | 完成：`skills_install_exec.go`（M90 `installPreview`：KB 逐候选列 create/exists；工具复用 `PreviewDistribute` 得出 create/exists/overwrite/protected 与备份位置；M91 `install`：KB 走 `vault.ImportSkills`（不覆盖、FR-SKILL 限额），成功后写 `.mile-kb/skill-sources.json`（含 pathCommit、安装后哈希）；工具复用 `ExecuteDistribute`（拷贝模式、覆盖前备份），成功后写 `skills/sources.json`；每次安装写 `install` 日志；有成功即清扫描缓存并 E12、删除暂存；全部失败保留暂存供重试）。`ToolSource` 改为平铺字段，去掉伪造 skillPath。transport 增加 installPreview/install。测试 5 项（KB 安装与来源、KB 已存在跳过且保留暂存、工具安装、覆盖带备份、坏选择） | 见下 | `go test ./internal/service/`、`go test -count=1 ./...`、vet/gofmt、generate、tsc、bindings、Vitest | 等待放行 T07 |
| 2026-11-21 | T05 | 完成：`skills_install.go`（M88 `readGithub`：解析 → 取提交 → 下载到暂存 → 解包 → 删压缩包 → 候选 + treeHashes；M89 `pickZip`：原生对话框过滤 `*.zip`，取消返回 cancelled；M92 `discardStage`；取消时删暂存并返回 cancelled；E13 进度按 200 ms 节流，requestId 用输入的短哈希）；`SkillService` 新增可注入的 githubClient 与 openDialog（测试用）；`SkillStageProgress` 事件注册；`StageMeta.pathCommit`；transport 暴露 readGithub/pickZip/discardStage；数据库设计 4.6 与 API §13.1 补 `capped` 与 pathCommit。测试 8 项（读 GitHub 成功/坏输入/取消/404、丢弃、进度、zip 成功/取消） | 见下 | `go test ./internal/service/`、`go test -count=1 ./...`、vet/gofmt、bun generate、tsc、bindings、Vitest | 等待放行 T06 |
| 2026-11-21 | T04 | 完成：`sources.go`（A 区 `.mile-kb/skill-sources.json`：版本 1、records 按 skillPath 的 UTF-8 字节序、键序固定、2 空格 + 换行、不转义 HTML、原子写；未知版本只读不写并报 unsupported；坏条目丢弃并 warning；按 skillPath 合并/删除。B 区 `skills/sources.json`：`g-<12 hex>`、按 targetPath 合并、损坏改名 `.corrupt-<时间>` 后从空开始）。`AttachKbRemotes` / `toolRemoteFor` 给扫描结果补 `remote`（基线哈希不符 → `local-modified`，否则 `unchecked`，远端检查留给 T08）；`ScanOptions.ToolSources` 与 `ScanTools` 合并工具来源；服务 `kbSkills` 读 A 区并附加、`runScan` 传 B 区。数据库设计 3.8 补「字节序排序」澄清。测试 5 项 | 见下 | `go test ./internal/skills/`（65 项）、`go test -count=1 ./...`、vet/gofmt、tsc/bindings | 等待放行 T05 |
| 2026-11-21 | T03 | 完成：`candidates.go`（根目录自身有 SKILL.md 即单技能，名字取 frontmatter `name`，否则用仓库名；否则向下 3 层找，跳过点号与 node_modules、不进入重解析点、不钻进已识别的技能目录；按相对路径排序；状态 ok/exists/too-large/duplicate/invalid-name；50 个上限并标 truncated；文件数与字节按 FR-SKILL 上限截停）；`vault.ImportSkills` 支持任意深度候选并校验来源相对路径（拒绝 `..`、绝对、盘符、点号开头），源码路径必须解析到来源根内。测试 6 项 | 见下 | `go test ./internal/skills/ ./internal/vault/`（101 项）、`go test -count=1 ./...`、vet/gofmt、e2e 构建 | 等待放行 T04 |
| 2026-11-21 | T02 | 完成：`unpack.go`（tar.gz 去顶层目录；zip 先验中央目录再解压；`..`/绝对路径/盘符/保留名/末尾点或空格/大小写重名整包拒绝并删暂存根；符号链接与特殊文件跳过并记录；解开后 200 MiB 与 20000 条目上限，按实际字节计数；单段 ≤255）；`stage.go`（`s-<12 hex>`、30 分钟 TTL、最多 3 个按目录时间淘汰、`meta.json` 原子写与 treeHashes、过期即删、`CleanAll`）。测试 9 项覆盖七条不变量与暂存生命周期 | 见下 | `go test ./internal/skills/`（普通与 e2e）、`go test -count=1 ./...`、vet/gofmt | 等待放行 T03 |
| 2026-11-21 | T01 | 完成：`ghref.go`（3 种输入、owner/repo 字符集、ref 单段、path ≤8 段、`.git` 与尾斜杠、其他主机与 http/查询串/凭据拒绝）；`github.go`（匿名客户端：默认分支、`application/vnd.github.sha` 取头部、`commits?path=` 取子目录最近提交、codeload 下载钉在提交上、系统代理、跨主机重定向白名单、网络错误重试 1 次、404 → not-found、403/429 且剩余 0 → rate-limited 带重置时间、15s/120s 超时、200 MiB 流式上限）；`github_e2e.go` / `github_prod.go` 提供 `SKILLS_FAKE_GITHUB`（仅 e2e 读取）。测试 9 项 + e2e/prod 各 1 项。API §13.3 与 技术设计 16.9 补 ref 单段说明 | 见下 | `go test ./internal/skills/`（普通与 e2e 构建）、`go test ./...`、vet/gofmt、tsc/bindings/Vitest | 等待放行 T02 |
| 2026-11-21 | T00 | 完成：技术设计 §16、API §13（含 §1.3 三个错误码、§2.2 M88–M96、§2.3 E13、M29 的 `importFile`）、数据库设计 3.8 与 4.6 增补、UI 稿 9.4、DESIGN 11.5、本计划去掉草稿标记并写「用户 2026-11-21 审核通过」；PRD 与 D-016 同步。`internal/contract` 新增：3 个错误码、`EnvSkillsFakeGithub`、`MutImportFile` 与 MutationAction 的 stageId/stagePath/stageHash、`KbSkill.remote`/`ToolSkill.remote`、§13.1 的 11 个类型与 E13 事件；`mygo generate` 重写绑定 | 见下 | `go build ./...`、`gofmt`、`go vet`、`bun run generate`、`bun run check:bindings`、`bun run typecheck`、Vitest 全量 | 已放行 T01 |
| 2026-11-21 | 计划 | v0.1 草案；同时起草技术设计 §16、API §13、数据库设计 3.8/4.6 增补、UI 稿 9.4、DESIGN 11.5、原型 7 个状态与 14 张截图；PRD v2.4 关闭 Q-SKM-1/2/5 | 见下 | 自查见第 9 节；原型截图 14 张、无脚本错误、无横向溢出 | 用户 2026-11-21 批准，放行 T00 |

## 9. 自查

- 每个 M5 的 FR 都有任务与验证项：SRC → T01–T06、T10；UPD → T07–T09、T11。
- AC-SKM-12–16 都映射到 T12：12（多技能仓库只装勾选的）、13（`../evil` 整包拒绝无残留）、14（断网无写入）、15（变更计划更新且可恢复）、16（本地改过显示两边都改了、无一键更新）。
- 契约先行：T00 未完成前不写业务代码；技术设计 16.3 的七条解包不变量各有验证项。
- 安全：外连只连 3 个主机，测试不连真实网络；解包只写暂存区；`importFile` 只给用户计划，Agent 两处拒绝；来源记录在系统区，Agent 不能写。
- 已知限制写进技术设计 16.9：改名或移动后不再检测更新；私有仓库、Enterprise、LFS 不支持。
- 未覆盖：真实网络下的代理与重定向（B-M5-02 人工项）；干净机器。
