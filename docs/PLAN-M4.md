# M4 实施计划：R2 技能管理核心

> 状态：v0.1 已获用户批准（2026-11-20），T00 已放行；其余任务按「一次一个」逐项放行。B-M4-01–03 随计划批准按建议执行（1.2.0 / 真实工具目录联接测试执行前单独确认 / task-reviewer 审查），如与预期不符请在对应任务前提出。
> 本次目标：交付 PRD 第 15 节的 M4 范围（FR-SKM-TOOL/DETECT/SCAN/DIST/UNINST/ADOPT/LOG/PANEL）。闭环为「打开浮层看到两套技能与来源状态 → 把一个知识库技能分发到工具（链接优先）→ 在工具里读到知识库的改动 → 卸载只删链接或先备份 → 把工具里的独立技能收进知识库 → 操作记录可恢复」。
> 上一里程碑：docs/PLAN-M3.5.md（已完成并推送，`a566615`）。下一里程碑：M5（GitHub / zip 安装与更新）。

## 1. 目标、范围与授权

目标：

1. 工具注册表与真身探测：25 个工具（通用目录 + 24 个）按技术设计 13.2 的目录与证据，三态显示；「官方未核实」与「默认不启用」按 13.2 标注。
2. 扫描：知识库 `skills/` 与各工具目录两层扫描；六种来源状态（链接到知识库、链接失效、拷贝、独立、内置、链接到外部）与拷贝同步状态；先显示缓存，后台刷新。
3. 分发：默认目录联接，失败自动退回拷贝；同名冲突、覆盖前备份、重复加载提示、内置保护。
4. 卸载：链接只删链接；拷贝与独立先移备份区；恢复与冲突处理。
5. 收编：复制进知识库（复用 FR-SKILL 规则），可选把原目录换成链接；多工具同内容合并。
6. 操作记录与备份：每次操作一条记录；有备份的提供恢复；设置页「清理备份」（Q-SKM-3）。
7. 浮层：标题栏「技能」按钮与角标、两页、分发/卸载/收编/安装子视图、键盘与焦点、无库状态；侧栏「导入技能」并入浮层「安装 → 本地文件夹」。
8. 验收：AC-SKM-1–11、19；18 张视觉对照；扫描性能实测并接入 G-PERF。

不做（留给 M5/M6）：GitHub / zip 安装、更新检测、自定义工具目录、项目源、Git 跨机同步。

授权边界：知识库外只写技术设计 15.3 列出的三类位置；`SKILLS_*` 假环境只用于 e2e 构建；真实工具目录的写操作只在用户浮层操作时发生；真实工具目录的联接兼容性测试是人工项，执行前单独征得同意。

## 2. 来源基线与决策点

| 项 | 内容 |
| --- | --- |
| 需求与设计基线 | PRD 第 15 节（v2.2）、技术设计第 13 节（已审核）与本计划配套的第 15 节草稿、API 第 12 节草稿、数据库设计 4.6 草稿、UI 稿第 9 节、DESIGN 第 11 节、D-014 |
| 代码基线 | 本地 `main` `a566615`（= `origin/main`） |
| 本计划 | v0.1，2026-11-20 草案，待审批 |

待用户批准时决定：

| 编号 | 问题 | 选项 | 建议 |
| --- | --- | --- | --- |
| B-M4-01 | 版本号（Q-SKM-6） | (a) M4 完成发 1.2.0，打本地安装包并做 1.1.0 → 1.2.0 升级冒烟；(b) 等 M6 全部完成再发。PRD Q-SKM-6 原文写的「M4 用 1.1.0」已被 M3.5 使用，选 (a) 时同步改 PRD 的这句 | (a)，与 M3.5 的发布节奏一致 |
| B-M4-02 | 真实工具目录的联接兼容性测试（技术设计 13.5 第 3 条） | (a) 列入 T16 人工项，执行前单独确认，测完删除；(b) 不做，全部按拷贝处理 | (a)，否则链接优先无法在真实工具上验证 |
| B-M4-03 | 整体审查方式 | (a) task-reviewer 独立审查（同 M3.5）；(b) 自审 | (a) |

## 3. 任务清单

执行方式：**一次只做一个任务，做完汇报并停下，等用户放行后再做下一个**。状态：待做、进行中、阻塞、待验证、已验证。

