---
name: mygo-app-scaffold
description: 在 Windows 11 上为 Mile Knowledge Base（Go 重构版）创建并跑通 egoist/mygo v0.10.0 的 web 模板项目，并叠加 React 19 与技术设计锁定的全部依赖。用于阶段 10 建工程：mygo init、环境检查（Go、Bun、WebView2）、mygo.config.ts / vite.config.ts / package.json 契约字段、加 React、生成 src/mygo.ts、resources/icon.png、mygo dev|build|generate|doctor 报错，以及 "no required module provides package github.com/egoist/mygo" 一类依赖问题。只在用户授权进入阶段 10、开始建业务工程后才使用。
---

# mygo-app-scaffold（Mile Knowledge Base 版）

针对 egoist/mygo **v0.10.0** 源码核对。本项目由 Electron 旧版（Mile Knowledge Base v2.7.0）重写而来，技术选型已全部锁定在 `docs/技术设计.md` 第 2 节，本 skill 只负责按它落地脚手架，不改选型。

## 0. 铁律

1. **阶段 10 之前不要执行本 skill。** 当前在阶段 04，还没建工程、不写业务代码。用户明确授权进入阶段 10 后才动手。
2. **不凭记忆手写模板。** 权威模板在 `C:\Users\LHmile\go\pkg\mod\github.com\egoist\mygo@v0.10.0\cmd\mygo\template\web\`，由 `cmd/mygo/init.go` 的 `templateFS` embed。`*.tmpl` 是 Go template，变量见 `templateData`。能用 `mygo init` 就用，不能用（目录非空）才逐字照抄模板。
3. **模板生成的字段是运行期契约。** `mygo.config.ts` 的 `devUrl/devCommand/buildCommand/frontendDist/bindings/out`、`package.json` 的 `mygo-runtime` / `mygo-cli`、`vite.config.ts` 的 `strictPort`，叠加 React 与依赖时只增不改。
4. **`identifier` 定稿后不再改**，它决定应用数据目录（`mygo.App.Path(mygo.PathUserData)`）和单实例锁。当前**待阶段 10 确定**，建议值 `dev.mile.kb`，标注为建议不是结论。
5. **开发端口待阶段 10 确定**，建议值 `5274`（避开 Mile Typora 已用的 5273 和模板默认 5173），标注为建议。

## 1. 环境前置检查

| 依赖 | 验证 | 本项目要求 |
| --- | --- | --- |
| Go | `go version` | 技术设计 2.1：go1.27.3 windows/amd64，本机已装 |
| Bun | `bun --version` | 技术设计 2.1：1.3.13，mygo 模板 `devCommand` 用 bun |
| WebView2 | Windows 11 自带 | 运行时依赖 |
| mygo CLI | 初始化用 `go run github.com/egoist/mygo/cmd/mygo@v0.10.0 init`；项目建好后用 `bun run dev/build/generate` 调 npm `mygo-cli`；体检 `bunx mygo doctor` | 本机全局 CLI 是 0.8.3，**不使用**，避免版本漂移 |

**`mygo init` 默认取最新版**（实测 v0.9.x 时代：go.mod 写最新、npm 写 `^x.y.z` 并装最新）。初始化后必须钉回锁定版 v0.10.0：

- `go get github.com/egoist/mygo@v0.10.0`（改 go.mod require）
- `package.json` 的 `mygo-runtime`、`mygo-cli` 写**精确版本** `0.10.0`（去掉 `^`）
- 再 `bun install` 生成 `bun.lock` 并提交

技术设计 2.4：三处（Go 模块、`mygo-runtime`、`mygo-cli`）版本必须一致且精确；以后升级显式评估，不随 init 自动取最新。

**E2E/automation 构建要点（R-4 §6 实测）**：要跑 `plugins/automation` 端到端时需 `go build -tags e2e -o <固定名>.exe` 出固定名字的 exe 再跑——不要 `go run -tags e2e`，它每次编译出新临时 exe，Windows 防火墙逐 exe 弹「是否允许网络访问」，遮挡应用窗口并干扰自动化截图。`MYGO_AUTOMATION` 在 Windows 上是 AF_UNIX 文件 socket（`net.Listen("unix")`），Git Bash 的 socat/curl 连不上，驱动端用 Go `net.Dial("unix")` 或 `tcp:` 扩展。详见 `mygo-web-app` 的 `references/build-and-verify.md`。

## 2. 创建项目（目录非空的处理）

仓库根目录已有 `docs/`、`.claude/`、`CLAUDE.md`、`README.md` 等文件，`mygo init` **拒绝非空目录**（`cmd/mygo/init.go` `newProject`：`%s is not empty`）。做法：

1. 在临时目录生成：
   ```bash
   go run github.com/egoist/mygo/cmd/mygo@v0.10.0 init -name "Mile Knowledge Base" -module <module路径> <临时目录>
   ```
   `-name` 与 `-module` 不填时取目录名。`identifier` 没有对应参数，默认 `com.example.<slug>`，生成后在 `mygo.config.ts` 改成定稿值。
2. 逐个复制到仓库根，**不覆盖已有文件**；`.gitignore` 合并，不替换。
3. v0.10.0 web 模板实际文件：`gitignore.tmpl`、`index.html.tmpl`、`main.go.tmpl`、`mygo.config.ts.tmpl`、`package.json.tmpl`、`tsconfig.json`、`vite.config.ts`、`src/main.ts`、`src/style.css`。
4. 之后顺序：`go mod tidy` → `bun install` → `bun run generate` → `bun run dev`。

## 3. 叠加 React 与技术设计锁定的依赖

版本全部以 `docs/技术设计.md` 2.2 / 2.3 为准，不要照抄下面的注释数字之外再发明。

```bash
# 前端运行依赖（版本见技术设计 2.2，bun add 时写确切版本）
bun add react react-dom zustand @phosphor-icons/react @tanstack/react-virtual \
  @codemirror/view @codemirror/state @codemirror/commands @codemirror/search @codemirror/language @codemirror/lang-markdown
