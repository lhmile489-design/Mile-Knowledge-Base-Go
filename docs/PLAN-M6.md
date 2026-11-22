# M6 实施计划：R2 扩展（自定义工具目录、项目源）

> 状态：v0.1 草案（2026-11-22），待用户审核；未获批前不实施。
> 本次目标：交付 PRD 第 15 节的 FR-SKM-CUSTOM 与 FR-SKM-PROJ。闭环为「登记一个自定义工具目录或一个项目根目录 → 它们出现在工具技能页并参与扫描 → 把知识库技能分发到它们、从它们卸载与收编，规则与内置工具一致 → 移除登记只删登记、不删文件」。
> 上一里程碑：docs/PLAN-M5.md（已完成并推送，`aaaadef`；补丁 1.3.1 `14a0256`、1.3.2 `035eb2c`）。下一里程碑：M7（Git 跨机同步，Q-SKM-4 由用户 2026-11-22 决定拆出）。

## 1. 目标、范围与授权

目标：

1. 登记存储：自定义工具（名称 + 一个技能目录）与项目（名称 + 项目根目录）存到应用数据，增删改都经 Go 校验。
2. 路径规则：登记目录必须是本机已存在的普通目录；不能在知识库里，也不能和内置工具目录、另一个已登记目录互为父子或相同（FR-SKM-CUSTOM）。
3. 项目级目录：注册表为每个内置工具增加「项目内相对目录」，项目源按它扫描（用户 2026-11-22 选择：按注册表推导，代替 PRD 原写的 skillkit 9 个）。
4. 扫描并入：自定义工具作为普通工具分组；每个项目作为一个分组，行内标出来自哪个工具的子目录。
5. 写操作并入：分发、卸载、收编、直接安装、工具技能更新都能以自定义工具与项目为目标，规则与内置工具相同（链接优先、覆盖先备份、内置只读）。
6. 移除登记：只删登记与指向它的分发 / 来源记录；目录里的文件原样保留（AC-SKM-17）。
7. 界面：技能浮层里的目录管理子视图（列表、新增、编辑、移除确认）；工具技能页的自定义分组与项目分组；分发、安装目标格里出现新目标。
8. 验收：AC-SKM-17、AC-SKM-18 与路径规则自动化（假 HOME，临时项目目录）；视觉对照；扫描性能不退化。

不做（M7 或以后）：Git 跨机同步（FR-SKM-SYNC）、项目内的 GitHub 更新以外的新来源、按项目覆盖工具注册表、网络路径（UNC）。

授权边界：测试与 e2e 只用临时目录与假 HOME，不读写真实工具目录、真实项目与真实应用数据；登记只能由用户在界面完成，Agent 没有可用工具（同 AC-SKM-19）。

## 2. 来源基线与决策点

| 项 | 内容 |
| --- | --- |
| 需求基线 | PRD 第 15 节 FR-SKM-CUSTOM / FR-SKM-PROJ、AC-SKM-17/18；本轮同步改 PRD：Q-SKM-4 关闭（拆到 M7）、FR-SKM-PROJ 改为按注册表推导、版本 v2.5 |
| 设计基线 | 技术设计 13.2 注册表与 13.5 第 4 条；第 15 节扫描、分发、卸载、收编；第 16 节安装与更新；API 第 12、13 节；数据库设计 4.6 |
| 代码基线 | `origin/main` `373f851`（1.3.2） |
| 已定 | B-M6-01：完成后发 **1.4.0**，打本地安装包并做 1.3.2 → 1.4.0 升级冒烟（用户 2026-11-22）；Q-SKM-4：Git 同步拆到 M7（用户 2026-11-22）；项目目录按注册表推导（用户 2026-11-22） |

待用户批准时决定（每项都有建议，批准计划即按建议执行，除非另有选择）：