| 编号 | 目标 | 修改范围 | 依赖 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| T00 | 契约落盘 | 技术设计 §15、API §12、数据库设计 4.6、本计划 | 用户批准本计划 | 三份契约与本计划按批准意见定稿；伪造开关与错误码进入 contract | 待验证（门禁已过，待用户验收） |
| T01 | 路径与链接底层 | `internal/skills/{reparse_windows,paths}.go`、`internal/contract`、测试 | T00 | 重解析点识别/创建/删除；删链接不跟随、`RemoveAll` 回归通过 | 待验证（门禁已过，待用户验收） |
| T02 | 注册表与探测 | `internal/skills/{registry,detect}.go`、注入表与测试 | T01 | 25 个工具表；3 条证据；三态；假 PATH/注册表单测 | 待验证（门禁已过，待用户验收） |
| T03 | 扫描与缓存 | `internal/skills/{scan,state}.go`、`scan-cache.json` | T02 | 六种来源状态与同步状态；缓存损坏重建；单测覆盖 | 待验证（门禁已过，待用户验收） |
| T04 | 服务与事件骨架 | `internal/service/skills_panel.go`（新）、`contract`、`src/lib/transport.ts`、`src/stores/skillsStore.ts`（新） | T03 | M76/M77 + E12；前端 store 能拿到注册表与扫描结果 | 待验证（门禁已过，待用户验收） |
| T05 | 分发 | `internal/skills/{distribute,backup,state}.go`、M78/M79、测试 | T04 | 联接成功、回退拷贝、同名冲突、覆盖备份、重复加载提示 | 待验证（门禁已过，待用户验收） |
| T06 | 卸载与备份 | `internal/skills/{backup,uninstall}.go`、M80/M81/M83/M85/M86 | T05 | 链接只删链接；拷贝移备份；恢复冲突；清理后不可恢复 | 待验证（门禁已过，待用户验收） |
| T07 | 收编 | `internal/skills/adopt.go`、M82 | T06 | 复制进知识库；换链前备份；多工具同内容合并 | 待验证（门禁已过，待用户验收） |
| T08 | 操作记录 | `internal/skills/log.go`、M84 | T07 | 每次操作与恢复各一条；坏行跳过 | 待做 |
| T09 | 浮层外壳 | `src/components/skills/`（新）、标题栏、侧栏、总览磁贴 | T04 | 开关/角标/Esc/焦点/层级/无库；移除侧栏导入入口 | 待做 |
| T10 | 知识库页 | `src/components/skills/KbSkillsPage.tsx`（新） | T09 | 分组卡片、搜索、网格/列表、批量选择、分发入口 | 待做 |
| T11 | 工具页 | `src/components/skills/ToolSkillsPage.tsx`（新） | T09 | chip、行、状态胶囊、行内操作、卸载/收编确认卡 | 待做 |
| T12 | 安装视图 | `src/components/skills/InstallView.tsx`（新） | T09 | 本地文件夹向导迁移；最近记录与恢复 | 待做 |
| T13 | 状态与键盘 | 两页与子视图 | T10–T12 | 骨架、空状态、部分失败结果卡、未检测工具折叠、键盘 | 待做 |
| T14 | e2e 验收 | `scripts/e2e/ac-m4.sh`（新）、假环境注入 | T05–T13 | AC-SKM-1–11、19 自动化；真实工具目录零写入 | 待做 |
| T15 | 视觉对照 | `design/r2/shots/`、`scripts/visual/capture-new.py`（如需要） | T13 | 18 张对照；玻璃对比度用真实工作台复测 | 待做 |
| T16 | 性能与人工项 | `scripts/e2e/perf-m4.sh`（新）、G-PERF、人工联接测试 | T14 | 扫描 p95 ≤200ms；联接兼容性按 B-M4-02 处理并记录 | 待做 |
| T17 | 整体审查（按 B-M4-03） | 审查报告、本计划 | T16 | 阻断 0，需改项已修或经用户接受 | 待做 |
| T18 | 收尾（按 B-M4-01） | README、CLAUDE.md、路线、发布清单、本计划 | T17 | 版本与安装包（若选 a）；文档同步；列提交；推送前询问 | 待做 |

依赖顺序 T00 → T18，串行。T10/T11/T12 互不依赖，但仍按「一次一个」分开放行。

## 4. 单任务精确交接（摘要）

> 通用纪律：Go 与 TS 的业务逻辑测试先行；样式与视觉按验收脚本与截图验证；测试只用临时目录与假环境，不读写真实工具目录与真实应用数据；中文 UI、无 emoji；提交前扫描密钥与真实路径。