# 前端开发依赖
bun add -d @vitejs/plugin-react @types/react @types/react-dom \
  tailwindcss @tailwindcss/vite \
  vitest jsdom @testing-library/react
# Go 依赖（版本见技术设计 2.3）
go get github.com/yuin/goldmark \
  go.abhg.dev/goldmark/wikilink \
  go.abhg.dev/goldmark/frontmatter \
  github.com/microcosm-cc/bluemonday \
  modernc.org/sqlite
```

- `vite.config.ts`：在模板配置上加 `plugins: [react(), tailwindcss()]`，保留 `server.port`、`strictPort: true`、`watch.ignored`。端口用阶段 10 定稿值，`mygo.config.ts` 的 `devUrl` 同改；建议用 `host: "127.0.0.1"` 而非 `localhost`（Node 可能只解析 ::1，WebView 走 IPv4）。
- `tsconfig.json`：加 `"jsx": "react-jsx"`，其余保留模板值（TypeScript ^7）。
- 模板 `src/main.ts` 换成 `src/main.tsx`，`index.html` 入口同改；`src/style.css` 换成本项目设计令牌（Tailwind + 旧版 `tokens.css` 语义令牌，阶段 06 拆整）。
- **不引入** react-markdown / remark-gfm / DOMPurify（技术设计 2.2「不再使用」）：Markdown 在 Go 侧用 goldmark 渲染、bluemonday 清理，见 `markdown-render-pipeline` skill。
- 图标用 `@phosphor-icons/react` **深导入**，不是 lucide（旧版是 phosphor，技术设计 2.2 已确认）。

## 4. 建议目录结构（与技术设计第 3 节模块对应，标为建议）

```text
Mile-Knowledge-Base-Go/
├── main.go                 启动顺序：单实例 -> 协议 -> 外观 -> Bind -> WhenReady -> Run
├── internal/               见 go-service-engineering；分包建议（技术设计 §3）：
│   ├── vault/              知识库登记、打开、切换、断开
│   ├── pathguard/          路径与真实路径校验，唯一入口
│   ├── index/              扫描、分词、SQLite FTS5、链接图、健康
│   ├── mutation/           变更计划、事务、操作日志、恢复
│   ├── render/             goldmark -> HTML、wikilink、bluemonday
│   ├── engine/             OpenAI 兼容流式、工具循环、上下文预算
│   ├── agent/              4 个 Agent、路由、权限门、steering、共享上下文
│   ├── conversation/       对话历史（应用数据目录）
│   ├── settings/           应用设置、旧版设置一次性导入
│   ├── service/            mygo 绑定服务层，唯一 import mygo 的业务层
│   └── contract/           前端可见的契约类型与错误码
├── src/                    前端（以 mygo init 实际结构为准）
│   ├── mygo.ts             mygo generate 生成物，不手改
│   ├── main.tsx
│   ├── app/                根组件、Zustand store、快捷键、transport
│   ├── components/         标题栏、侧栏、文件树、编辑器、对话、设置等
│   ├── lib/transport.ts    唯一调用 src/mygo.ts 的模块
│   └── styles/             tokens.css、markdown.css
├── resources/icon.png      1024 x 1024 带透明通道
├── testdata/               渲染、检索、路径校验样本
├── mygo.config.ts
└── vite.config.ts
```

## 5. 绑定契约（要点）

- `main.go` 里 `mygo.Bind(...)` 的导出方法经 `mygo generate` 写成 `src/mygo.ts`，返回 `Promise<T>`；`*mygo.Channel[T]` 参数变成 `Channel<T>`；`mygo.NewEvent[T]("name")` 变成 `events.name.on/once`。
- `generate` 报 `no required module provides package github.com/egoist/mygo`：先 `go mod tidy`。
- 页面加载 `URL: "/"`：开发期是 `devUrl`，构建后 Windows 上是 `http://mygo.localhost`。
- 绑定设计、Channel 流式、事件定向：见 `mygo-web-app` skill，不在本 skill 展开。

