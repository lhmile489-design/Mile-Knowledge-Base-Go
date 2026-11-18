# 协议、SSE 组装、重试与出站（旧版 file:line 对照）

> 旧版路径均相对 `E:\WebApp\Mile-Project\Mile-Knowledge-Base-master\src\`。本文件是实现对照，不是新设计。

## 1. 请求形状

`POST {baseUrl}/chat/completions`（`main/engine/openaiClient.ts:105-112`）：

```json
{
  "model": "...",
  "messages": [...],
  "stream": true,
  "stream_options": { "include_usage": true },
  "tools": [...],          // 有工具时才带
  "max_tokens": N          // 可选
}
```

请求头：`content-type: application/json`、`accept: text/event-stream`、`authorization: Bearer <key>`（`openaiClient.ts:129-133`）。`baseUrl` 去末尾斜杠后拼 `/chat/completions`（`endpoint.ts:40-42`）。

消息线型（`openaiClient.ts:25-38`）：`role ∈ system|user|assistant|tool`；assistant 无正文只有工具调用时 `content: null`；`tool` 消息带 `tool_call_id`；`reasoning_content` 是可选字段，但带 tool_calls 的 assistant 消息**必须带上**（空串也带，DeepSeek 思考模式缺它返回 400）。

## 2. SSE 组装（streamAssembler.ts）

- 只认 `data:` 前缀行；空行与其它行忽略（`streamAssembler.ts:107-114`）。
- `data: [DONE]` → done；`data` 是 JSON 对象，解析失败抛 `StreamDecodeError`（`115-125`）。
- 流里带 `{"error":{...}}` → `ProviderStreamError`（`127-132`）。
- `choices[0].finish_reason` 非空字符串就记下；`usage` 对象随时出现都发 usage 块（`137-143`）。
- `delta.reasoning_content` 优先、`reasoning` 次之（`146-152`）；`delta.content` 过 ThinkSplitter（`154-158`）。
- `tool_calls`：按 `index`（缺省 0）累积 `id`/`function.name`/`function.arguments`；name 首次出现发 `tool-call-start`；结束时按 index 排序发完整 `tool-call`；无 id 补 `call_<index>`（`160-178, 189-204`）。
- `complete()` = 见过 `[DONE]` 或 `finish_reason`；否则 `StreamIncompleteError`（`183-186`）。

## 3. ThinkSplitter（thinkSplitter.ts）

状态机 probe → inside → passthrough：

- probe：缓冲开头，trim 后不足 `<think>` 长度且是其前缀就继续等；命中 `<think>` 进 inside；否则 drain 为正文。
- inside：找 `</think>`；找到则之前内容为 reasoning、之后 trim 开头空白转 passthrough；找不到时用 markerSuffixLen 扣住可能是标签尾巴的后缀，其余当 reasoning 吐出。
- flush：inside 里剩下的是 reasoning，其余是正文；未闭合的 `<think>` 块算思考（`thinkSplitter.ts:52-57`）。
- 只在回合最开头认标签：回答中间出现的 `<think>` 不会被劫持（`1-7`）。

## 4. usage 归一（streamAssembler.ts:63-79）

| wire 字段 | 目标字段 |
| --- | --- |
| `prompt_tokens` | promptTokens |
| `completion_tokens` | completionTokens |
| `total_tokens` | totalTokens；缺省 = prompt + completion |
| `prompt_cache_hit_tokens` | cacheHitTokens；缺省用 `prompt_tokens_details.cached_tokens` |
| `prompt_cache_miss_tokens` | cacheMissTokens；缺省且 hit>0 时用 `prompt − hit` |
| `completion_tokens_details.reasoning_tokens` | reasoningTokens |

负数 / 非有限数一律按 0（`num`，`56-57`）。费用不在这里算。

## 5. 重试与退避（retry.ts）

- `MAX_RETRIES = 10`（`:15`）；`MAX_BACKOFF_MS = 15_000`；`MAX_RETRY_AFTER_MS = 60_000`；错误体读取上限 4096 字符（`16-18`）。
- `retryableStatus`：408、429、5xx（`38-40`）。401/403 → `ProviderAuthError`（`30-36, 138-140`）；其余 4xx 立即失败（`141-143`）。
- 退避：`min(2^(attempt−1) × 500, 15000) + floor(random × 250)`；有 Retry-After 时 `min(retryAfterMs, 60000)`（`42-50`）。
- `parseRetryAfter`：纯数字按秒；否则按 HTTP-date `Date.parse` 减 now；解析不了返回 0 走自己的退避（`53-61`）。
- 网络层错误（fetch reject）也进重试计数；取消立即终止不再重试（`124-133`）。
- 每次退避前回调 `onRetry{attempt, max, delayMs, error}`，引擎转成 status 事件「第 n/m 次重试，x 秒后自动继续」（`NativeEngine.ts:298-307`）。

## 6. 空闲与完成判定（openaiClient.ts）

- `DEFAULT_IDLE_TIMEOUT_MS = 300_000`（`:22`）。每读到一个 chunk 重新武装 idle 定时器；超时置 `stalled` 并 abort 底层连接（`116-122, 151-164`）。
- read 失败时：stalled → `StreamStalledError`；外部取消 → AbortError；其余原样抛（`156-162`）。
- 消费方提前 break 或出错时 `reader.cancel()` 释放连接（`184-187`）。
- 结束后：`stalled` → StalledError；`signal.aborted` → AbortError；`assembler.complete()` 为假 → `StreamIncompleteError`（`189-192`）。
- buffer 收尾：连接 done 后 buffer 里若还有残行再喂一次 assembler（`177-183`）。

## 7. 出站地址（endpoint.ts）

- `checkEndpoint`：URL 解析失败 → 「供应商地址不是合法的 URL」；带 username/password → 「不能包含用户名或密码」；https → 放行；http + 回环（localhost、::1、127.x.x.x）→ 放行；http 其他 → 「明文 http 只允许本机地址」；其余协议 → 「只允许 http(s)」（`21-37`）。
- 模型列表 `GET {base}/models`：10s 超时、响应 ≤2 MiB、`data[].id` 去 `models/` 前缀、≤128 字符、≤500 条排序返回；错误消息先把本次用的 key 原样替换再脱敏（`modelCatalog.ts:24-28, 88-144`）。

## 8. 代理（mygo netproxy 对照旧版 proxy.ts）

旧版用 Electron `session.resolveProxy` + undici `ProxyAgent`（`proxy.ts:1-19`），规则：回环永不代理、只认 HTTP(S) 代理、SOCKS 回落直连。

新版用 `github.com/egoist/mygo/netproxy`（v0.10.0 源码 `C:\Users\LHmile\go\pkg\mod\github.com\egoist\mygo@v0.10.0\netproxy\netproxy.go`）：

- `netproxy.Transport() *http.Transport`（`netproxy.go:82-88`）：`http.DefaultTransport` 的克隆，`Proxy` 换成 `netproxy.Proxy`；包级单例，要改设置先 Clone。
- `Proxy(req)`（`netproxy.go:66-76`）：回环直连 → 环境变量（HTTP_PROXY/HTTPS_PROXY/ALL_PROXY/NO_PROXY 及小写）非空走 `http.ProxyFromEnvironment` → 否则问系统。
- 回环：`localhost`、`*.localhost`、`net.ParseIP(host).IsLoopback()`（`137-144`）。
- Windows 走 WinHTTP（自动检测 + PAC）；系统答案按 scheme+host 缓存 5 秒（`24-28, 47-48, 113-135`）。
- Go transport 原生支持 HTTP/HTTPS 代理与 SOCKS5；SOCKS4 跳过（`30-33`）——旧版「SOCKS 回落直连」在新版变成 SOCKS5 可用，属改善。
- 引擎只执行出站决定，不自己判断代理（对齐旧版 `openaiClient.ts:56-59` 的纪律）。

## 9. 模型选择顺序（nativeProvider.ts）

会话所选 → Agent 保存的 → 第一个可用供应商的默认模型 → 该供应商模型列表第一个；只认 `kind=openai` 的供应商；认不出的模型名一律回落（`nativeProvider.ts:24-51`）。DeepSeek 预设：`https://api.deepseek.com`、deepseek-v4-flash / deepseek-v4-pro、`DEEPSEEK_API_KEY`、窗口 1 000 000（PRD FR-AI-PROV）。