- **T01**：`FSCTL_*` 实现按 spikes/r9-junction 的结论；识别同时认符号链接与联接；删除只 `RemoveDirectory`；遍历自写（不进入重解析点）；测试含「删链接前后目标哈希不变」与 `RemoveAll` 回归。
- **T02**：注册表逐行对齐技术设计 13.2；探测三条证据可注入（`SKILLS_FAKE_HOME/PATH/REGISTRY`，e2e 构建才读）；MiniMax 默认不启用；WorkBuddy 目录按 Q-M4-1 处理并记录。
- **T03**：两层扫描、跳过点号与 `node_modules`；状态判定顺序见技术设计 15.4；拷贝同步状态按基线哈希；缓存原子写、损坏重建。
- **T05**：预览与执行都重新校验；联接创建后 `os.Stat` 一次；失败删链接并退回拷贝；覆盖前备份；内置拒绝；逐项结果不整体回滚。
- **T06**：卸载先移备份再删；恢复冲突不覆盖；清理备份后记录 `restorable=false`；设置页入口按技术设计 15.6（关于与诊断一行 + 设置页对话框）。
- **T09–T13**：结构与状态按 UI 稿第 9 节逐项实现；品牌图标从 `design/r2/agents/` 移到 `src/assets/agents/` 并附 NOTICE；交互与可访问性按 DESIGN 11.3；`prefers-reduced-motion` 只做透明度。
- **T14**：假用户目录、假 PATH、假注册表；AC-SKM-1–11、19；断言真实工具目录（`%USERPROFILE%\.claude` 等）在测试前后哈希不变（仅 e2e 构建）。
- **T16**：扫描 p95 与操作时延写入本计划第 6 节；G-PERF 增补阈值按实测校准（阻断）。

## 5. 验证与验收

| 编号 | 给定条件 | 操作 | 预期结果 | 方式 |
| --- | --- | --- | --- | --- |
| V-SKM-PATH-1 | 目标含 `..`、绝对路径、UNC、点号开头、保留名 | 分发/卸载 | `invalid-input`，无写入 | Go |
| V-SKM-PATH-2 | 链接指向知识库技能 | 卸载 | 只删链接；目标目录哈希不变 | Go |
| V-SKM-PATH-3 | 含链接的目录 | `RemoveAll` 回归 | 链接目标哈希不变 | Go |
| V-SKM-DETECT-1 | 只有残留目录、没有 CLI/注册表/安装位置 | 探测 | `leftover`，不进默认分发目标 | Go |
| V-SKM-DETECT-2 | CLI、注册表、安装位置三种命中 | 探测 | 均 `detected` | Go |
| V-SKM-SCAN-1 | 六种来源各一个 | 扫描 | 状态正确；外部链接显示目标且可卸载/收编 | Go |
| V-SKM-SCAN-2 | 缓存损坏、版本不符 | 打开浮层 | 删除重建，不报错 | Go |
| V-SKM-DIST-1 | 联接可创建 | 分发 | 工具目录是联接，读到的内容跟随知识库 | Go + e2e |
| V-SKM-DIST-2 | 联接创建失败（注入） | 分发 | 自动拷贝，结果写「拷贝」 | Go |
| V-SKM-DIST-3 | 同名冲突、目标已存在、内置覆盖 | 分发 | 分别 `conflict` / `skipped` / `skill-protected` | Go |
| V-SKM-DIST-4 | 覆盖已存在的独立目录 | 分发（覆盖） | 原目录进备份，可恢复 | Go |
| V-SKM-UNINST-1 | 链接 / 拷贝 / 独立 / 内置 | 卸载 | 只删链接；其余进备份；内置拒绝 | Go |
| V-SKM-UNINST-2 | 有备份的记录 | 恢复 | 回到原位置；冲突时不覆盖 | Go |
| V-SKM-UNINST-3 | 备份被清理 | 恢复 | `backup-missing`，记录不可恢复 | Go |
| V-SKM-ADOPT-1 | 两个工具同内容技能 | 收编 + 换链 | 知识库出现该技能；两处变链接；原目录在备份 | Go |
| V-SKM-LOG-1 | 分发、卸载、恢复 | 查看记录 | 每次一条，恢复也记 | Go |
| V-SKM-PANEL-1 | 设置页打开 | 点「技能」 | 浮层在设置页之上、不盖侧栏；Esc 两级 | e2e |
| V-SKM-PANEL-2 | 角标 | 有更新拷贝与失效链接 | 数量正确；为 0 不显示 | e2e |
| V-SKM-PANEL-3 | 无知识库 | 打开浮层 | 知识库页空状态，工具页可用 | e2e |
| V-SKM-PERF-1 | 700 个条目 | 扫描 | p95 ≤ 200ms；首屏用缓存 | 脚本 |
| V-SKM-AC | AC-SKM-1–11、19 | 见 PRD 15.6 | 全部通过 | e2e + 人工 |

