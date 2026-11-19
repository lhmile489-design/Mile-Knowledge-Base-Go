# M3 实施计划：其余设置、技能导入、总览、视觉与性能验收、发布

> 状态：已批准 v0.2（2026-11-19，已按独立审查修订），用户当日批准并放行 T00。
> 本次目标：交付技术设计第 9 节能力 C8（设置其余各节、技能导入、总览），完成全量视觉验收与性能验收，打出 1.0.0 Windows 安装包并在本机冒烟。闭环为「首次启动 → 新建知识库 → 导入技能 → 总览可见 → 设置五节可用 → 安装包安装、升级、卸载」。
> 上一里程碑：docs/PLAN-M2.md（已完成并推送，`f6d663a`）。

## 1. 目标、范围与授权

### 1.1 交付的可观察结果

1. 设置页 5 节齐全：导航分「应用 / AI / 知识库 / 其他」4 组，↑↓ 切换；常规、模型服务、Agents、分类与检索、关于与诊断。选择类控件即时生效，文本类控件进草稿，由页面底部统一的保存条或 Ctrl+S 保存；切换分节或关闭时有未保存草稿就弹确认；保存成功的提示 2 秒后消失。
2. 「分类与检索」列出真实一级目录和已不存在的旧配置；「进检索」开关真正影响全文检索（口径见 B-M3-01）；显示名草稿走保存条；「目录已不存在」可清除。
3. 「关于与诊断」显示版本 1.0.0、技术栈、快速上手、引擎状态与能力（可折叠）、存储位置（每行可复制）。
4. 侧栏底部有「导入技能」入口；3 步向导（选来源 → 勾选技能并填分类 → 结果页）按旧版限制导入到 `skills/<分类>/<技能名>/`，跳过原因与旧版文案一致。
5. 对话区没有消息也没有待审批时显示总览：9 块磁贴，容器宽度 <720 时 2 列、<480 时 1 列；健康磁贴打开健康面板（D-009.3 的用户入口），最近改动磁贴打开操作日志。
6. 全部 53 张基线截图有新版对应截图和并排对照记录；差异要么修掉，要么记为有意差异并经用户确认（G-VISUAL）。
7. 旧版性能基线在隔离沙箱里实测；PRD 10.1 指标按实测校准并经用户确认后，G-PERF 从警告改为阻断。
8. `mygo build` 生成 `mile-knowledge-base-setup-1.0.0-amd64.exe`；安装、从 0.1.0 升级、卸载在本机冒烟通过；docs/发布清单.md 按实际结果填写。不发 GitHub Release。

### 1.2 关联需求

FR-SET（常规、分类与检索、关于与诊断、保存条与草稿）、FR-AREA（分类配置与进检索）、FR-SKILL、FR-OVW、FR-HLTH（用户入口）、FR-MUT（最近改动磁贴打开操作日志）、FR-AI-PERM（待审批计数）、FR-AI-PROV（模型服务行的「更多」菜单，M2 记为差异）；NFR-PERF（PRD 10.1 全部指标、内存与包体）；PRD 10.3（应用级数据导入在安装版上的验证）；PRD Q7。

### 1.3 明确不做（M3 非目标）

| 不做 | 原因 / 去向 |
| --- | --- |
| GitHub Release 与任何对外发布 | 用户 2026-11-19 选择「本地安装包 + 冒烟」 |
| 代码签名 | 没有证书；SmartScreen 会警告，写进发布清单的已知风险 |
| 自动更新（mygo updates） | 技术设计第 6 节：不配置自动更新 |
| P1 修复三项（D-002：手动/自动差异、强度控件、system 提示） | D-002 原样复刻；修复需单独提案 |
| 新增旧版没有的功能 | 重写原则；视觉验收中发现的问题只修到与旧版一致 |
| 干净机器（无开发环境、无 WebView2 预装检查）上的安装 | 开发机测不出来，列为用户环境的手动项（第 6 节 V-REL-5），结果回填 |
| 读取用户真实的旧版应用数据做导入验证 | 安全约束：测试只用种子文件；真实导入由用户在安装版首次启动时自行观察 |

### 1.4 审批记录

见第 8 节。

## 2. 前置资料与现状

| 输入 | 版本 / 实际情况 | 缺口 | 是否阻断 |
| --- | --- | --- | --- |
| PRD | v1.9（T00 修订）；FR-SET、FR-SKILL、FR-OVW、AC-OVW-1 已定义 | 10.1 目标待 T12 校准（Q7 已关闭，D-013）；FR-SET 关于页的技术栈文字需改为新版（T07） | 否 |
| API.md | v1.0 + T00：新增 M74、M75，M65 补进检索副作用，方法总数 74 | 无（T00 已补齐；M74/M75 待 T07 实现） | 否 |
| 数据库设计 | v1.0 + T00：4.5 加 `search_excluded`，4.6 写明 schema +1 的删库重建 | 无（T02 按此实现） | 否 |
| 技术设计 | v1.0 + T00：第 6 节交付（1.0.0、当前用户安装、无签名）、第 10 节 G-PERF 校准说明 | G-PERF 仍为警告级（T12 校准后改阻断）；G-VISUAL 人工执行未自动化（T10） | 否 |
| DESIGN.md | v1.0；第 9 节视觉验收方法与对照清单 | 无 | 否 |
| 发布清单 | 未填写模板 | 全部 | T14 填写 |
| 现有代码 | 见下表 | — | 否 |

现有代码盘点（2026-11-19，`f6d663a`）：

| 能力 | 现状 | M3 要做 |
| --- | --- | --- |
| M14/M15 技能导入 | Go 与前端都没有 | T03、T08 全做 |
| M65 分类保存 | Go 没有；`AppConfigResult.categories` 是 `json.RawMessage` 原样透传 | T01 |
| 进检索 | 索引只用 `ignored`（系统区）过滤，`internal/index/build.go:241`；链接解析与健康也读 `ignored`（`project.go:30,76`、`health.go:49`） | T02 新增独立字段，不复用 `ignored` |
| M72/M73 最近文档 | Go 已实现（`internal/settings/settings.go`）；打开文档时已调用（`src/stores/docStore.ts:60,105`）；没有界面消费 | T09 |
| 旧版设置导入 | M0 已实现（`settings.go` `ImportLegacy`，`main.go:71`），有种子文件测试 | T14 在安装版上用临时 APPDATA 复验 |
| 技能分类数 | 根目录 tree 结果已带 `skillCategoryCount`（`internal/vault/tree.go:138`、`treeStore.ts:44`） | T09 直接用 |
| 健康面板 | M1 已实现，只能从 e2e 打开（`appStore.openHealth`） | T09 加总览入口 |
| 设置页 | M2 只有 AI 组两节；Agents 节自带保存条（`AgentsSection.tsx`）、离开守卫在 `settingsStore.requestLeave` | T04 改为页面级统一保存条 |
| 安装包 | `mygo build` 已能产出 `mile-knowledge-base-setup-0.1.0-amd64.exe`（NSIS，当前用户安装到 `%LOCALAPPDATA%\Programs`，mygo distribution.md:428-492） | T14 改版本、复验、冒烟 |
| 截图能力 | mygo automation 有 `capture` 方法（`plugins/automation/automation.go:239`，`CapturePage` 出 PNG） | T10 用它批量截图 |
| 版本号 | `mygo.config.ts` 为 `0.1.0`；Go 侧 `mygo.Application.Version()` 默认读打包信息（mygo `app.go:418`） | T00 定、T14 改 |

