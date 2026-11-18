# 测试策略与最值得移植的旧版单测

## 1. 假供应商（net/http/httptest）

参照旧版 `scripts/fixtures/fake-openai-provider.cjs`（`E:\WebApp\Mile-Project\Mile-Knowledge-Base-master\scripts\fixtures\`）在 `httptest.NewServer` 里复刻：

- `GET /models` → `{"data":[{"id":"fake-model-kb"}]}`。
- `POST /chat/completions` → `content-type: text/event-stream`，按用户消息首段触发词决定响应：
  - `FS_READ:<path>` → 第一轮发 `read_note` tool_call；
  - `FS_WRITE:<path>|<content>` → `write_note`；
  - `FS_MUTATE:<kind>|<src>|<dst>`、`FS_MKDIR:<dir>` → `mutate_notes`；
  - `HANDOFF:<owner>|<goal>` → `handoff_task`；
  - `PERMISSION_TEST` → `write_note raws/permission-test.md`（触发审批卡）；
  - `USAGE` → 正文 `USAGE_SENT` + usage 块（prompt 1000 / completion 400 / hit 800 / miss 200 / reasoning 100）；
  - `STREAM_MD` → 先 4 片 `reasoning_content`，再把一段 Markdown 切成 10 片正文（模拟流式）；
  - 第二轮（`messages` 末尾是 role=tool）→ 把工具结果翻译成固定标记（`FS_CONTENT:` / `FS_WRITE_OK|ERROR` / `FS_MUTATE_OK|ERROR` / `HANDOFF_OK|ERROR`）；
  - 默认 → 分两片慢回「好的，」「收到。」（400ms 间隔，给取消留窗口）；请求里含 `# 先前对话回顾` 时多回 `RECAP_SEEN`。
- 每个请求的 body 记录到 slice，供断言「送到模型的是什么」（消息数、tool_calls 形状、reasoning_content 是否带上）。

Go 里写 handler 时注意：SSE 每帧 `data: <json>\n\n`；tool_calls delta 按 OpenAI wire 形状（`index`、`id`、`type:"function"`、`function:{name, arguments}`）；收尾可发 `data: [DONE]` 或 `finish_reason:"stop"`。

## 2. 必覆盖场景

| 场景 | 断言 | 旧版依据 |
| --- | --- | --- |
| 429 → 200 | 重试 1 次后成功，onRetry 收到 attempt=1 | retry.ts:38-50 |
| Retry-After: 2 | 退避取 2000ms（夹 ≤60s） | retry.ts:53-61 |
| 401 | 立即 ProviderAuthError，不重试 | retry.ts:138-140 |
| 流中 300s 无字节 | StreamStalledError | openaiClient.ts:22,189 |
| 半截流（无 DONE/finish_reason） | StreamIncompleteError；不提交半截 tool 参数 | openaiClient.ts:191 |
| 流中 `{"error":{...}}` | ProviderStreamError | streamAssembler.ts:127-132 |
| `<think>` 跨块/未闭合 | 拆成 reasoning；未闭合块算思考 | thinkSplitter.ts |
| tool_calls 按 index 累积、缺 id 补 call_N | tool-call-start 顺序、完整 call 按 index 排序 | streamAssembler.ts:160-204 |
| 带 tool_calls 的 assistant 回填 | 请求体里该消息带 `reasoning_content`（空串也带） | NativeEngine.ts:351-366 |
| 取消 | 剩余调用回填「已取消，未执行」；下一轮请求不被供应商拒绝 | NativeEngine.ts:374-383 |
| 系统区写 | `.mile-kb/settings/x.json`、点号目录 → 拒绝且不碰盘（读/写分别断言） | agentPolicy.ts、VaultFsBridge.ts |
| 8.3 短名 / junction 绕过 | 第一道放行、第二道（realpath）拒绝 | VaultFsBridge.ts:161-172 |
| permanentDelete | 无新鲜审批 → `fresh-approval-required`；yolo 下必然拿不到 | EngineHost.ts:735-741,815-827 |
| 审批卡 | 发卡时模式随卡走；60s 过期；用过即焚 | VaultFsBridge.ts:57-95、EngineHost.ts:707-729 |
| 上下文压缩 | 80% 触发、尾部保留整轮、摘要失败回 truncateToFit | contextBudget.ts |
| 路由 | 「帮我整理归档」→ curator；「找出…」→ scout；多命中 scout 优先；无命中 general | agentRouting.ts |
| handoff | 自己交自己被拒；发起者按 sessionId 反查 | EngineHost.ts:882-895 |
| 用量 | estimatedCost 缺失 →「— 未提供」；hit/miss 皆 0 →「—」 | usage.ts |

## 3. 出站请求计数（RoundTripper 计数法）

包装 `http.RoundTripper` 计调用次数与记录请求（做法同 `go-service-engineering`）：断言重试的实际次数、代理是否被调用、回环是否直连。配合 `netproxy.Transport()` 时注意不要给 `http.DefaultTransport` 全局替换——为客户端显式构造 `&http.Client{Transport: counting{base: netproxy.Transport()}}`。

## 4. 最值得移植的旧版单测（tests/unit/，89 个文件）

按优先级（行为规格价值 / 迁移成本）：

1. `openaiClient.test.ts` —— SSE 解析、空闲、半截流、取消。
2. `nativeEngine.test.ts` —— 工具循环、commit boundary、取消回填、审批流。
3. `contextBudget.test.ts` —— 估算公式、校准夹值、切点、truncate 兜底。
4. `agentPolicy.test.ts` —— 权限矩阵穷举（含「过宽前缀不能绕过全局禁写」）。
5. `permissionRouter.test.ts`、`pathAliasBypass.test.ts` —— 两道判定与 8.3 短名。
6. `agentRouting.test.ts`、`followUpRouting.test.ts` —— 词表命中、优先级、追问粘性。
7. `contextTail.test.ts`、`steeringBlock.test.ts` —— 2400 预算裁剪顺序与文案。
8. `conversationStore.test.ts`、`conversationRecorder.test.ts`、`conversationM133.test.ts` —— 落盘事件、上限、残行补换行、索引重建、recap。
9. `usageFormat.test.ts` —— 「未知不等于零」与小数位。
10. `lineDiff.test.ts` —— LCS 退化、预览三形态。
11. `kbToolsError.test.ts` —— 工具错误文案分类。
12. `engineHost.test.ts` —— 只取 writeApprovals / mutationApprovals / scopeOf 断言；进程重启部分不迁。
13. `modelCatalog.test.ts`、`proxy.test.ts`、`credentialStore.test.ts` —— 模型列表解析、代理解析、key 存储行为（Go 侧对应实现各自的等价物）。
14. `confirmGate.test.ts`、`executionMode.test.ts` —— 两步确认时间门、模式归一。

不迁移：`engineRpc.test.ts`、`spawnEngine` 相关（新版无引擎进程）；`agentEngineContract.test.ts` 中与 sidecar 进程协议绑定的部分只取事件形状断言。