## 6. Windows 执行要点

- bash 里找不到 `go`：`export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`。
- `mygo dev` 依赖 `bun` 在 PATH（`devCommand` 是 `bun run dev:web`）。
- `mygo build` 产物在 `build/windows-amd64/`；NSIS 首次需联网下载；`-skip-installer` 只出 exe 且清空输出目录。
- 复制 PNG 等二进制用 `cp` / `Copy-Item`，不用文本写入工具。
- 本机前端测试用 `bunx vitest run`，不用 `bun run test`（会命中系统 `test` 命令）。

## 7. 跑通判定

- `bunx mygo doctor` 全绿。
- `bun run dev` 弹出窗口，React 根组件渲染出骨架，控制台无报错。
- 一个最小绑定能从页面调用并返回。
- `go test ./...`、`bun run typecheck` 通过。
- 汇报时附命令输出或窗口截图；"弹窗正常"不能只凭构建成功判断。

## 交付前自检

- [ ] 用户已授权进入阶段 10，本 skill 才被执行
- [ ] go.mod 的 `github.com/egoist/mygo` 精确到 v0.10.0
- [ ] `package.json` 的 `mygo-runtime`、`mygo-cli` 是精确 `0.10.0`（无 `^`）
- [ ] `mygo.config.ts` 的 `identifier`、端口是阶段 10 定稿值，不是建议值占位
- [ ] `devUrl` 与 `vite.config.ts` 端口一致，`strictPort: true`
- [ ] 前端依赖只含技术设计 2.2 的版本，无 lucide / react-markdown / DOMPurify
- [ ] Go 依赖只含技术设计 2.3 的版本
- [ ] 目录结构与模块划分与技术设计第 3 节一致，标为建议处已注明