### 2.1 旧版实现盘点（只读；路径相对 `Mile-Knowledge-Base-master/`）

总览：

| 内容 | 位置 | 要点 |
| --- | --- | --- |
| 总览组件 | `src/renderer/src/components/workbench/OverviewGrid.tsx`（617 行） | 只在 `AgnesDrawer.tsx:564-568` 渲染：当前目标 Agent 的 transcript 为空且没有待审批时显示，否则让位给消息列表 |
| 断点 | `assets/tokens.css:2094-2109, 2264-2284` | `.overview-shell` 是 `container-type: inline-size`；`@container (max-width: 719px)` 2 列，`(max-width: 479px)` 1 列且 2×2/2×1 收为 1 格；行高 120px |
| 最近文档 2×2 | OverviewGrid 265-308；`stores/overviewStore.ts`（60 行） | 文案「最近文档」「正在读取」「读取失败：{err}」「还没有打开过文件。从左侧目录树选一个开始。」；requestSeq 丢弃过期响应 |
| 最近文档存储 | `main/services/RecentDocumentsStore.ts`（90）、`shared/recentDocuments.ts`（66）、`ipc/register.ts:571-597` | 每库 12 条、去重置顶；读取时过滤不存在的但**不回写**；记录不存在的文件返回 `not-found`「文件不存在，未记入最近文档」 |
| 文档与索引 | OverviewGrid 311-338、53-58 | 「尚未建立 / 正在建立 / 已就绪 / 建立失败」，扫描中「· 已处理 N」，就绪显示相对时间 |
| 技能分类 | OverviewGrid 341-361；`VaultService.ts:663-672` | null 时「没有 skills 目录」，否则「来自文件系统扫描」 |
| 健康 | OverviewGrid 364-402、603-613 | 按钮；四类合计；文案「索引未就绪 · 断链 N · 孤立 N · 重名 N · 缺标题 N」，全零「断链、孤立、重名、缺标题都是 0」；点击打开健康面板，固定在「未解析链接」页 |
| 草稿 | OverviewGrid 159-185、405-438、70-84 | 对 `.mile-kb/drafts` 做一次 tree 计文件数，`not-found` 记 0；监听该目录变化；文案「蒸馏学习还没有产出草稿」「在侧栏中打开草稿目录」；点击在侧栏定位 |
| 待审批 | OverviewGrid 441-464、236、253-257 | 4 个 Agent 待审批数合计；「有 N 项等待你确认」「没有等待确认的操作」；点击聚焦输入框 |
| 分区 2×1 | OverviewGrid 467-501 | 根目录的目录项做成 chip，点击在侧栏定位；空时「知识库根目录下还没有目录」 |
| Agent 状态 2×1 | OverviewGrid 504-550 | 每个 Agent「不可用 / 等待你确认 / 就绪 / 未启动」或阶段文字；状态点「引擎在线 / 引擎未就绪」 |
| 最近改动 2×1 | OverviewGrid 187-204、556-600、42-51 | 操作日志最近 5 条，随日志变化刷新；按钮「打开操作日志」 |

技能导入：

| 内容 | 位置 | 要点 |
| --- | --- | --- |
| 向导 | `components/SkillImportWizard.tsx`（249） | 3 步 source/select/done；覆盖层从标题栏下方开始，Esc 关闭；跳过原因文案 15-20：「同名技能已存在，未覆盖」「不在所选来源中」「文件过多或过大」「复制失败」；分类名前端预检「分类名不能以点开头」「分类名不能包含 \ / : * ? " < > \|」，`maxLength=80`；结果页「完成」「换个分类再导入」 |
| 前端状态 | `stores/skillImportStore.ts`（119） | 空候选提示「这个文件夹里没有找到技能。技能是含 SKILL.md 的文件夹，请选择技能所在的上一级目录。」；导入后刷新 `''`、`skills`、`skills/<分类>`；token 导入后不作废（可换分类重试） |
| 业务 | `main/services/skillImport.ts`（203） | 2000 文件 / 50 MiB；名称 ≤80、单段、不以点开头；所选目录本身有 SKILL.md 即为单个技能，否则只看下一层；超限候选在**预览阶段**就丢掉；只接受预览里出现过的名字（否则 `unknown`）；目标已存在记 `exists` 不覆盖；失败只删本次新建目录；超限文案「技能包过大（文件数或总大小超限），请确认选的不是整个仓库」 |
| IPC | `ipc/register.ts:321-361`；`validate.ts:753, 759-793`；`PickTokens.ts` | 对话框标题「选择技能所在的文件夹」；预览按 token 缓存；token 过期「技能来源已过期，请重新选择」；token 10 分钟；names ≤200 去重 |
| 入口 | `VaultSidebar.tsx:244`、`Workbench.tsx:225` | 侧栏图标按钮「导入技能」 |
| 单测 | `tests/unit/skillImport.test.ts`（276） | 见 T03 |

设置其余各节：

| 内容 | 位置 | 要点 |
| --- | --- | --- |
| 壳层 | `components/settings/SettingsView.tsx`（548） | 分节与范围胶囊 70-104；导航分组 107-115；脏判定 163-167；保存提示 2000ms 216-218；Ctrl+S `stopPropagation` 防止保存背后的文档 292-300；Esc 关闭 302-305；↑↓ 只在导航 tab 内且循环 309-320；未保存确认 `role="alertdialog"`「有 N 处未保存的改动（…）。关闭设置/切换分节会丢弃它们。」按钮「放弃改动 / 继续编辑」482-514；确认期间导航与正文 `inert`；无库提示「打开一个知识库后才能配置模型服务、Agent 与分类。常规设置不受影响。」524-526；覆盖层不盖标题栏 |
| 保存条 | `SettingsSaveBar.tsx`（64） | 有脏项才显示；「有未保存的改动：<各项>」；阻断原因禁用保存；「放弃 / 保存 / 保存中…」 |
| 草稿保存 | `stores/appConfigStore.ts:215-342` | steering 超 4000、别名超 64 拒绝而不截断；steering 等于默认走 reset；别名批量一次 `setCategories`；保存期间没再改动的草稿才清掉；提示「已保存 / 已恢复默认 / 已保存服务 / API Key 已保存」 |
| 常规 | `AppearanceSection.tsx`（196） | 主题三块缩略图「浅色 / 深色 / 跟随系统」+「当前生效：深色 / 浅色」；关窗分段「每次询问 / 挂到托盘 / 退出应用」带说明；面板宽度只读「左栏 {w}px · 文档 {w}px」与范围「左栏 192–320px，文档面板 320–800px」 |
| 分类与检索 | `CategoriesSection.tsx`（219） | 行 = 真实一级目录 ∪ 已不存在的配置（46-76）；未配置目录按 `defaultIndexedFor` 显示；开关即时提交，切回默认等于删行（`isMeaningfulCategory`）；别名进草稿；「目录已不存在」+「配置还留着」+「清除」，清除同时清掉别名草稿；脚注「关掉「进检索」后，该目录下的文件不进全文检索。…输完走底部保存条统一保存。」 |
| 分类规则 | `shared/appConfig.ts:400-491` | 见数据库设计 3.2 |
| 关于与诊断 | `AboutSection.tsx`（276） | 应用信息（技术栈 `Electron 39 · React 19 · TypeScript · CodeMirror 6`、协议「尚未确定」、GitHub 链接）、快速上手 4 步、对话引擎（状态与「已重启 N 次」、折叠的「引擎协商到的能力（排障用）」）、存储位置（每行复制，已复制 1500ms 复原） |
| 布局写入 | `stores/settingsStore.ts:56-104` | 主题与关窗即时 patch；宽度 220ms 防抖 |
| 单测 | `settingsDrafts.test.ts`（180）、`appConfig.test.ts`（364，分类部分）、`appConfigStore.test.ts`（258）、`ipcValidate.test.ts`（426，分类与设置 patch 部分） | 见 T01、T04 |