门禁按技术设计第 10 节与第 15 节；实际命令按 README 第 5 节；每个任务的实际结果记在第 8 节账本。

## 6. 性能目标

| 指标 | 目标 | 依据 |
| --- | --- | --- |
| 扫描 700 条目 p95 | ≤ 200ms | R-9 实测 60–85ms，留杀毒软件干扰余量 |
| 浮层首屏（缓存命中） | ≤ 50ms | 缓存直接渲染 |
| 分发/卸载单次 | ≤ 300ms（不含备份大目录磁盘时间） | 与现有设置写盘同量级 |

M4 实测后按 G-PERF 阻断口径校准，超目标先报告再决定。

## 7. 协作、审查与执行方式

- 一次一个任务，汇报后停下；整份计划的批准不等于各任务放行。
- 不用子代理或并行会话执行编号任务；T17 的审查方式按 B-M4-03。
- 每个任务结束提交到本地（不推送）；推送只在 T18 列清单后由用户决定。
- 真实工具目录的联接兼容性测试（B-M4-02）执行前单独征得同意，测完删除测试技能。

## 8. 恢复入口与执行账本

新会话恢复时：读 CLAUDE.md 第 3 节、本计划第 3、8 节、相关契约；核对 `git log` 与工作区；不重跑已验证的任务，不凭聊天记忆补进度。