| 编号 | 问题 | 选项 | 建议 |
| --- | --- | --- | --- |
| Q-M6-1 | 分发到项目时装进哪个子目录 | (a) 登记项目时选一个「安装子目录」（默认取已存在的第一个约定子目录，都没有时用 `.claude/skills`），之后可改；(b) 分发目标格里项目按每个已存在的子目录各出一格；(c) 项目只扫描、卸载、收编，不作分发目标 | (a)：一个项目一个目标，和自定义工具同一模型，目标格不会被撑满 |
| Q-M6-2 | 目录管理放在哪里 | (a) 技能浮层「工具技能」页顶部「管理目录」进入子视图；(b) 设置页新增一节；(c) 两处都有 | (a)：和使用场景在一起；设置页「关于与诊断」只补一行数量与入口 |
| Q-M6-3 | 移除登记时的分发记录 | (a) 删掉指向该目录的分发记录与 B 区来源记录，文件不动，记一条 `unregister` 日志；(b) 保留记录，知识库卡片显示「已移除的目标」 | (a)：卡片不再显示已不存在的目标；重新登记同一目录后，扫描会把留下的链接 / 拷贝重新识别出来 |
| Q-M6-4 | 登记目录的路径限制 | (a) 只允许本机磁盘路径（拒绝 UNC 与 `\\?\`），目录必须已存在、本身不能是联接或符号链接；(b) 允许网络路径 | (a)：网络盘不保证联接可用，也绕开 R-9 没测过的情况 |
| Q-M6-5 | 原型与截图 | (a) T00 资料阶段在 `design/r2/skills-panel.html` 新增 4 个状态并出 8 张原型截图，审核后实施；(b) 只写文字线框，T10 直接拍真实界面 | (a)：与 M4、M5 一致，先看到样子再写代码 |
| B-M6-02 | 整体审查方式 | (a) task-reviewer 独立审查，重点看路径规则与写路径；(b) 自审 | (a) |
| B-M6-03 | 真实目录人工测试 | (a) T11 列为人工项：登记一个真实项目（只读扫描）并分发一个测试技能到临时自定义目录，执行前单独确认；(b) 不做 | (a) |

## 3. 任务清单

执行方式：**一次只做一个任务，做完汇报并停下，等用户放行后再做下一个**。状态：待做、进行中、阻塞、待验证、已验证。

| 编号 | 目标 | 修改范围 | 依赖 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| T00 | 契约起草与落盘 | 技术设计 §17、API §14（M97 起）、数据库设计 4.6 M6 增补（`skills/targets.json`）、UI 稿 9.5、DESIGN 11.6、原型（按 Q-M6-5）；审核通过后写 `internal/contract` | 用户批准本计划 | 契约按批准的 Q-M6 选择写成并经用户审核；新类型、方法进入 contract；`mygo generate` 通过 | 已验证（契约 2026-11-22 审核通过；`internal/contract` 与绑定已落盘） |
| T01 | 注册表项目级目录 | `internal/skills/registry.go`、技术设计 13.6、`registry_test.go` | T00 | 每个内置工具有 `ProjectDirs`（官方写明的标「已核实」，其余标推导）；去重后的项目目录清单有测试锁定 | 已验证 |
| T02 | 登记存储与路径规则 | `internal/skills/targets.go`（新）、`targets_test.go` | T01 | `targets.json` 原子写、坏条目丢弃；名称单段规则；路径必须本机、存在、非重解析点、不在知识库、不与内置 / 已登记目录重叠（相同、父、子都拒绝，大小写不敏感，按真实路径比较） | 已验证 |
| T03 | 探测与扫描并入 | `internal/skills/{detect,scan,state}.go`、`targets_scan_test.go` | T02 | 自定义工具生成 `DetectedTool`（`kind=custom`，ID = 登记 id `t-<12 hex>`，T00 定稿）；项目生成一个分组（ID = 登记 id），扫描实际存在的约定子目录，行带 `sourceToolId` / `subdir`；扫描缓存版本递增 | 已验证 |
| T04 | 登记服务 | `internal/service/skills_targets.go`（新）、`skills_panel.go`、`skills_dialog_*`、`skills/sources.go`、transport | T03 | 列出 / 选目录（令牌，10 分钟）/ 保存 / 移除 / 打开目录；M76 与 M77 并入登记、路径脱敏；保存与移除后清扫描缓存，前端轮询刷新（不依赖后台 E12，见 1.3.1 结论）；`mygo generate` | 已验证 |
| T05 | 写操作目标扩展 | `skills_distribute.go`、`skills_uninstall.go`、`skills_install_exec.go`、`skills_targets.go`（解析）、测试 | T04 | wire spec 新增 `project:<dir>:<rel>`；分发 / 卸载 / 收编 / 安装 / 更新都能解析新目标；目录不存在时写操作拒绝；内置保护与覆盖备份规则不变 | 已验证 |
| T06 | 移除登记的清理 | `skills_targets.go`、`state.go`、`sources.go`、`skills_panel.go`、测试 | T05 | 按 Q-M6-3：删分发记录与 B 区来源记录，写 `unregister` 日志，目录文件字节不变；与写操作串行（`opMu`）；重新登记后残留的拷贝能被扫描再识别 | 已验证 |
| T07 | 目录管理子视图 | `src/components/skills/TargetsView.tsx`（新）、`SkillsPanel.tsx`、`ToolSkillsPage.tsx`、`skills-panel.css`、测试 | T04 | 列表（名称、脱敏路径、技能数、类型、目录不存在）；新增自定义 / 项目（选目录、名称、项目安装子目录）；编辑；移除确认写明「文件不会删除」；校验错误就地显示；输入框用 `.field` | 已验证 |
| T08 | 工具页与目标格 | `ToolSkillsPage`、`DistributeDialog`、`InstallView`、`ToolIcon`、`AboutSection`、`appStore`、`skillsView` | T05、T07 | 自定义工具出现在 chip 与目标格；项目行显示来源工具图标与子目录；分发与安装目标格出现项目（写入 installRel）与自定义工具，目录不存在时禁用；设置页加入口 | 已验证 |
| T09 | e2e 验收 | `scripts/e2e/ac-m6.sh`（新）、`skills_dialog_e2e.go`（目录队列）、`scripts/test-e2e.sh`（自动收录） | T05–T08 | AC-SKM-17、18；重叠与知识库内路径拒绝；移除登记后文件仍在；真实工具目录零写入；全量回归通过 | 已验证（ac-m6 46/46；全量结果见账本） |
| T10 | 视觉对照 | `design/r2/shots-m6/`、`scripts/visual/capture-m6.py`（新）、UI 稿 9.5.1、DESIGN 11.6 | T08 | 目录管理 4 状态 × 浅 / 深真实截图；与原型差异记入 UI 稿；对比度复测 | 已验证 |
| T11 | 性能与人工项 | `scripts/e2e/perf-m6.sh`（新）、G-PERF（`kind=m6`）、`targets.go` 重叠判断、人工测试（按 B-M6-03） | T09 | 10 个项目 × 每个 5 个约定子目录 × 每个 20 个技能时扫描 p95 ≤ 300 ms；M4 的 700 条目扫描不退化（≤ 200 ms）；人工项经同意后执行并记录 | 已验证（性能门禁通过；人工项 15/15，用户 2026-11-22 同意后执行） |
| T12 | 整体审查（按 B-M6-02） | 审查报告、本计划、受影响代码与测试 | T11 | 阻断 0，需改项已修或经用户接受 | 已验证（0 阻断；4 需改已修；建议 8 条中 4 条已采纳、2 条文档澄清、2 条记录不改） |
| T13 | 收尾（按 B-M6-01） | README、CLAUDE.md、路线、发布清单、本计划、`mygo.config.ts` | T12 | 1.4.0 安装包与升级冒烟；文档同步；列提交；推送前询问 | 已验证（1.4.0 安装与 1.3.2 → 1.4.0 冒烟完成；门禁 Go 664 / Vitest 272 / 全量 e2e 297；提交在本地，待推送确认） |

依赖顺序 T00 → T13，串行；T07 与 T05/T06 互不依赖，但仍按「一次一个」分开放行。

## 4. 单任务精确交接（摘要）

> 通用纪律：Go 与 TS 的业务逻辑测试先行；测试只用临时目录与假 HOME，不读写真实工具目录、真实项目与真实应用数据；中文 UI、无 emoji；提交前扫描密钥与真实路径。

- **T01**：`ProjectDirs` 放在注册表条目里，不能直接把用户级目录去掉 `~/` 得到；官方没写项目级目录的工具不列。核实结果：19 个目录，见技术设计 13.6；计划草稿里的 `.pi/skills` 不成立（Pi 官方只写 `.agents/skills`）。
- **T02**：重叠判断用真实路径（`filepath.EvalSymlinks` 之后再比较），大小写不敏感；内置工具的安装目录、扫描目录、内置目录都算「已登记」。保存时把名称与路径一起重新校验，不信前端。
- **T03**：项目分组只扫描实际存在的约定子目录；同一个技能目录不会因为两个工具共用同一子目录（如 `.agents/skills`）出现两次。项目根下其他目录一概不读。
- **T04**：选目录沿用 M12 `pickFolder` 的令牌模式，前端拿不到绝对路径；保存时只收令牌。**不要依赖后台 goroutine 发出的 E12**（1.3.1 已证实不送达），需要刷新时由前端调 M77，store 已有 `scanning` 轮询。
- **T05**：wire spec 现有 `install:` / `scan:<n>:` / `builtin:<n>:`；项目新增 `project:<dir>:<rel>`（T00 定稿：`<dir>` 是项目内约定目录原样，如 `.claude/skills`，`<rel>` 1–2 段；服务端按最长前缀匹配校验）。
- **T06**：清理只改应用数据里的 `state.json` 与 `skills/sources.json`；不碰知识库里的 `skill-sources.json`（知识库技能不受目标移除影响）。
- **T07–T08**：结构按 UI 稿 9.5；组件按 DESIGN 11.6；所有文本输入用 `.field`（1.3.2 的教训）。
- **T09**：假 HOME 下建两个临时项目，一个含 `.claude/skills/x`，一个含 `.agents/skills/y` 与 `.cline/skills/z`；一个临时自定义目录；断言真实 `%USERPROFILE%\.claude` 前后不变。

## 5. 验证与验收

| 编号 | 给定条件 | 操作 | 预期结果 | 方式 |
| --- | --- | --- | --- | --- |
| V-TGT-1 | 名称为空、含 `/`、超长；路径不存在、是文件、是 UNC | 保存登记 | `invalid-input`，不写盘 | Go |
| V-TGT-2 | 路径在知识库里；等于 / 包含 / 位于内置工具目录或另一登记目录 | 保存登记 | `path-escape`（知识库）或 `skill-conflict`（重叠），detail 写明冲突对象 | Go |
| V-TGT-3 | 登记目录本身是联接 | 保存登记 | 拒绝（按 Q-M6-4） | Go |
| V-TGT-4 | `targets.json` 损坏或含坏条目 | 启动后读取 | 坏文件改名 `.corrupt-<时间>` 后从空开始；坏条目丢弃并记 warning | Go |
| V-SCAN-1 | 项目含 `.claude/skills/x`、`.agents/skills/y` | 扫描 | 项目分组下出现 x、y，各自标出来源工具 | Go + e2e |
| V-SCAN-2 | 两个已登记项目 + 一个自定义目录 + 内置工具 | 扫描 | 各分组互不重复；M4 的六种来源状态在新分组里同样正确 | Go |
| V-DIST-1 | 自定义目录 | 分发知识库技能（链接 / 拷贝 / 覆盖） | 与内置工具一致；覆盖先备份 | Go + e2e |
| V-DIST-2 | 项目（安装子目录 `.claude/skills`） | 分发、卸载、收编 | 写到 `<项目>/.claude/skills/<名>`；卸载链接只删链接 | Go + e2e |
| V-RM-1 | 自定义目录里有分发的链接与独立技能 | 移除登记 | 文件字节不变；分发记录与 B 区来源记录删除；日志一条 `unregister` | Go + e2e |
| V-AGENT-1 | 执行模式全自动 | 让 Agent 登记目录或分发到项目 | 没有可用工具，状态不变 | Go |
| V-SKM-AC | AC-SKM-17、18 | 见 PRD 15.6 | 全部通过 | e2e |

门禁按技术设计第 10、15、16、17 节；实际命令按 README 第 5 节；每个任务的实际结果记在第 8 节账本。

## 6. 性能目标

| 指标 | 目标 | 依据 |
| --- | --- | --- |
| 10 个项目 × 5 个约定子目录 × 20 个技能 + 内置工具扫描 | p95 ≤ 300 ms | M4 实测 700 条目 p95 123 ms；新增约 1000 条目 |
| M4 基准（700 条目，无项目） | p95 ≤ 200 ms（不退化） | G-PERF `kind=skills` |
| 登记保存 / 移除 | ≤ 100 ms（不含扫描） | 只读写一个小 JSON |

实测后按 G-PERF 阻断口径校准；超目标先报告再决定。

## 7. 协作、审查与执行方式

- 一次一个任务，汇报后停下；整份计划的批准不等于各任务放行。
- 不用子代理或并行会话执行编号任务；T12 的审查方式按 B-M6-02。
- 每个任务结束提交到本地（不推送）；推送只在 T13 列清单后由用户决定。
- 真实目录人工测试（B-M6-03）与安装包安装（B-M6-01）执行前分别单独征得同意；安装前确认没有运行中的实例（托盘里的也要退出）。

## 8. 恢复入口与执行账本

新会话恢复时：读 CLAUDE.md 第 3 节、本计划第 3、8 节、相关契约；核对 `git log` 与工作区；不重跑已验证的任务，不凭聊天记忆补进度。

| 日期 | 任务 | 结果 | 提交 | 验证 | 下一项与放行状态 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-22 | 计划 | v0.1 草案；PRD v2.5 草案（Q-SKM-4 转入 M7、FR-SKM-PROJ 改为按注册表推导）；路线 3.8 改写、新增 3.9 M7（原 3.9 顺延为 3.10）；D-017 | 未提交 | 自查见第 9 节 | 用户 2026-11-22 按建议批准，放行 T00 |
| 2026-11-22 | T00（1/2 契约草案） | 技术设计 §17（注册表项目级目录草案、路径 7 条规则、扫描并入、写路径 `project:<dir>:<rel>`、移除清理、安全、性能、测试）；API §14（M97–M101、`SkillTarget`、`SkillTool.kind` / `ToolSkill.sourceToolId` / `subdir`）；数据库设计 4.6 M6 增补（`targets.json`、scan-cache v2、日志 `unregister`）；UI 稿 9.5（4 个原型状态 + 文字补充）；DESIGN 11.6；原型新增 4 状态，8 张原型截图在 `design/r2/shots-m6/`（`scripts/visual/capture-proto-m6.py`，headless Edge，1440×900，浅 / 深） | `3ea31bd` + `abc9eb7`（本地，未推送） | 截图 8 张尺寸 1440×900；既有 `tools` 状态无回归；新文件 UTF-8 无 BOM；typecheck、绑定对拍、Vitest 264、Go 测试 / vet / gofmt 全过；Q-M6-7 新增（重复加载口径） | T00 2/2：`internal/contract`（`SkillTarget` 等 8 个类型、`SkillTool.kind` / `missing`、`ToolSkill.sourceToolId` / `subdir`、`op` 并集）加进 contract；`mygo generate` 重写 `src/mygo.ts`（新类型要等 T04 有方法引用才会出现在绑定里）；Go 构建通过 |
| 2026-11-22 | T01 | 注册表加 `ProjectDirs` 与 `ProjectSkillDirs()` / `ProjectDirTool()`：19 个去重目录（按注册表顺序）；核实结果写进技术设计 13.6——9 条官方依据（Claude Code、Cursor、Trae CN、Qoder、Grok、OpenCode、Gemini、Windsurf 旧版路径、Augment、CodeBuddy、Kiro、Cline、Warp、Kimi、DeepSeek、OpenClaw `skills`）加 1 条推导（`.codex/skills`，Cursor/Kimi/Warp 兼容读取）；`.pi/skills` 不成立已删；Hermes/WorkBuddy/ZCode/Antigravity/MiniMax/Trae CN 不列；Windsurf 文档迁到 Devin 记为 Q-M6-8；`registry_test.go` 锁定清单顺序、形状与空清单 | `162f40e` + `737a4d3` | `go test ./...` 全过（含新增 3 个测试）、`go vet` 与 `gofmt -l .` 无输出、`tsc --noEmit` 通过 |
| 2026-11-22 | T02 | `internal/skills/targets.go`：`targets.json` 读写（原子写、坏条目丢弃并记 warning、版本不认识改名 `.corrupt-<时间>`、目录被删仍保留登记）；名称规则（≤40、单段、不以点开头、不与内置工具重名、登记内不重名）；路径规则 7 条（本机绝对路径、存在、是目录、本身非重解析点、与知识库双向不重叠、与内置目录不重叠、与已登记目标不重叠，真实路径 + 大小写不敏感比较）；100 个上限、编辑跳过自身；`DefaultInstallRel`；`V-TGT-1–4` 的单测 | `0050e12` + `fcb3531` | `go test ./...` 全过（新增 7 项）、`go vet`、`gofmt -l .` 无输出 |
| 2026-11-22 | T03 | `DetectedTool` 增加 `Kind` / `Missing` / `ProjectDirs`；`MergeTargets`（内置 → 自定义 → 项目，各自按创建时间；目录不在时 `Missing=true`、空分组）；项目分组只扫实际存在的约定子目录，行带 `base=project:<dir>`、`sourceToolId`（`ProjectDirTool`）与 `subdir`；分发记录按登记 id 匹配；scan-cache 版本升 2 | `360a9e9` + `4fc5fae` | 新增 3 项测试（合并顺序/字段、项目与自定义分组、缺失目录空分组）全过；`go test ./...`、`go vet`、`gofmt -l .` 无输出 |
| 2026-11-22 | T04 | `skills_targets.go`：M97 列表（快速技能数、项目 dirs = 存在的约定目录 + installDir）、M98 选目录（10 分钟令牌、可重复用到保存成功）、M99 保存（新建 / 编辑，名称与路径二次校验、默认 installRel）、M100 移除（删分发与来源记录、记 `unregister`、文件不动）、M101 打开目录；M76/M77 并入 `MergeTargets` 并用 `<home>` / `<local-app-data>` / 盘符省略三段脱敏；`SKILLS_FAKE_PICK_DIR` 供 e2e；transport 新增 5 个方法并重新生成绑定 | `84c5f7c` + `7c6af57` | 新增 3 项服务测试（保存/列表/去重/重叠/知识库拒绝/编辑保留 createdAt、项目 dirs 与令牌过期、移除清理记录且文件不动）；`go test ./...`、`go vet`、`gofmt` 无输出；`bun run test:web` 绑定对拍 + Vitest 264 通过；`tsc --noEmit` 通过 |
| 2026-11-22 | T05 | `DetectedTool` 增加 `ProjectRoot` / `InstallRel`；`skillTools()` 统一供 M78/M79/M80/M81/M82/M90/M91 解析；`resolveSpec` 支持 `project:<dir>:<rel>`（最后一段冒号分隔，目录须在项目的约定目录里，installRel 可不存在）；分发与安装对已删除的登记目录返回 `not-found`；内置保护与备份规则不变 | `f17d4e2` + `13ad287` | 新增 2 项测试（自定义与项目分发、写入位置与记录、项目 wire spec 卸载、未知目录与未知登记拒绝、缺失目录拒绝）全过；`go test ./...`、`go vet`、`gofmt` 无输出 |
| 2026-11-22 | T06 | `opMu` 串行化全部写操作（分发 / 安装 / 卸载 / 收编 / 恢复 / 清理备份 / 更新 / 保存与移除登记）；`skills.RemoveDistributions` 与 `RemoveToolSources` 统一清理并返回数量；移除后目录哈希不变；重新登记同一目录后残留拷贝按 independent 再次出现 | `c3ea975` + `d93837c` | 新增 1 项测试、收紧 1 项（TreeHash 前后一致）；`go test ./...`、`go vet`、`gofmt` 无输出 |
| 2026-11-22 | T07 | `TargetsView`（列表 / 新增自定义 / 登记项目 / 编辑 / 移除确认 / Esc 分层返回）；工具技能页顶部「管理目录」入口 + 自定义与项目计数；SkillsPanel 子视图接线；样式 `.targets-*` 复用既有令牌与 `.pill` / `.skm-chip`；输入用 `.field` | `1f95409` + `3beae7f` | 新增 5 项组件测试全过（列表与缺失提示、令牌建自定义、项目默认安装子目录与切换、就地错误、移除确认与计数）；`tsc --noEmit` 通过；Vitest 269 项通过；真实界面视觉对照留给 T10 |
| 2026-11-22 | T08 | `installTargets` 共享筛选（通用目录优先、登记目标排在后面）；分发 / 安装目标格显示项目「写入 .claude/skills」与自定义目录尾巴，目录不存在禁用；`tool.kind` / `missing` / `installRel` 进契约与绑定；项目行图标换成来源工具并带「工具 · 子目录」title，项目行不显示「重新分发」；`ToolIcon` 支持 folder；设置页「关于与诊断」一行「自定义 N · 项目 M」+「管理」直达管理目录 | `abf9c8d` + `e7d0309` | 新增/扩充 3 项组件测试（目标格顺序与备注与禁用、项目 chip 与行归属与管理入口、设置页计数与跳转）；`tsc` 通过、Vitest 272 项通过、Go 全量门禁通过 |
| 2026-11-22 | T09 | `scripts/e2e/ac-m6.sh`（46 项断言：登记自定义/项目、掩码路径、知识库与重叠拒绝、AC-SKM-18 分组与归属、AC-SKM-17 分发与读穿联接、移除后文件哈希不变与计数横幅、设置入口、真实 `.claude/skills` / `.agents` 快照不变）；`SKILLS_FAKE_PICK_DIR=@<队列文件>` 支持一次运行多次选目录；README 更新 | `3cb8d88` | ac-m6 46/46；全量 e2e 297 项（15/16/22/47/37/60/54/46）EXIT=0；首跑发现两处脚本问题已修（安装子目录默认落在 `.agents/skills` 需显式选 `.claude/skills`；横幅文案要按具体文本等待） |
| 2026-11-22 | T10 | `capture-m6.py`：真实窗口 8 张（管理目录 / 自定义表单含错误 / 登记项目 / 移除确认 × 浅深），替换原型图；UI 稿 9.5.1 记录逐状态差异（错误在表单顶部、掩码两段、默认安装子目录按注册表顺序、底栏沿用 M4）；DESIGN 11.6 记录实测对比度；顺带把表单底栏改为右对齐并重拍 | `6aa566f` | 对比度最低 6.03（门槛 4.5）；`tsc` 通过；Vitest 272 项通过；8 张均 1440×900 |
| 2026-11-22 | T11 | `perf-m6.sh` + `perf-gate --kind m6`：M4 基准 p95 114 ms、10 项目扫描 240 ms、保存 22.7 ms、移除 17.1 ms（门槛 200/300/100/100）；首测 6 s/3.7 s 定位到重叠判断重复 `EvalSymlinks`，改为每目录解析一次（`targetDirsText` / `targetDirsResolved`）后达标，技术设计 17.8 记录已知上限；人工项 `manual-m6.sh`（用户同意后执行）：本仓库登记为项目只读扫描 9 个技能、agnes-engine 归属 Claude Code · .claude/skills、路径脱敏，分发测试技能到临时自定义目录并读穿联接，移除登记后联接仍在、仓库与真实 `.claude` 快照不变；首跑还验证了「登记目录包含假 HOME 工具目录会被拒」 | `eee60ae` + `354f062` | perf-gate m6 通过；ac-m6 46/46 复跑；`go test ./...` 全过；人工项 11+4 项全过 |
| 2026-11-22 | T12 | task-reviewer 独立审查（0 阻断、4 需改、8 建议）；需改全部修复：错误码对齐 `not-found`、`maskPath` 单段/根目录、探测到的内置目录并入重叠规则、项目分组标题；采纳建议 R7（去掉重复读 targets）、R9（日志统一原值）、R10（打开目录入口）、R11（行身份）；R5/R6 文档澄清；R8（M4 遗留 seq 竞态）/R12 记录不改 | `0dc247a` + `c903238` | 修复后复验：Go 全量、vet/gofmt 干净、Vitest 272、ac-m6 46/46、perf-m6 四项在门槛内（109/243/25.1/14.3 ms） | T13 已交付（见下行） |
| 2026-11-22 | T13 | 收尾（B-M6-01）：`mygo.config.ts` 升 1.4.0；构建并在本机覆盖安装 1.3.2 → 1.4.0，以临时 APPDATA 种子 1.3.2 设置做升级冒烟；`docs/发布清单.md` 新增 §13 实际记录；README 安装包与 e2e 行、CLAUDE 第 3 节、路线 3.8 状态同步；账本表格按行修复 | `0ae9eba`（本地，未推送） | 安装包 `mile-knowledge-base-setup-1.4.0-amd64.exe`（6.9MB）与 exe（20.4MB）；静默安装退出 0，安装版 FileVersion / ProductVersion = 1.4.0；冒烟进程存活 9 秒、临时 `settings.json` 前后 sha256 一致；一次 APPDATA 透传失败的真实数据事件已记录（settings / log / window-state 未变，派生缓存改写后自动重建）；全量 e2e 297 项（15/16/22/47/37/60/54/46）EXIT=0（首跑误复用正式包、无自动化驱动而无效，已弃），Go 664 项（18 包）、Vitest 272 项通过 | 推送待用户确认（M6 共 39 个本地提交） |

## 9. 自查

- FR 映射：FR-SKM-CUSTOM → T02–T09；FR-SKM-PROJ → T01、T03、T05、T08、T09；AC-SKM-17、18 → T09。
- 契约先行：T00 未完成前不写业务代码；路径规则（相同 / 父 / 子 / 知识库内 / 重解析点 / UNC）每条都有验证项。
- 安全：登记只能用户在界面完成，Agent 无工具；前端拿不到绝对路径（令牌 + 脱敏显示）；移除登记不删文件；写入仍只走 M4 的分发 / 卸载 / 收编路径，备份与内置保护不变。
- 吸取 M5 补丁的教训：后台刷新不依赖 goroutine 发出的 E12；所有输入框用 `.field`。
- T00 契约相对计划草稿的细化：项目 wire spec 定为 `project:<dir>:<rel>`（最长前缀匹配，避免下标随扫描漂移）；知识库重叠检查按「读写目标双向」执行（挡住 `installRel` 恰好是知识库的情况）；`SkillLogRecord.op` 补齐 M5 的 `install`、`update` 与 M6 的 `unregister`；新增 Q-M6-7（重复加载口径不变）。
- 未覆盖：网络路径（不支持）；干净机器；项目里非约定目录的技能（不扫描）。

## 10. T12 独立审查记录（2026-11-22）

审查者：task-reviewer（B-M6-02），范围 `373f851..HEAD`，对照 PLAN/PRD §15/技术设计 13.6+17/API §14/数据库 4.6/UI 9.5/DESIGN 11.6；命令：`git diff`、`go test ./...`（662）、`go vet`、`gofmt`、`npx vitest run`（272）。结论：**阻断 0**。

| 编号 | 级别 | 问题 | 处理 |
| --- | --- | --- | --- |
| R1 | 需改 | 登记已移除 / 令牌过期返回 `invalid-input`，与 API §14.3 的 `not-found` 不符 | 已修：`unknownToolFailure`（`t-` 开头 → `not-found`「登记已移除」）、过期令牌 → `not-found`；测试同步 |
| R2 | 需改 | `maskPath` 对不足两段的本机路径原样返回（如 `D:\site`） | 已修：一段 / 根目录也走省略形式；新增 `TestMaskPathShortForms` |
| R3 | 需改 | `builtinDirs` 漏掉 `AppBuiltinRelative`（ZCode / MiniMax 的内置目录） | 已修：`TargetEnv.BuiltinDirs` 接收探测结果（已含展开后的内置目录），服务层填充；新增 `TestTargetPathDetectedBuiltinDirs` |
| R4 | 需改 | 工具技能页没有「项目 · 名称」分组标题 | 已修：选中登记 chip 时行上方显示「项目 / 自定义 · 名称 + 技能数」；UI 稿 9.5 措辞按实况修正；测试补断言 |
| R5 | 建议 | 加载校验不重建 vault/内置环境 | 文档澄清（17.3）：读取只做形状与登记间重叠，两条环境规则只在保存时强制，附 T11 性能理由 |
| R6 | 建议 | `install:` 对项目等价于 `project:<installRel>:` | API §14.3 补一句兼容口径 |
| R7 | 建议 | `runScan` 两次读 targets.json | 已修：直接用已加载列表合并 |
| R8 | 建议 | 后台 goroutine 里 `s.seq++` 无锁（M4 遗留） | 记录不改：非 M6 引入，列入后续里程碑修复 |
| R9 | 建议 | `unregister` 日志写脱敏值、其它 op 写原值 | 已修：统一写原值，读取时统一脱敏 |
| R10 | 建议 | M101 已绑定但无前端入口 | 已修：管理列表行加「打开所在目录」，测试覆盖 |
| R11 | 建议 | 行 key / 同名比较用脱敏路径，两个同名尾段的项目可能冲突 | 已修：改用 `toolId + subdir + rel` 身份 |
| R12 | 建议 | missing 禁用逻辑在两个组件各写一遍 | 记录不改：行为正确、后续如加第三处再抽公共标记 |

修复后复验：`go test ./...` 全过、`go vet` / `gofmt` 无输出、Vitest 272 通过、ac-m6 46/46、perf-m6 四项在门槛内（109 / 243 / 25.1 / 14.3 ms）。