打包与首次运行：

| 内容 | 位置 | 要点 |
| --- | --- | --- |
| 打包配置 | `electron-builder.yml`（81） | `appId io.mile.knowledgebase`、NSIS、桌面快捷方式；新版用 mygo 自带 NSIS，不照搬 |
| 打包单测 | `tests/unit/packaging.test.ts`（98） | 图标存在、appId 非默认、不含 `example.com`；新版对应项在 T14 检查 |
| 首次运行 | `WelcomeView.tsx`（206）、`OnboardingGuide.tsx`（156） | 无库时显示引导；旧版引导列了 3 个步骤标签（含「导入你的技能」），新版 M0 引导只有 2 步，视觉验收时核对 |

## 3. 总路线

M0、M1、M2 已完成并推送。M3 是路线图上最后一个里程碑：代码量最少，验收和发布的工作最多。

规模参考：旧版对应源码约 3 500 行（设置其余各节约 1 900、总览约 600、技能导入约 450、零散约 550）。另有 53 张截图对照、旧版性能基线、安装包冒烟。

主要风险：
- 视觉验收会冒出 M0–M2 漏掉的细节，修复量在 T10 跑完之前无法估计。T11 先按清单定范围，范围超出本计划时停下来请用户确认。
- 旧版性能基线要跑旧版 Electron。只读复制、隔离 APPDATA，任何一步可能写到旧版目录或真实用户数据时立即停止。
- 安装包在干净机器上的表现开发机测不出来。

## 4. 任务清单

执行方式沿用：**一次只做一个任务，做完汇报并停下，等用户放行后再做下一个**。状态：待做、进行中、阻塞、待验证、已验证。

| 编号 | 目标 | 修改范围 | 依赖 | 产物 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- | --- |
| T00 | 决策落盘与契约补齐 | docs/decisions.md、docs/PRD.md、docs/API.md、docs/数据库设计.md、docs/技术设计.md | 用户批准本计划 | D-013；PRD v1.9；API 新增关于页绑定与存储路径复制；索引排除字段与 schema 版本；版本号 1.0.0 | 文档之间无冲突；新增字段与绑定能一一对应到后续任务 | 已验证 |
| T01 | 分类配置读写（M65） | `internal/vault/categories.go`（新）、`internal/service/appconfig.go`、`internal/contract` | T00 | categories.json 字节兼容读写、非法行丢弃、200 上限、默认条目不落盘 | V-CAT 全部通过 | 已验证 |
| T02 | 「进检索」生效 | `internal/index/{build,update,search}.go`、migrations、`internal/service/tools.go`（search_notes） | T01 | 独立排除字段、开关后后台重标记、搜索与 search_notes 排除 | V-IDX 全部通过；链接与健康结果不变 | 待做 |
| T03 | 技能导入后端（M14、M15） | `internal/vault/skillimport.go`（新）、`internal/service/vault.go`、`internal/contract` | T00 | 来源选择、预览缓存、限制、复制、回滚 | V-SKILL 全部通过 | 待做 |
| T04 | 设置页壳层与统一保存条 | `src/components/settings/{SettingsPage,SettingsSaveBar}`、`src/stores/settingsStore.ts`、`AgentsSection.tsx` | T00 | 4 组 5 节导航、↑↓、页面级保存条、Ctrl+S、未保存确认、2 秒提示、无库提示 | V-SET-SHELL 全部通过 | 待做 |
| T05 | 设置「常规」 | `src/components/settings/GeneralSection.tsx`（新） | T04 | 主题缩略图、关窗三档、宽度只读 | V-SET-GEN 全部通过；截图 47、48 | 待做 |
| T06 | 设置「分类与检索」 | `src/components/settings/CategoriesSection.tsx`（新） | T01、T02、T04 | 目录表、进检索开关、别名草稿、已不存在与清除 | V-SET-CAT 全部通过；截图 52 | 待做 |
| T07 | 设置「关于与诊断」 | `src/components/settings/AboutSection.tsx`（新）、关于页绑定 | T00、T04 | 版本、技术栈、快速上手、引擎状态与能力、存储位置复制 | V-SET-ABOUT 全部通过；截图 53 | 待做 |
| T08 | 技能导入向导与侧栏入口 | `src/components/SkillImportWizard.tsx`、`src/stores/skillImportStore.ts`、`VaultSidebar.tsx` | T03 | 3 步向导、跳过原因、导入后刷新树 | V-SKILL-UI 全部通过；截图 55 | 待做 |
| T09 | 总览磁贴 | `src/components/overview/*`、`src/stores/overviewStore.ts`、`ChatPane.tsx`、侧栏定位 | T08 | 9 块磁贴、容器断点、健康入口、草稿与分区定位、最近改动 | V-OVW 全部通过；AC-OVW-1；截图 10、11、56 | 待做 |
| T10 | 新版截图与并排对照 | `scripts/visual/`（新）、`design/compare/`（生成物，不提交图片以外的二进制） | T09 | 按 manifest 编号驱动新版到同状态并截图；生成本地并排对照页；差异清单 | 53 张都有新版截图或写明不能截的原因；差异清单入第 6 节 | 待做 |
| T11 | 视觉差异修复 | 按 T10 清单 | T10 | 修复项与有意差异记录 | 清单每项为「已修 / 有意差异（用户确认）」；G-VISUAL 由用户确认 | 待做 |
| T12 | 旧版性能基线与 G-PERF 校准 | `scripts/perf-legacy/`（新）、docs/PRD.md 10.1、docs/技术设计.md 第 10 节 | T11 | 旧版在同样本上的实测值；新旧对比；校准后的目标 | 用户确认校准值后 G-PERF 改为阻断，并做正反对照 | 待做 |
| T13 | M3 闭环验收与全量回归 | `scripts/e2e/ac-m3.sh`（新）、`scripts/test-e2e.sh` | T12 | AC-M3、ac-m0..m3 全量回归 | `bun run test:e2e` 退出码 0 | 待做 |
| T14 | 1.0.0 安装包与本机冒烟 | `mygo.config.ts`、`resources/`、docs/发布清单.md、README | T13 | 安装包；安装、升级、卸载冒烟；发布清单 | V-REL 中本机项全部通过；用户环境项列明 | 待做 |
| T15 | 整体独立审查 | 本计划第 6 节、审查报告 | T14 | 审查结论与修复 | 阻断与需改全部处理 | 待做 |
| T16 | 收尾 | README、CLAUDE.md、本计划、docs/PLAN-M0.md 路线 | T15 | 文档同步；推送前确认 | 用户确认后推送 | 待做 |