| 日期 | 任务 | 结果 | 提交 | 验证 | 下一项与放行状态 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-20 | 计划 | v0.1 草案 | — | 自查见第 9 节 | 用户 2026-11-20 批准，B-M4-01–03 按建议执行，放行 T00 |
| 2026-11-20 | T07 | 完成：`skills/adopt.go`（复制进知识库、不覆盖已有目标；可选换链：先备份原目录再建联接，失败时恢复原目录并保留知识库副本）；服务 M82（vaultId + 工具/路径/分类/换链/同内容 extras；内置目录可收编；FR-SKILL 的文件数与大小上限；extras 内容哈希必须一致；每个工具独立换链，失败进 warnings；清缓存发 E12）。契约补 `SkillToolRef` 与 `SkillAdoptResult.warnings`；API §12.2 的 M82 补 vaultId。新增 Go 4 项测试（3 项 skills + 1 项 service 验证两工具同内容合并） | 见下 | `go test ./...` 567 项、Vitest 222 项、`tsc`、`check:bindings`、vet/gofmt | 等待放行 T08 |
| 2026-11-20 | T06 | 完成：`skills/uninstall.go`（卸载预览按 Classify 判 link/copy/independent；链接只删链接、目录先移备份；恢复冲突不覆盖、坏 id/已清理返回 backup-missing；列表按时间倒序、清理后不可恢复）；服务 M80/M81/M83/M85/M86（工具与内置目录解析、内置技能拒绝、共享目录卸载列出受影响工具、切除对应分发记录、清缓存发 E12、路径脱敏）；备份列表与设置页数据就绪。API §12.2 补 `removed` 结果值。新增 Go 5 项测试（4 项 skills + 1 项 service） | 见下 | `go test ./...` 563 项、Vitest 222 项、`tsc`、`check:bindings`、vet/gofmt | 等待放行 T07 |
| 2026-11-20 | T05 | 完成：`skills/distribute.go`（预览/执行、联接优先失败退回拷贝并写明方式、目标已存在跳过、覆盖先备份、跨分类同名冲突、重复加载提示）；`skills/backup.go`（覆盖/卸载的备份创建，移入 `backups/<id>/files` 并写 meta）；`skills/state.go` 分发记录（state.json 原子写，按工具+目标替换）；服务 M78/M79（校验 vault/技能路径/目标、注册表解析安装目录、禁用与未检测工具拒绝、跨分类冲突检测）；分发成功后写记录、清扫描缓存、发 E12；transport 暴露两个方法。新增 Go 7 项测试（6 项 skills + 1 项 service） | 见下 | `go test ./...` 558 项、Vitest 222 项、`tsc`、`check:bindings`、vet/gofmt | 等待放行 T06 |
| 2026-11-20 | T04 | 完成：`SkillService`（M76 `registry` 走探测并脱敏 `<home>` / `<local-app-data>`；M77 `scan` 缓存优先、无缓存时同步扫描、后台刷新后发 E12）；`SkillsChangedEvent` 与 `skills-changed` 事件；知识库技能来自索引（`SkillDocuments` + `BuildSkillCatalog`），描述读 SKILL.md frontmatter；扫描目标补上安装目录；`skills.SkillDescription` 导出。前端：`skillsApi`、类型再导出、`skillsStore`（registry/scan/refresh、过期响应丢弃）；`main.go` 绑定服务。`mygo generate` 重写 `src/mygo.ts`。新增 Go 3 项 + 前端 5 项测试 | 见下 | `go test ./...` 551 项、Vitest 222 项、`tsc`、`check:bindings` | 等待放行 T05 |
| 2026-11-20 | T03 | 完成：注册表补「内置目录」并把 Trae/Grok/Antigravity/ZCode/MiniMax 的内置路径归入；`scan.go` 两层扫描（跳过点号与 node_modules、不进入链接、非内置条目逐级下钻、`*` 段展开、frontmatter description）、六种来源状态（链接到知识库 / 失效 / 拷贝 / 独立 / 内置 / 外部）与拷贝同步四态（基线哈希）；`state.go` 扫描缓存（版本 1、2 MiB 上限、原子写、损坏/版本不符返回空、超 2000 条截断并记 warning）。新增 6 项测试 | 见下 | `go test ./internal/skills/` 20 项（普通与 e2e 构建）、`go test ./...` 548 项、vet / gofmt、`check:bindings` | 等待放行 T04 |
| 2026-11-20 | T02 | 完成：25 个工具注册表（通用目录 + 24 个，Readers 15 个、Trae/ZCode 标「官方未核实」、MiniMax 默认不启用、ZCode/MiniMax 的内置目录按安装位置相对展开）；`Detector` 可注入 Home/LocalAppData/UserProgramFiles/PATH/CLI 目录/卸载项，真实模式读注册表与已知文件夹，e2e 构建读 `SKILLS_FAKE_HOME/PATH/REGISTRY`；三态与三条证据按 13.3。新增 5 项测试（注册表形状、三条证据与三态、CLI 扩展名、路径展开、假环境） | 见下 | `go test ./internal/skills/` 14 项（普通与 e2e 构建各跑一遍）、`go test ./...` 542 项、vet / gofmt、`check:bindings` | 等待放行 T03 |
| 2026-11-20 | T01 | 完成：`internal/skills/reparse_windows.go`（Classify 按重解析数据判联接/符号链接、CreateJunction 走 FSCTL_SET_REPARSE_POINT 并拒绝 UNC/缺失目标、RemoveLink 只删链接、SafeWalk 不进入重解析点、UnderRoot 按分隔边界比较）与 `paths.go`（IsValidSegment 与 FR-SKILL 同名规则、ResolveUnder 拒绝越界与联接父目录、TreeHash 跳过链接）。9 项测试覆盖 R-9 C6–C8 回归：删链接后目标哈希不变、`RemoveAll` 不跟随、路径守卫与名称规则与 vault 侧逐例对齐。`golang.org/x/sys` 转为直接依赖 | 见下 | `go test ./internal/skills/` 9 项、`go test ./...` 537 项、vet / gofmt、`check:bindings` | 等待放行 T02 |
| 2026-11-20 | T00 | 完成：技术设计 §15、API §12、数据库设计 4.6、本计划去掉草稿标记并写「用户 2026-11-20 审核通过」；`internal/contract` 新增技能类型（ToolSkill、SkillScan、预览/结果/记录/备份）、3 个错误码（skill-conflict / skill-protected / backup-missing）与 3 个假环境开关常量（SKILLS_FAKE_HOME/PATH/REGISTRY，仅 e2e 构建读取）。`mygo generate` 确认前端绑定无变化（SkillService 在 T04 才出现） | 见下 | `go build ./...`、`bun run check:bindings`、`bun run typecheck` | 已放行 T01 |

## 9. 自查

- 每个 M4 的 FR 都有任务与验证项：TOOL/DETECT→T02、SCAN→T03、DIST→T05、UNINST→T06、ADOPT→T07、LOG→T08、PANEL→T09–T13。
- AC-SKM-1–11 与 19 都映射到 T14 或人工项；M5/M6 的 AC 不在本计划范围。
- 契约先行：T00 未完成前不写业务代码；技术设计 15.3 的六条不变量各有验证项。
- 风险：真实工具目录的写入安全（T01 回归 + T14 零写入断言）；工具目录约定随版本变化（技术设计 13.2 为基线，M5 前不改）；备份无限增长（Q-SKM-3 已定手动清理）。
- 未覆盖：干净机器上的工具探测（本机为准）；各工具对联接的兼容性（B-M4-02 人工项）。
