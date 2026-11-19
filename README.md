# Mile Knowledge Base（Go 重构版）

> 状态：项目完成（M0–M3，T00–T16 全部交付），2026-11-19 经用户人工测试通过。应用现在能做：打开知识库、文件树与树的增删改/拖放、阅读/源码/分栏、编辑保存与冲突、外部改动同步、SQLite 全文检索与搜索面板、三层链接解析与反向链接、健康检查、变更计划与事务、操作日志与恢复、来源追踪与一键同步、图片粘贴上传；以及内置 AI 助手：模型服务与 key 管理、4 个 Agent 与 steering、自动路由与改派、流式回答与工具审批、对话历史与搜索、用量收据、设置页「模型服务」「Agents」两节；M3 补齐设置五节、技能导入向导、总览 9 块磁贴、全量视觉与性能验收，并打出 1.0.0 安装包（本机冒烟通过）。M3 已推送（`765f908`），图标修复随后推送（`f6860aa`）。
> 更新日期：2026-11-19

## 1. 这是什么

本地优先的 Markdown 知识库桌面应用，内置权限隔离的 AI 助手 Agnes。

这是旧版 [Mile Knowledge Base](../Mile-Knowledge-Base-master)（Electron 39 + React 19，v2.7.0）的重写。旧版功能完整可用，但经常卡死、性能低。新版改用 egoist/mygo（Go + 系统 WebView2）承载后端，前端继续使用 React，功能、UI/UX 与布局与旧版保持一致。

两条从旧版继承的前提：

- 普通 `.md` 文件始终是唯一事实源。不导入专有格式，不改变用户用文件管理器、Git 或 Obsidian 访问这些文件的方式。
- AI 的权限由宿主强制，不靠提示词约束。

产品定位与边界见 [PRODUCT.md](PRODUCT.md)，本次重写的需求范围见 [docs/PRD.md](docs/PRD.md)。

## 2. 文档导航

| 文档 | 用途 | 当前状态 |
| --- | --- | --- |
| [CLAUDE.md](CLAUDE.md) | 唯一约束入口：执行规则、当前阶段、文档索引、Skills | 已填项目事实 |
| [WORKFLOW.md](WORKFLOW.md) | 13 阶段流程、审核、完成标准 | 通用规范 |
| [PRODUCT.md](PRODUCT.md) | 产品定位、用户、概念、长期边界 | 已确认 |
| [docs/PRD.md](docs/PRD.md) | 重写版需求范围、规则与验收 | R1 已确认 v1.9；第 15 节 R2 技能管理已确认 v2.0，v2.1 按工具目录核实修订 |
| [DESIGN.md](DESIGN.md) | 全局 UI 契约 | 已确认 v1.0 |
| [docs/技术设计.md](docs/技术设计.md) | 选型、架构、质量门禁；第 13 节 R2 工具目录核实 | 已确认 v1.0；第 13 节 2026-11-20 审核通过 |
| [docs/API.md](docs/API.md) | Go 绑定与事件契约 | 已确认 v1.0 |
| [docs/数据库设计.md](docs/数据库设计.md) | 本地数据与索引存储 | 已确认 v1.0 |
| [docs/PLAN-M0.md](docs/PLAN-M0.md) | M0 实施计划 + M1–M3 粗粒度路线图 + 会话恢复 | 已完成并推送（39041a0） |
| [docs/PLAN-M1.md](docs/PLAN-M1.md) | M1 实施计划、交接、验收与账本 | 已完成并推送（59a1076） |
| [docs/PLAN-M2.md](docs/PLAN-M2.md) | M2 实施计划、交接、验收、性能与账本 | 已完成并推送（f6d663a） |
| [docs/PLAN-M3.md](docs/PLAN-M3.md) | M3 实施计划、验收、性能与账本 | 已完成并推送（765f908） |
| [docs/UI设计稿.md](docs/UI设计稿.md) 与 `design/baseline/` | 页面与状态清单、旧版截图基线 | 已确认 v1.0 |
| [docs/decisions.md](docs/decisions.md) | 重要取舍 | 按需记录 |
| [docs/发布清单.md](docs/发布清单.md) | 发布与恢复验证 | 模板 |

## 3. 参考资料（只读）

| 位置 | 用途 |
| --- | --- |
| `E:\WebApp\Mile-Project\Mile-Knowledge-Base-master` | 旧版源码、DESIGN.md、单测与真窗口验证脚本，是功能与 UI 一致性的对照基线 |
| `E:\WebApp\Mile-Project\Mile-Typora` | 已跑通的 mygo v0.9.0 + React 19 项目，用作工程配置参考 |
| `E:\WebApp\Mile-Project\skillkit-main` | 开源技能管理工具 skillkit（MIT，desktop 0.8.1），R2 技能管理的参考实现 |
| `C:\Users\LHmile\go\pkg\mod\github.com\egoist\mygo@v0.10.0\docs\` | mygo 锁定版本的官方文档 |

## 4. 开发环境（2026-11-18 本机实测）

| 依赖 | 实测版本 | 说明 |
| --- | --- | --- |
| Go | go1.27.3 windows/amd64 | bash 中找不到时：`export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"` |
| Bun | 1.3.13 | 前端包管理与脚本 |
| mygo CLI（全局） | 0.8.3 | 比锁定的 v0.10.0 旧，不使用；建工程时用 `go run github.com/egoist/mygo/cmd/mygo@v0.10.0 init`，工程内用 npm `mygo-cli` 0.10.0 |
| WebView2 | Windows 11 自带 | 未单独核对版本 |