依赖顺序 T00 → T01 → … → T16，按「一次一个」串行。T12 排在视觉修复之后，是为了让性能测的是最终界面。

## 5. 单任务精确交接

> 通用纪律：Go 业务逻辑先写失败测试；`.mile-kb/` 下的文件按数据库设计 3.0.1 字节兼容；错误码按 API 第 1.3 节；中文 UI、无 emoji；测试和脚本只用临时目录与临时 APPDATA，不读写 `%APPDATA%\mile-knowledge-base` 与新版真实应用数据；旧版仓库只读；提交按 M0 检查清单，提交前扫描密钥与真实数据。

### T00 决策落盘与契约补齐

- **D-013**（新增）记录用户 2026-11-19 的 4 项选择与本计划的 2 项契约修订：
  1. 「进检索」真正生效，范围按 B-M3-01 的最终批准写；
  2. 旧版性能基线在 M3 实测，校准后 G-PERF 改为阻断；
  3. 发布只做本地安装包与冒烟，不发 GitHub Release；
  4. 首个发布版本号 1.0.0；
  5. 关于页新增绑定（见下）；
  6. 存储位置在界面上显示脱敏形式，复制由 Go 写剪贴板（见下）。
- **PRD v1.9**：FR-AREA 的「进检索即时生效」改为按 D-013 的实际口径，删掉「M1 与旧版一致不生效」的过渡说明；FR-SET 关于页写明技术栈文字为新版实际值（与旧版 `Electron 39 · …` 不同，属有意差异）；Q7 记录测量计划。
- **API**：
  - 新增 M74 `SettingsService.aboutInfo()` → `{version, techStack, engine: EngineStatus, capabilities: EngineCapabilities, restartCount, storage: [{kind, label, display}]}`。`display` 是脱敏路径（`<home>`、`<vault>` 替换，PRD 10.2）。`kind` 枚举：`vault-root`、`vault-settings`、`providers`、`app-data`、`logs`、`credentials`（固定文字「Windows 凭据管理器」，不是路径，D-011）。
  - 新增 M75 `SettingsService.copyStoragePath(kind)`：Go 把真实路径写入剪贴板，返回 `{ok}`；`credentials` 返回 `unsupported`。理由：API 第 1 节约定不向前端发完整路径，而 PRD FR-SET 要求可复制。
  - M65 细化：出参复用 `AppConfigResult`；进检索变化后由 Go 触发后台重标记，结果经 E02 `index-status` 反映。
  - 同步 4 处：§2.2 接口目录加两行，并把方法总数（第 189 行）改为 74（M19 仍是协议端点）；§3 映射表注明两者没有旧版频道（旧版关于页直接读前端 store）；§4.8 加入参校验与出参；§10 范围声明。
- **数据库设计**：4.5 `documents` 表新增 `search_excluded INTEGER NOT NULL DEFAULT 0`，与 `ignored` 分开；`meta.schema_version` +1，按第 6 节「不符则删库重建」处理；写明排除只作用于全文检索。
- **技术设计**：第 10 节 G-PERF 的「校准后阻断」写明在 T12 执行；第 6 节交付补「版本 1.0.0、当前用户安装、无签名」。
- 验证：文档互相引用一致（编号、字段名、版本）；`git diff` 只含 docs。

### T01 分类配置读写（M65）

- 读：`.mile-kb/settings/categories.json`，顶层数组；按数据库设计 3.2 逐字段归一：`directory` 单段、≤64、不含 `[\\/:*?"<>|]`、非 `.`/`..`、大小写不敏感去重；`alias` ≤64；`area` 合法枚举或 null；缺 `indexed` 用 `defaultIndexedFor`（系统区为 false）；`order` 非有限数回 0；200 上限。非法行丢弃不报错（旧版 `validate.ts:567-577`）。
- 写：只写 `isMeaningfulCategory` 为真的条目；原子写，`JSON.stringify(…, null, 2) + "\n"` 同款字节；未知字段保留规则按 3.0.1。
- `AppConfigGet` 返回解析后的 categories（不再原样透传 RawMessage，契约字段名不变）。
- 测试先行，移植 `appConfig.test.ts` 分类部分与 `settingsDrafts.test.ts` 的 `defaultIndexedFor`、`isMeaningfulCategory` 用例；加一条与旧版文件的字节往返用例（用种子文件，不读旧版真实库）。

### T02 「进检索」生效

- 按 B-M3-01 的批准范围实现。计划建议的范围：**只排除全文检索**——搜索面板（M24）与 Agent 的 `search_notes`；链接解析、反向链接、健康检查、文件树不受影响。
- 实现：`documents.search_excluded` 由一级目录的 `indexed=false` 决定；首建、增量更新、M65 保存后都按当前 categories 计算；M65 改变开关时只做 `UPDATE documents SET search_excluded=… WHERE path LIKE 'dir/%'`，不重建全部 FTS；搜索 SQL 增加 `search_excluded = 0`。
- 系统区目录原本就 `ignored`，开关对它们的显示与旧版一致（默认关）；打开系统区开关不让系统区进检索（系统区规则优先，PRD FR-AREA），界面在 T06 说明。
- 测试：关掉 `projects` 后搜不到其中文档、wikilink 仍能解析、健康结果不变；再打开后恢复；重启后保持；schema 版本升级时旧库被重建。
- 性能：1 万文档库切换一次开关的耗时记录到第 6 节（预期远小于重建）。

### T03 技能导入后端（M14、M15）

- 移植 `skillImport.ts`：`probe`（忽略 `.ds_store`/`thumbs.db`/`desktop.ini`）、`walk`（跳过符号链接与点号项，超过 2000 文件或 50 MiB 报 too-large 并用旧版文案）、`isSingleSegmentName`（≤80、单段、不以点开头）、`preview`（所选目录本身含 SKILL.md 即单个技能，否则只看下一层；超限候选在预览阶段丢弃）、`import`（只接受预览中的名字，否则 `unknown`；目标经路径校验落在 `skills/<分类>/<名>/`；已存在记 `exists`；失败只删本次新建的目录）。
- M14：系统选目录对话框标题「选择技能所在的文件夹」；返回 token（复用 M12 的 10 分钟 token 机制，前端拿不到绝对路径）与候选 `{name, fileCount}`；预览按 token 缓存。
- M15：token 过期返回「技能来源已过期，请重新选择」；token 导入后不作废；names ≤200 去重。导入走文件复制，不经变更计划（旧版即如此，导入只新建、不覆盖）。完成后发 E01 让树刷新。
- 测试先行，移植 `skillImport.test.ts` 全部场景。Windows 上建符号链接可能需要开发者模式：用例先尝试 `os.Symlink`，失败时改用目录联接验证「跳过链接」，两者都不可用时 `t.Skip` 并写明原因（不伪造通过）。

### T04 设置页壳层与统一保存条

