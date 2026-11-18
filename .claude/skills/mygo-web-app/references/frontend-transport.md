# 前端 transport 与页面约定（Mile Knowledge Base）

## 目录

1. transport 层
2. Zustand 细粒度订阅与迟到结果
3. Channel 消费（引擎流式）
4. 事件订阅与就绪握手
5. 错误码到文案
6. 快捷键模块
7. mock 与单测
8. 桌面整页布局与标题栏

## 1. transport 层

组件、hook、store 不直接 import `src/mygo.ts`（生成物），只经过 `src/lib/transport.ts`：

```ts
export interface Transport {
  ready(): Promise<void>;
  // 窗口/标题栏
  minimize(): Promise<void>;
  toggleMaximize(): Promise<void>;
  close(): Promise<void>;
  pickVaultDir(): Promise<PickResult>;     // 返回一次性 token，前端拿不到绝对路径
  openExternal(url: string): Promise<void>;
  // 知识库 / 树 / 文档 / 检索 / 变更 / 设置
  load(rel: string, requestId: number): Promise<LoadResult>;
  save(rel: string, content: string, revision: string): Promise<SaveResult>;
  query(req: SearchReq, requestId: number): Promise<SearchResult>;
  // 引擎流式
  send(req: SendReq): { abort: () => void; done: Promise<void> };
  // 事件
  on<K extends keyof AppEvents>(name: K, cb: (p: AppEvents[K]) => void): () => void;
}
```

- 真实实现包装生成的客户端；单测与设计期注入假实现。
- 只在这一层处理 `isCallError` 与传输错误，上层拿到的都是带 `failure` 的结构。
- 方法返回的 Promise 创建后立即挂 `.then/.catch`，避免 unhandled rejection。

## 2. Zustand 细粒度订阅与迟到结果

状态管理用 Zustand（技术设计 2.2），**强制细粒度 selector**，不订阅整个对象（P-8：旧版 Agnes 抽屉订阅整个 agents 对象导致每 token 重渲染整棵抽屉）。

```ts
// 取需要的字段/切片，不整库订阅
const text = useChatStore(s => s.turns[s.activeTurn]?.text);
const running = useChatStore(s => s.running);
// 文档、树、检索、对话、设置各自一个 store 或切片
```

- 切换文档/检索：`requestId += 1` 后发起 `load`/`query`；收到结果 `if (res.requestId !== current) return`，丢弃迟到结果。
- 刷新失败且已有内容：标 `stale` 保留旧内容（与 FR-DOC「磁盘有新版本」信息卡对应），不整片清空。
- 状态分片组合（树 `partial` 与文档 `ready` 同时成立），不用一个全局 error 覆盖整页。

## 3. Channel 消费（引擎流式 token）

```ts
import { Channel, isCallError } from "mygo-runtime";
import { EngineService } from "../mygo";

export function send(req: SendReq, onFrame: (f: StreamFrame) => void) {
  const ch = new Channel<StreamFrame>();
  const call = EngineService.send(req, ch);
  const settled = call.then(() => null, (e) => e); // 立即挂上
  const done = (async () => {
    for await (const f of ch) {
      onFrame(f);
      if (f.finish) { ch.close(); break; }
    }
    const err = await settled;
    if (err) throw isCallError(err) ? new TransportError("api", err.message) : new TransportError("network", String(err));
  })();
  return { abort: () => ch.close(), done };
}
```

- Go 侧已按帧合并（见 bindings-and-channels §4），前端拿到的是**帧**不是单 token：只更新正在输出的那条消息的增量，已完成消息不重解析、不重分组（FR-AI-CHAT 新增约束）。
- `kind` 分 `delta`/`think`/`tool_call`/`usage`/`done`/`error`：思考内容进折叠区、工具调用行按 id 原地更新、用量写运行参数条、`done` 收尾。
- 停止生成 → `abort()`；新一轮先 `abort()` 旧流再发起。
- 进行中显示呼吸方块动画（`prefers-reduced-motion` 时静止），等待审批时停动画改提示（FR-AI-CHAT）。

## 4. 事件订阅与就绪握手

页面挂载、订阅完全部事件后再调 `ready()`，告诉 Go 可以开始投递（Go 侧未就绪期间的事件/意图不入队推送，避免页面没订阅就丢）。

