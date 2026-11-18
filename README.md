# Mile Knowledge Base（Go 重构版）

> 状态：文档阶段 01–08 已完成并审核通过；阶段 09 的实施计划 docs/PLAN-M0.md 待审批；尚未建立工程，没有可运行的代码。
> 更新日期：2026-11-18

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
| [docs/PRD.md](docs/PRD.md) | 重写版需求范围、规则与验收 | 已确认 v1.5 |
| [DESIGN.md](DESIGN.md) | 全局 UI 契约 | 已确认 v1.0 |
| [docs/技术设计.md](docs/技术设计.md) | 选型、架构、质量门禁 | 已确认 v1.0 |
| [docs/API.md](docs/API.md) | Go 绑定与事件契约 | 已确认 v1.0 |
| [docs/数据库设计.md](docs/数据库设计.md) | 本地数据与索引存储 | 已确认 v1.0 |
| [docs/PLAN-M0.md](docs/PLAN-M0.md) | 实施计划（M0 详细 + M1–M3 粗粒度路线图）与会话恢复 | 草案 v0.3，待审批 |
| [docs/UI设计稿.md](docs/UI设计稿.md) 与 `design/baseline/` | 页面与状态清单、旧版截图基线 | 已确认 v1.0 |
| [docs/decisions.md](docs/decisions.md) | 重要取舍 | 按需记录 |
| [docs/发布清单.md](docs/发布清单.md) | 发布与恢复验证 | 模板 |

## 3. 参考资料（只读）

| 位置 | 用途 |
| --- | --- |
| `E:\WebApp\Mile-Project\Mile-Knowledge-Base-master` | 旧版源码、DESIGN.md、单测与真窗口验证脚本，是功能与 UI 一致性的对照基线 |
| `E:\WebApp\Mile-Project\Mile-Typora` | 已跑通的 mygo v0.9.0 + React 19 项目，用作工程配置参考 |
| `C:\Users\LHmile\go\pkg\mod\github.com\egoist\mygo@v0.9.0\docs\` | mygo 锁定版本的官方文档 |

## 4. 开发环境（2026-11-18 本机实测）

| 依赖 | 实测版本 | 说明 |
| --- | --- | --- |
| Go | go1.27.3 windows/amd64 | bash 中找不到时：`export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"` |
| Bun | 1.3.13 | 前端包管理与脚本 |
| mygo CLI（全局） | 0.8.3 | 比锁定的 v0.10.0 旧，不使用；建工程时用 `go run github.com/egoist/mygo/cmd/mygo@v0.10.0 init`，工程内用 npm `mygo-cli` 0.10.0 |
| WebView2 | Windows 11 自带 | 未单独核对版本 |

锁定版本以阶段 03 的技术设计为准，上表只是环境现状。

## 5. 运行与验证入口

工程尚未建立，以下命令都还没有。

| 命令标识 | 操作 | 实际命令或脚本入口 | 前置条件与验证状态 |
| --- | --- | --- | --- |
| 待分配 | 安装依赖 | 阶段 10 建立工程后填写 | 未配置、未验证 |
| 待分配 | 本地启动 | 同上 | 未配置、未验证 |
| 待分配 | 检查与测试 | 同上 | 未配置、未验证 |
| 待分配 | 构建与打包 | 同上 | 未配置、未验证 |

工程建立后给技术设计中的每个质量门禁分配稳定的命令标识，并在此表或项目脚本中写入真实命令。不要把旧版或 Mile-Typora 的命令未经验证就填成可用。