- 导航：「应用」常规；「AI」模型服务、Agents；「知识库」分类与检索；「其他」关于与诊断。每节的范围胶囊按旧版 `SettingsView.tsx:70-104`。
- ↑↓ 只在导航 tab 内循环切换；Esc 关闭；覆盖层不盖标题栏。
- 统一草稿：steering（4 个 Agent）与分类显示名；页面底部一个保存条，列出脏项；Ctrl+S 保存并 `stopPropagation`；超限拒绝而不截断；保存期间又被改动的草稿不清；保存成功提示 2 秒消失。
- 切换分节或关闭时有草稿：`role="alertdialog"` 确认，文案与按钮照旧版；确认期间导航与正文 `inert`，关闭后焦点归还。
- 把 T18 在 AgentsSection 里的局部保存条迁到页面级，`settingsStore` 的离开守卫扩展到 5 节。
- 无活动库时只有常规与关于可用，提示文案照旧版。
- 测试：脏项判定、Ctrl+S、确认流程、超限拒绝的 vitest；M2 已有的 Agents 草稿用例不能削弱。

### T05 设置「常规」

- 主题三块缩略图 +「当前生效」；关窗三档分段 + 说明；面板宽度只读与范围文字；选择即时经 M61 保存（E04/E05 回显）。
- 截图 47、48。

### T06 设置「分类与检索」

- 表格行：真实一级目录（来自根目录 tree）∪ 配置中已不存在的目录；每行图标、目录名、显示名输入（草稿）、进检索开关（即时经 M65）。
- 未配置目录按默认值显示；切回默认等于删掉该条配置；已不存在的目录显示「目录已不存在」「配置还留着」和「清除」，清除同时清掉该行显示名草稿。
- 系统区行的开关说明「系统区不进检索」（按 T02 的规则）；脚注文案按 D-013 定稿后的口径改写旧版那句。
- 截图 52。

### T07 设置「关于与诊断」

- 应用信息：版本（M74，1.0.0）、技术栈（新版实际文字）、仓库链接用 `OpenExternal`；快速上手 4 步照旧版文案。
- 对话引擎：状态点与文字、重启次数、可折叠的能力列表（API M38 的固定能力集）。
- 存储位置：每行显示脱敏路径和复制按钮（M75），已复制 1500ms 后复原；凭据行显示「Windows 凭据管理器」，无复制。
- 剪贴板验证：M75 写剪贴板的部分经可替换接口注入，Go 单测断言写入的是真实路径、`credentials` 返回 `unsupported`。e2e 先试 automation `eval` 里的 `navigator.clipboard.readText()`；WebView2 拒绝权限时改为人工确认一次，不伪造通过。仓库里目前没有读剪贴板的先例。
- 截图 53（技术栈与路径文字属有意差异）。

### T08 技能导入向导与侧栏入口

- 侧栏底部图标按钮「导入技能」（D-009.4 计划的入口）；无活动库时禁用。
- 3 步：选来源（M14，空候选提示照旧版）→ 勾选并填分类（前端预检两条文案，`maxLength=80`，最终以 Go 为准）→ 结果页（导入数、跳过列表与 4 种原因文案、「完成」「换个分类再导入」）。
- 覆盖层从标题栏下方开始，Esc 关闭，焦点归还到入口按钮。
- 导入后刷新 `''`、`skills`、`skills/<分类>` 三层树。
- 截图 55；e2e 覆盖一次真实导入（临时来源目录）。

### T09 总览磁贴

- 显示条件：当前 Agent 会话没有消息也没有待审批（与旧版 `AgnesDrawer.tsx:564-568` 一致），发出第一条消息后让位。
- 9 块磁贴的数据来源：最近文档 M72（读取失败与空状态文案照旧版，requestSeq 丢弃过期响应）；文档与索引 E02；技能分类 `skillCategoryCount`；健康 M28（打开健康面板，固定在未解析链接页）；草稿 = 对 `.mile-kb/drafts` 的一次 tree，`not-found` 记 0，监听 E01；待审批 = agentStore 中 4 个 Agent 的待审批数（点击聚焦输入框）；分区 = 根目录的目录项；Agent 状态 = E10 + agentStore；最近改动 = M31 最近 5 条 + E03 刷新 +「打开操作日志」。
- 侧栏定位：新增「展开到某路径并滚动可见」的能力，供草稿与分区磁贴使用。新版侧栏已显示「系统配置」分区（`src/components/VaultSidebar.tsx:33,36`，与旧版一致），草稿磁贴定位到其中的 `.mile-kb/drafts`。
- 容器查询：`.overview-shell` 用 `container-type: inline-size`，断点 719/479，与欢迎页的容器互不影响。
- AC-OVW-1：发起一次审批卡后磁贴计数为 1 且带警示边框，处理后归零（e2e 用 M2 的假供应商）。
- 截图 10、11、56。

### T10 新版截图与并排对照

- 脚本 `scripts/visual/capture-new.sh`：临时 vault（与旧版截图同一套样本生成方式，见 `design/baseline/capture.cjs` 文件头）+ 临时 APPDATA + 假供应商；按 `design/baseline/manifest.json` 的编号逐个驱动到同状态（automation `eval` 设置 store 与点击），用 automation `capture` 存 PNG 到 `design/compare/new/<id>.png`。
- 生成 `design/compare/index.html`：左旧右新并排，本地打开，不外发。
- 58、59（原生对话框）不截，按 FR-WIN 文字验收；60（关闭有未保存修改的标签）在新版补截，旧版没有对应图，按旧版 `TabStrip` 行为核对。
- 产出差异清单：编号、差异描述、类别（布局 / 令牌 / 文字 / 状态 / 字体渲染）、建议（修 / 有意差异 / 仅字体渲染）。
- 不提交生成的 PNG 与 HTML（加入 `.gitignore`），清单写进第 6 节。

### T11 视觉差异修复

- 已知待处理项（来自 M2 账本）：模型服务行的「更多」菜单（更换 Key、移到首位、删除，PRD FR-SET）；截图 34–46 的对话视觉对照；引擎离线口径（截图 40 与 API M38 的差异，已有记录，需要用户确认保留）。
- 其余项按 T10 清单。每项修完重新截图对照；「有意差异」逐条请用户确认。
- 范围控制：修复只到与旧版一致；如果清单显示修复量明显超出一个任务，先拆成 T11a、T11b… 并请用户确认后再做。

### T12 旧版性能基线与 G-PERF 校准

- 沙箱：把样本知识库复制到临时目录；用旧版仓库自带的 `node_modules/electron/dist/electron.exe` 加载本仓库的驱动脚本，脚本里 `app.setPath('appData' / 'userData')` 指向 `os.tmpdir()` 下新建的沙箱（与 `design/baseline/capture.cjs:58-61` 同一做法）；不修改旧版仓库任何文件，脚本放在本仓库 `scripts/perf-legacy/`。
- 指标（同样本、同窗口尺寸）：冷启动到侧栏可点、大库首屏、检索 p95（典型 / 大）、外部改 1000 文件期间的长任务、历史查看器首帧（旧版已有 172–189ms）、空闲与打开典型库后的内存、安装包体积。旧版无法自动化注入的指标改用同链路代理指标并写明。
- 新版同项复测；结果表写进第 6 节；PRD 10.1 的目标值按「新版实测、旧版基线、原目标」三者给出校准建议，**用户确认后**再改 PRD 与技术设计第 10 节，把 G-PERF 改为阻断，并做一次「故意超标失败、正常通过」的正反对照。
- 前置断言：驱动脚本启动后先检查 `app.getPath('appData')` 与 `app.getPath('userData')` 都在本次沙箱内，不在则立即退出，不加载任何库。
- 旧版仓库快照：开跑前记录 `git -C <旧版> status --porcelain`，跑完再比对。2026-11-19 核对时旧版仓库已有一条与本项目无关的既有改动 `D commit-msg.txt`，快照会把它算进基线；快照之外出现任何变化即停止并汇报。
- 任何一步检测到会写入旧版仓库或真实 `%APPDATA%` 时立即停止并汇报。