## 10. 引擎事件词表（shared/agent.ts:165-230）

`message`（正文增量）、`thought`（思考增量）、`tool`（call:{id,title,kind?,status,detail?}）、`plan`、`permission`（request 见下）、`status`（state/phase/model/effort/detail）、`usage`（{turn,cumulative}）、`turn-end`（stopReason）、`error`（message 已脱敏）、`sidecar`（status）。新版没有独立进程，但事件形状保留——Renderer 不需要改。

PermissionRequest（`agent.ts:138-163`）：requestId、agent、sessionId、toolName、detail、options（`allow_once`/`reject_once`）、approval（发卡时模式）、targetPath、preview（Main 生成的 WritePreview，引擎的 newContent 不转发给 Renderer）。

## 11. 崩溃与恢复（旧版 vs 新版）

- 旧版：引擎是 utilityProcess；崩溃给在途回合发 error、`restarting`、退避 `min(10s, 500×2^(n−1))` 重启、最多 3 次后 `offline`（`EngineHost.ts:500-543, 172-174`）；会话按记录重建并重发审批模式与模型（`545-570`）。
- 新版（PRD FR-AI-ENG 新增约束）：引擎在 Go 进程内，没有进程可崩；单回合用 `defer recover()` 兜住转 error 事件，其余会话与 UI 不受影响。不实现重启退避，但「引擎不可用时如实显示原因、编辑器和检索不受影响」保留。