锁定版本以阶段 03 的技术设计为准，上表只是环境现状。

## 5. 运行与验证入口

| 命令标识 | 操作 | 实际命令或脚本入口 | 前置条件与验证状态 |
| --- | --- | --- | --- |
| 安装依赖 | 安装前端依赖 | `bun install` | 已验证 |
| 本地启动 | 开发模式 | `bun run dev`（端口 5274，自定义协议 kbimg；AI 功能需先在设置里配置供应商与 key） | 已验证 |
| 类型检查 | `tsc --noEmit` | `bun run typecheck` | 已验证 |
| 前端门禁 | 绑定对拍 + Vitest | `bun run test:web`（先 `check:bindings`，再 vitest） | 已验证（2026-11-19：绑定对拍通过、152 项） |
| Go 测试 | 单元测试与静态检查 | `bun run test:go` / `bun run lint:go`；格式另跑 `gofmt -l .` | 已验证（2026-11-19：484 项，17 包；vet 与 gofmt 无输出） |
| 竞态检测 | `go test -race` | `bun run test:go-race` | 未验证（需要 cgo，本机未跑；不在常规门禁内） |
| 模糊测试 | 路径与渲染（`FuzzValidateRel`、`FuzzRender`） | `bun run test:fuzz`（`scripts/test-fuzz.sh`，每个目标默认 60s，`FUZZ_TIME=3s` 可缩短） | 已验证（2026-11-18：正向通过；故意失败的对照返回非零） |
| 端到端 | e2e 构建 + automation | `bun run test:e2e`（`scripts/test-e2e.sh`：先 `GOFLAGS=-tags=e2e MYGO_INSPECTOR=1 bun run build`，再依次跑 `scripts/e2e/ac-*.sh`，即 ac-m0、ac-m1、ac-m2；`E2E_SKIP_BUILD=1` 跳过构建） | 三个脚本分别验证过（ac-m0 15/15、ac-m1 16/16、ac-m2 22/22）；M2 后未整体重跑 |
| M2 验收 | 假供应商闭环（不写凭据管理器） | `bash scripts/e2e/ac-m2.sh`（需要先做 e2e 构建；自带假供应商） | 已验证 22/22（2026-11-19） |
| 新旧版兼容 | 事务与 provenance 字节互通 | `bash scripts/compat/legacy-compat.sh`（需要旧版仓库与 bun，只读复制到临时目录） | 已验证 7/7 |
| M0 性能 | 冷启动 / 大库 / 滚动 / 外部改动 | `bash scripts/e2e/perf.sh` | 已验证（见 docs/PLAN-M0.md §6） |
| M1 性能 | 冷启动 / 索引 / 检索 p95 / 批量改动 | `bash scripts/e2e/perf-m1.sh` | 已验证（见 docs/PLAN-M1.md §6.0） |
| M2 性能 | 流式帧率 / 输入延迟 / 历史首帧 / 消息虚拟化 | `bash scripts/e2e/perf-ai.sh`（假供应商；需要先做 e2e 构建） | 已验证（见 docs/PLAN-M2.md §6） |
| M3 验收 | 总览、设置五节、主题与分类持久化、进检索、重启 | `bash scripts/e2e/ac-m3.sh`（需要 e2e 构建） | 已验证 47/47 |
| M3 视觉对照 | 49 张新版截图 + 本地并排对照页 | `python -I scripts/visual/capture-new.py`（生成物在 `design/compare/`，已 gitignore） | 已验证（见 docs/PLAN-M3.md §6） |
| 性能门禁 | 指标 JSON 校验（G-PERF，阻断） | `bun run perf:gate --kind typical --input <json>` | 已接入（阻断，正反对照通过） |
| 旧版性能基线 | 只读运行旧版 Electron 测冷启动与检索 | `bash scripts/perf-legacy/bench.sh typical\|big` | 已验证（见 docs/PLAN-M3.md §6） |
| 安装包 | 1.0.0 NSIS 安装包 | `bun run build`（产物 `build/windows-amd64/mile-knowledge-base-setup-1.0.0-amd64.exe`） | 本机安装/启动/卸载冒烟通过；升级未测 |
| M2 冒烟 | Secrets 与系统代理能力 | `go run ./spikes/t00-smoke` | 已验证（Secrets 存取/超长拒绝/删除；netproxy 回环直连） |
| 构建 | 生产构建 | `bun run build`（产物 `build/windows-amd64/`） | 已验证 |

门禁阈值与例外以 docs/技术设计.md 第 10 节为准；上表是实际命令入口，未经验证的不写成可用。