### T13 M3 闭环验收与全量回归

- `scripts/e2e/ac-m3.sh`：临时 APPDATA 首次启动 → 引导新建知识库 → 导入技能（临时来源）→ 总览显示技能分类数与最近文档 → 设置五节可打开、常规改主题即时生效、分类改显示名经保存条保存并在侧栏可见、关掉进检索后搜不到、关于页显示 1.0.0 → 重启后保持。
- `bun run test:e2e` 跑 ac-m0、ac-m1、ac-m2、ac-m3 全量，退出码 0。

### T14 1.0.0 安装包与本机冒烟

- `mygo.config.ts` 版本改 1.0.0；核对图标、显示名、安装目录名；`bun run build` 产出安装包，记录体积。
- 本机冒烟（**执行前再向用户确认一次**：安装会写 `%LOCALAPPDATA%\Programs` 与开始菜单）：先装 0.1.0 安装包 → 再装 1.0.0（升级到同一目录）→ 用临时 APPDATA 启动安装版，检查首次运行、旧版设置导入（种子文件）、打开样本库 → 卸载后程序目录消失、应用数据保留（mygo distribution.md:541）。
- 填写 docs/发布清单.md：范围、环境、实际证据、已知风险（无签名、未在干净机器验证）、回退方式（卸载后装回 0.1.0；库内数据新旧版兼容）。
- README 增加「安装」一节。

### T15 整体独立审查

- 范围：M3 全部提交。重点：技能导入的路径与复制边界、分类配置字节兼容、进检索对检索与 Agent 工具的一致性、关于页不泄露完整路径、设置草稿的数据丢失风险、安装包内容（不夹带 design/、spikes/、测试数据）。
- 红线核对：旧版仓库的 `git status --porcelain` 与 T12 开跑前的快照一致；真实 `%APPDATA%\mile-knowledge-base` 与新版真实应用数据目录在本里程碑内没有被读写。
- 修复上限 3 轮；无法独立审查时如实标为自审，等用户接受。

### T16 收尾

- README、CLAUDE.md、本计划账本、docs/PLAN-M0.md 路线表；列提交；询问后推送。

## 6. 验证与验收

| 编号 | 给定条件 | 操作 | 预期结果 | 方式 |
| --- | --- | --- | --- | --- |
| V-CAT-1 | categories.json 含非法目录名、重复（大小写不同）、未知 area | 读取 | 非法行丢弃、重复只留第一个、area 归 null，不报错 | Go |
| V-CAT-2 | 201 条 | 读取 / 保存 | 截到 200 | Go |
| V-CAT-3 | 一条与默认无异的条目 | 保存 | 不落盘 | Go |
| V-CAT-4 | 旧版写的 categories.json 种子 | 读写往返 | 字节一致（数据库设计 3.0.1） | Go |
| V-IDX-1 | 关掉 `projects` 的进检索 | 搜其中独有的词 | 搜索面板与 `search_notes` 都无结果 | Go + e2e |
| V-IDX-2 | 同上 | 打开指向 `projects` 文档的 wikilink、看健康面板 | `search_excluded` 不影响 `ignored` 链路：链接解析与健康四类结果和关闭开关前相同 | Go |
| V-IDX-3 | 再打开开关、重启应用 | 搜索 | 恢复命中；重启后开关与结果保持 | Go + e2e |
| V-IDX-4 | 旧 schema 版本的索引库 | 启动 | 删库后台重建，界面可用 | Go |
| V-SKILL-1 | 所选目录含 SKILL.md | 预览 | 单个技能，名称取目录名 | Go |
| V-SKILL-2 | 所选目录下一层有 3 个技能、1 个点号目录、1 个超 2000 文件的目录 | 预览 | 只列 2 个合法候选 | Go |
| V-SKILL-3 | 目标 `skills/<分类>/<名>` 已存在 | 导入 | 记 `exists`，原文件不变 | Go |
| V-SKILL-4 | 复制中途失败 | 导入 | 只删除本次新建的目录 | Go |
| V-SKILL-5 | 分类名 `.x`、`a/b`、81 字符 | 导入 | `invalid-input`，磁盘无变化 | Go |
| V-SKILL-6 | 预览后磁盘上新出现的技能 | 导入该名 | `unknown` | Go |
| V-SKILL-7 | token 超过 10 分钟 | 导入 | 「技能来源已过期，请重新选择」 | Go |
| V-SKILL-8 | 来源含符号链接与 `.git` | 导入 | 链接与点号项不复制 | Go（环境不支持时如实 skip） |
| V-SKILL-UI-1 | 真实来源目录 | 走完 3 步 | 树出现新技能；结果页列出跳过原因 | e2e |
| V-SET-SHELL-1 | 有 steering 与分类显示名两处草稿 | 切换分节 | 弹确认，列出 2 处；「继续编辑」留在原处 | vitest + e2e |
| V-SET-SHELL-2 | 有草稿，文档面板也有未保存修改 | Ctrl+S | 只保存设置草稿，文档不被保存 | e2e |
| V-SET-SHELL-3 | 显示名 65 字符 | 保存 | 拒绝，保存条显示原因 | vitest |
| V-SET-SHELL-4 | 保存成功 | 等待 | 提示 2 秒后消失 | vitest |
| V-SET-SHELL-5 | 无活动库 | 打开设置 | 只有常规与关于可用，提示文案照旧版 | e2e |
| V-SET-GEN-1 | 选深色 | 点击 | 即时生效，重启保持 | e2e |
| V-SET-CAT-1 | 配置里有已删除的目录 | 打开分类与检索 | 显示「目录已不存在」，清除后配置消失 | e2e |
| V-SET-ABOUT-1 | 关于页 | 打开 | 版本 1.0.0；存储路径为脱敏形式；页面与绑定返回值里没有用户主目录与库的绝对路径 | Go + e2e |
| V-SET-ABOUT-2 | 点存储位置复制 | 读剪贴板 | 是真实路径；按钮 1500ms 后复原 | Go（注入剪贴板）+ e2e；读剪贴板被拒时人工确认 |
| V-OVW-1 | 新会话无消息 | 打开对话区 | 显示 9 块磁贴 | e2e |
| V-OVW-2 | 对话区宽 700 / 460 | 观察 | 2 列 / 1 列 | e2e |
| V-OVW-3 | 健康磁贴 | 点击 | 健康面板打开在未解析链接页 | e2e |
| V-OVW-4 | 最近文档有 1 条已被外部删除 | 打开总览 | 不显示该条；`recent-documents.json` 中仍在 | Go + e2e |
| AC-OVW-1 | 有 1 张未处理的审批卡 | 打开总览 | 待审批磁贴为 1 且带警示边框；处理后归零 | e2e |
| V-A11Y-1 | 设置页、技能导入向导、总览 | 只用键盘操作；开启减少动效 | 按 DESIGN.md 7.2 逐项：图标按钮有可读名、焦点环可见、确认框期间导航与正文 inert、覆盖层关闭后焦点归还、分段控件角色正确、循环动画停止、对比度达标 | e2e + 人工 |
| V-VIS-1 | 53 张基线截图 | T10 对照 | 每张有新版截图或不截原因；差异清单每项有结论 | 人工（用户确认） |
| V-PERF-1 | 同样本 | T12 | 旧版基线有实测值或写明代理指标 | 脚本 |
| V-PERF-2 | 校准后 | 故意超标 / 正常 | G-PERF 前者失败、后者通过 | 脚本 |
| AC-M3-1（本计划自定的闭环用例） | 临时 APPDATA | 首次启动 → 新建库 → 导入技能 → 总览 → 设置五节 → 重启 | 每步与 PRD/截图一致 | e2e |
| V-REL-1 | 1.0.0 安装包 | 安装 | 装到 `%LOCALAPPDATA%\Programs\…`，开始菜单有快捷方式 | 本机 |
| V-REL-2 | 已装 0.1.0 | 安装 1.0.0 | 原目录升级，关于页显示 1.0.0 | 本机 |
| V-REL-3 | 安装版 + 临时 APPDATA + 种子旧版设置 | 首次启动 | 主题、关窗、布局、最近库按种子导入；只提示一次 | 本机 |
| V-REL-4 | 卸载 | 执行 | 程序目录消失，应用数据保留 | 本机 |
| V-REL-5 | 干净的 Windows 11 | 安装并打开样本库 | 能运行；WebView2 缺失时的表现记录在发布清单 | 用户环境 |
| V-MAN-1 | 13 项窗口手动确认（spikes/r4-window/RESULT.md 第 4 节） | 用户操作 | 结果回填 | 用户环境 |
| V-MAN-2 | 真实供应商（R-4） | 用户填真实 key 发长回答 | 流式流畅，结果回填 | 用户环境 |