```ts
useEffect(() => {
  const offs = [
    transport.on("file-changed", p => store.onFileChanged(p)),
    transport.on("tree-changed", p => store.onTreeChanged(p)),
    transport.on("index-status", p => store.onIndexStatus(p)),
    transport.on("approval-card", p => store.onApproval(p)),
    transport.on("theme-changed", p => applyTheme(p.appearance)),
    transport.on("settings-changed", p => store.onSettings(p)),
    transport.on("app-notice", p => store.notice(p.text)),
  ];
  transport.ready();
  return () => offs.forEach(o => o());
}, []);
```

- React 严格模式挂载两次，Go 侧 `Ready` 可重复调用。
- `file-changed`/`tree-changed` 是"再读一次"的提示，页面收到后按当前 revision 重拉，防抖合并（树 ~250ms）。
- `approval-card` 状态以 Go 侧为准（FR-AI-PERM 新增约束）：界面先发回应、收到 Go 确认后再移除卡片。

## 5. 错误码到文案

集中在 `src/lib/errors.ts`，一张表，文案对应 DESIGN.md 的行内警示卡/状态提示（无全局 Toast，PRD §8）：

| code | 文案（示例，以 docs/API.md 清单为准） |
| --- | --- |
| `not_found` | 文件或目录不存在 |
| `permission` | 没有访问权限 |
| `too_large` | 超出大小上限（实际 {size}，上限 {limit}） |
| `encoding` | 不支持的编码，请转换为 UTF-8 |
| `name_conflict` | 同级已有同名项 |
| `revision_conflict` | 磁盘版本已变化，保存被阻止 |
| `plan_rejected` | 变更计划被拒绝 |
| `secret_unavailable` | 系统加密不可用，无法保存 Key |
| 未知码 / `CallError` | 发生未知错误，请重试 |

文案不出现堆栈、完整路径、key、文件内容（PRD 10.2）。

## 6. 快捷键模块

- 一个 `useShortcuts` 在根组件注册 `keydown`（捕获阶段），表驱动 `{key, ctrl, shift, alt}`。命中后 `preventDefault()` 再执行命令。
- 本项目快捷键（对齐旧版/FR）：`Ctrl+P` 打开搜索面板、`Ctrl+S` 保存当前文档、`Ctrl+F` 文档内查找、输入框 `Enter` 发送 / `Shift+Enter` 换行、`Esc` 关覆盖层。
- 输入框聚焦时只放行必要键（Enter/Shift+Enter/Esc/↑↓ 调高度）。
- 生产构建 WebView 默认按键已被 `DevToolsDisabled` 关掉；前端仍 `preventDefault` 保证开发/生产一致。

## 7. mock 与单测

- 设计期假数据放 `src/mocks/`，**生产构建不引用**（构建后 grep `dist/` 确认）。
- 假数据不能顶替调不通的真实接口；真实接口失败时界面显示错误（红线）。
- 单测（Vitest + Testing Library + jsdom）注入假 `Transport`，覆盖：迟到结果丢弃、刷新失败进 `stale`、流式帧只更新进行中消息、审批卡按 Go 确认移除、切换文档先确认未保存。
- 本机用 `bunx vitest run`，不用 `bun run test`。

## 8. 桌面整页布局与标题栏

- 根节点 `height:100dvh; overflow:hidden`；各滚动区自己 `overflow-y:auto`；flex 链路每层 `min-height:0`；整窗不出现滚动条。
- 自绘标题栏 40px（FR-WIN）：整条 `--app-region: drag`；左侧图标+名称+「VAULT」标签、中间库名截断、右侧主题分段控件与最小化/最大化/关闭按钮标 `--app-region: no-drag`。最大化按钮按窗口态切换「最大化/还原」。
- 工作台三栏：侧栏（默认 232，192–320）、对话区、文档面板（默认 560，320–800，默认折叠）；各栏宽与开合持久化（FR-SET），只读显示在设置页。
- 尺寸、颜色全部取 `tokens.css` 变量（见 `design-tokens`），不写魔法数字与硬编码颜色。
- 375/768/1024/1440 与 DPI 100/125/150% 验算不溢出（PRD 10.3）。