性能（PRD 10.1；T12 填写）：

| 指标 | 原目标 | 新版实测 | 旧版基线 | 校准后目标（用户确认） |
| --- | --- | --- | --- | --- |
| 冷启动到工作台可操作 | ≤1.5s（典型） | M0：1101ms；M1：典型 910ms、大 1093ms | 待测 | 待定 |
| 大知识库首屏 | ≤300ms | M0：5 000 项单目录数据 320ms 到位（略超） | 待测 | 待定 |
| 检索响应 p95 | ≤100ms 典型 / ≤200ms 大 | M1：2.7ms / 45.3ms | 待测 | 待定 |
| 外部改动同步 | ≤1s | 未单独计时（M0/M1 只测了期间长任务） | 旧版承诺 1 秒 | 待定 |
| 批量变更不阻塞 | 单次任务 ≤200ms | M0、M1：1 000 文件期间 0 长任务 | 待测 | 待定 |
| 流式输出 / 按键 p90 | 60fps / ≤16ms | M2：0 慢帧 / 8ms | 待测 | 待定 |
| 历史查看器首帧 | ≤100ms | M2：62.4ms | 172–189ms | 待定 |
| 长列表滚动 | 60fps | M0：树 5 000 项 0 长任务；M2：消息虚拟化最多 9 项；搜索与日志 200 条未单独测 | 未虚拟化 | 待定 |
| 内存与包体 | 低于旧版 | 待测 | 待测 | 待定 |

## 7. 协作、审查与执行方式

- 一次一个任务；每完成汇报并停下。
- 测试先行：Go 侧分类、索引排除、技能导入一律先写测试；移植旧版单测按 2.1 清单。
- TDD 例外：契约文档（T00）、纯视觉外壳（T05、T07 的布局，逻辑仍有单测）、截图与性能脚本本身（T10、T12）、发布清单（T14）。
- 独立审查：T15 做整体审查；T03 完成后可以按需加一次技能导入的专项审查（写文件、路径边界），是否做由用户在 T03 汇报时决定。
- 写磁盘的边界：技能导入只写 `skills/` 下本次新建的目录；分类只写 `.mile-kb/settings/categories.json`；其余任务不写知识库。
- 提交：每个任务一次，不推送；M3 末询问。

## 8. 来源基线与批准记录

| 资料 | 版本 / 日期 | 适用范围 |
| --- | --- | --- |
| PRD | v1.9 | FR-SET、FR-AREA、FR-SKILL、FR-OVW、10.1、10.3 |
| API | v1.0 + T00（M74、M75，M65 细化） | M14、M15、M60–M66、M72–M75 |
| 数据库设计 | v1.0（T00 改 4.5） | 3.2、4.5 |
| 技术设计 | v1.0 | 第 6 节交付、第 10 节门禁 |
| DESIGN.md | v1.0 | 第 9 节视觉验收 |
| decisions | D-001–D-012（T00 加 D-013） | D-002、D-006、D-009、D-011 |
| 旧版代码 | v2.7.0，只读 | 2.1 盘点 |
| mygo | v0.10.0 | distribution.md 第 Windows 节、automation `capture` |

| 批准标识 | 内容 | 依据和时间 | 状态 |
| --- | --- | --- | --- |
| B-M3-01 | 「进检索」真正生效，**排除范围只含全文检索**（搜索面板与 `search_notes`），链接解析、反向链接、健康检查不受影响。说明：提问时选项说明写的是「不进检索、不进链接图与健康统计」；核对新版索引后发现，链接与健康若一并排除，指向这些目录的 wikilink 会全部变成未解析，因此改为只排除全文检索。范围差异已在 v0.1 与 v0.2 汇报中写明 | 用户 2026-11-19 选择「真正生效」；用户 2026-11-19 批准 v0.2 并放行 T00 | 已确认 |
| B-M3-02 | 旧版性能基线在 M3 实测，校准值经用户确认后 G-PERF 改为阻断 | 用户 2026-11-19 选择 | 已确认 |
| B-M3-03 | 发布只做本地安装包与本机冒烟，不发 GitHub Release；T14 安装前再确认一次 | 用户 2026-11-19 选择 | 已确认 |
| B-M3-04 | 首个发布版本号 1.0.0 | 用户 2026-11-19 选择 | 已确认 |
| B-M3-05 | 新增 M74 关于页信息、M75 存储路径复制；界面显示脱敏路径，复制由 Go 写剪贴板 | 用户 2026-11-19 批准 v0.2 并放行 T00 | 已确认 |
| B-M3-06 | M3 范围 = 第 1.1 节；非目标 = 第 1.3 节；任务与顺序 = 第 4 节；TDD 例外与审查安排 = 第 7 节；执行节奏「一次一个任务」 | 用户 2026-11-19 批准 v0.2 并放行 T00 | 已确认 |

## 9. 恢复入口与执行账本

### 下次会话先读

- 当前阶段：T01 已完成并提交，等待用户放行 T02。
- 下一个任务：T02（「进检索」生效）。
- 工作区：`main` 与 `origin/main` 同在 `3aa3fba`；本计划尚未提交。
- 待决事项：无（B-M3-01、B-M3-05、B-M3-06 已确认）；其余设计口径随各任务确认。

| 时间 | 任务与状态变化 | 文件 / 版本范围 | 验证与证据 | 待决事项 | 下一任务 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-19 | 独立审查（task-reviewer，只读）0 阻断、2 需改、5 建议，主会话逐条复核后全部采纳：**需改** ①T12 补前置断言（沙箱内 appData/userData）与旧版仓库快照比对；②V-SET-ABOUT-2 补剪贴板验证通道（Go 注入 + e2e readText，权限被拒时人工确认）；**建议** ③新增 V-A11Y-1（DESIGN.md 7.2）；④T09 写明新版侧栏已显示系统配置分区；⑤T15 加旧版仓库与真实应用数据的红线核对；⑥T00 写明 API 四处同步；⑦V-IDX-2 改为验证 `ignored` 链路不受影响。另修 2 处行号偏差（OverviewGrid 265、603）。复核时新发现：旧版仓库已有既有改动 `D commit-msg.txt`（非本轮造成，来源无法确认，未处理），T12 改为快照比对 | docs/PLAN-M3.md v0.2 | 审查抽查：旧版引用 20 处中 18 处属实（2 处行号差 1 行已修）、新版代码事实 9 处、mygo 文档 3 处、契约一致 4 处全部属实；审查标「未核实」的旧版 `electron.exe` 已确认存在 | B-M3-01、05、06 | T00（待批准） |
| 2026-11-19 | T01 完成（分类配置读写 M65） | `internal/vault/categories.go`（新）：`Category`、`ParseCategories`（逐项容错——非对象/类型不符/非法目录名丢行；目录 trim、单段 ≤64、禁 `/:*?"<>\|`、非 `.`/`..`、大小写不敏感去重；alias trim 后按 rune 截 64；area 只接受 9 个合法值否则 null；缺 `indexed` 按 `DefaultIndexedFor`（系统区 false）补；order 非有限数归 0、其余按 `Math.floor(x+0.5)` 取整；上限 200）、`NormalizeCategories`（M65 类型化入参同一套规则，indexed 取调用方真实值）、`ReadCategories`（缺失/损坏 → 空表）、`WriteCategories`（归一后丢弃与默认无异的行、原子写、`JSON.stringify(…, null, 2)+"
"` 同款字节、字段序 directory/alias/area/indexed/order、`SetEscapeHTML(false)`）、`DefaultIndexedFor`、`IsMeaningfulCategory`、`IsAreaID`、`CategoriesPath`。`internal/service/appconfig.go`：M62 返回解析后的 `categories`（不再透传 RawMessage）；新增 M65 `AppConfigSetCategories`（写盘后回读，失败 `io-failure`）；M66 响应同样带 `categories`；所有 `AppConfigResult` 构造初始化空数组，线上不会是 null。`internal/contract`：`Categories []CategoryConfig` 与新类型 `CategoryConfig{directory, alias, area *string, indexed, order}`，删除不再使用的 `encoding/json` 导入。`src/mygo.ts` 重新生成（M65 绑定 + 类型）。测试：vault 新增 8 项（非法行/重复/缺 indexed 默认/area 非法→null/order 取整与字符串→0/200 上限/非数组/别名截断/类型化入参/默认值判定/缺失与损坏/写入字节与默认行过滤/旧版种子字节往返）；service 新增 M65 用例（写盘字节、非法行丢弃、默认行不落盘、回读一致、无库失败）并更新别名测试里 categories 的旧断言。**差异/待办**：类型化入参无法区分「缺 indexed」与「false」，按调用方真实值处理（前端总会带完整行，T06 保证）；alias 截断按 rune（旧版按 UTF-16 单元，仅星平面字符有差异）；进检索尚未接索引（T02） | `internal/vault/{categories.go,categories_test.go}`、`internal/service/{appconfig.go,appconfig_test.go}`、`internal/contract/contract.go`、`src/mygo.ts`、docs/PLAN-M3.md | `go test ./...` **494 项通过（+10）**；`go vet ./...`、`gofmt -l` 无输出；`npm run test:web` 152 项 + `check:bindings` 通过；`go build ./...` 通过。e2e 未跑：T01 无界面变化，全量回归在 T13 | 无 | T02（待放行） |
| 2026-11-19 | T00 完成（决策落盘与契约补齐） | **D-013**：4 项用户选择（进检索只排除全文检索、M3 实测旧版基线、本地安装包 + 冒烟、版本 1.0.0）+ 2 项契约修订（M74/M75、`search_excluded` 与 schema +1）。**PRD v1.9**：FR-AREA 改写进检索语义（含系统区优先与旧版差异说明）、FR-SET 关于页写明脱敏显示与宿主剪贴板、Q7 关闭、修订记录。**API**：§2.2 加 M74/M75 并把方法总数 72 → 74；§3 映射表加两行「无旧版频道」；§4.8 加 M74/M75 行、M62 的 categories 改为解析后数组、M65 加进检索副作用；§10 声明改为 M0–M2 已实现、M74/M75 与 M65 副作用待实现。**数据库设计**：4.5 加 `search_excluded` 列与独立语义说明、4.6 迁移段写明首个升级用例删库重建。**技术设计**：第 6 节交付改为 1.0.0 / 当前用户安装 / 无签名；第 10 节 G-PERF 写明 T12 校准与正反对照 | docs/{decisions.md,PRD.md,API.md,数据库设计.md,技术设计.md}、docs/PLAN-M3.md | docs-only：UTF-8 无 BOM、LF；`git diff` 仅这 5 个文件；随后做跨文档一致性核验（PRD/API/数据库设计/技术设计/计划互引，逐条 grep 核对） | 无 | T01（待放行） |
| 2026-11-19 | 计划批准（B-M3-01、B-M3-05、B-M3-06 确认） | docs/PLAN-M3.md v0.2 | 用户 2026-11-19「批准并放行 T00」 | 无 | T00 |
| 2026-11-19 | 计划草案完成 | docs/PLAN-M3.md v0.1 | 用户 4 项选择（进检索生效、M3 测旧版基线、本地安装包 + 冒烟、1.0.0）；旧版盘点由只读子代理完成，关键 file:line 已写入 2.1；新版现状按代码核对（见第 2 节表） | B-M3-01、05、06 | T00（待批准） |

## 10. 自查

- 覆盖：PRD 10.3 的键盘与可访问性由 V-A11Y-1 覆盖；PRD FR-SET（常规、分类与检索、关于与诊断、保存条）、FR-SKILL、FR-OVW、FR-HLTH 入口、AC-OVW-1、NFR-PERF 全部指标、PRD 10.3 安装版导入、Q7 都有任务与 V 项；M2 遗留的模型服务「更多」菜单、截图 34–46、引擎离线口径进 T11；13 项手动确认与 R-4 列为用户环境项（V-MAN）。
- 接口一致：M14、M15、M65、M72、M73 来自 API 2.2；M74、M75 是新增，T00 定义后才进入 T07；索引新字段在数据库设计 4.5 定义后才进入 T02。
- 未定值：进检索的排除范围（B-M3-01）、校准后的性能目标（T12 后由用户定）、视觉修复量（T10 后才知道）都没有在计划里预先填数字。
- 已知风险：视觉修复量不可预估（T11 可拆分）；旧版基线需要跑旧版 Electron（只读、隔离，触发写入即停）；干净机器未验证；无代码签名。
- 安全：测试与脚本不碰真实应用数据与凭据；关于页不向前端发完整路径；安装前单独确认。
