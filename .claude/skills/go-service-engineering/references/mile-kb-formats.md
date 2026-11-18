# `.mile-kb/` 与应用数据文件格式（旧版字节级对照）

> 依据：旧版 `E:\WebApp\Mile-Project\Mile-Knowledge-Base-master` v2.7.0 源码，2026-11-18 逐项核对。
> 兼容承诺：PRD 10.3 / D-003 —— `.mile-kb/` 下格式与旧版兼容，新旧版可轮流打开同一库。
> 通用规则：所有 JSON 一律 `JSON.stringify(obj, null, 2)` + 结尾 `\n`，UTF-8 无 BOM；原子写入（同目录 tmp + rename）；读取逐字段 coerce，非法值回退默认或丢弃该条。
> 标记「未核实」= 源码中未找到明确读写证据，实现前需再确认。

## 目录结构总览

```text
<vault>/.mile-kb/
├── config.json              # 声明存在；当前源码未见读写（未核实，仅在禁写前缀中列出）
├── index/                   # 声明为派生数据区；旧版实际未用（旧版索引快照在 userData）（未核实）
├── drafts/                  # 草稿暂存区（系统区，Agent 禁写）；内容由前端草稿功能写入（未核实文件格式）
├── provenance.json          # 来源追踪表（见 §1）
├── transactions/<tx-id>/
│   ├── manifest.json        # 事务清单（见 §2）
│   └── payload/<sha256>     # pre-image 文件内容，文件名 = 路径的 SHA-256（无扩展名）
└── settings/
    ├── agents.json          # Agent 配置（见 §3）
    ├── categories.json      # 分类映射（见 §4）
    ├── aliases.json         # 目录别名（见 §5）
    └── providers.json       # 供应商定义（见 §6，不含 key）
```

Agent 全局禁写前缀（`src/shared/agentPolicy.ts:25-44`）：`config.json`、`transactions/`、`index/`、`provenance.json`、`settings/`、`drafts/`，及所有点号目录。

---

## 1. provenance.json（`src/shared/provenance.ts`）

```json
{
  "version": 1,
  "records": [
    {
      "copyPath": "projects/foo/SKILL.md",
      "sourcePath": "raws/foo/SKILL.md",
      "baseRevision": 12,
      "baseHash": "<sha256 hex，复制时来源内容哈希；空串表示缺基线>",
      "copiedAt": "<ISO 8601 字符串>"
    }
  ]
}
```

- **序列化稳定**（`serializeProvenance`，provenance.ts:141-156）：records 按 `copyPath` 升序排序；每个对象的键固定顺序 `copyPath, sourcePath, baseRevision, baseHash, copiedAt`；顶层 `{version, records}`；缩进 2 空格 + 结尾换行。目的：文件进 vault 会被 Git 跟踪，不稳定序列化制造无意义 diff。
- 解析（`parseProvenance`，provenance.ts:159-181）：坏 JSON 返回空列表；条目缺 `copyPath`/`sourcePath` 字符串则跳过；`baseRevision` 非 number→0，`baseHash` 非 string→`''`，`copiedAt` 非 string→`''`。
- 只在复制到 `projects/`（前缀 `projects/`，大小写不敏感）时记录（`PROVENANCE_TRACKED_PREFIX`，provenance.ts:185-194）。
- 状态机见 `assessProvenance`（provenance.ts:77-139）：`up-to-date`/`source-changed`/`copy-modified`/`both-changed`/`source-missing`/`copy-missing`/`unknown`；只有 `source-changed` 允许同步。

## 2. transactions/<tx-id>/manifest.json（`src/main/services/TransactionService.ts`）

```json
{
  "id": "<tx id：base36时间戳-uuid前8位，如 `l9x2ab-3f8a2c1e`>",
  "vaultId": "<vaultId>",
  "planId": "<plan id>",
  "createdAt": "<ISO 8601>",
  "status": "open | committed | partial",
  "note": "<可选，partial 时的说明>",
  "entries": [
    {
      "path": "<vault 相对路径，`/` 分隔>",
      "kind": "file | directory | absent",
      "hash": "<file 才有：内容 sha256>",
      "size": "<file 才有>",
      "afterHash": "<可选，提交时磁盘内容 sha256>",
      "afterKind": "<可选：file | directory | absent>"
    }
  ]
}
```

- tx id 形态：`${Date.now().toString(36)}-${randomUUID().slice(0,8)}`（TransactionService.ts:96），目录名天然按时间排序。
- pre-image 在 `payload/<sha256(path)>`：文件名是相对路径的 SHA-256，无扩展名（TransactionService.ts:66-68）。`absent` 表示事务前不存在，恢复 = 删除。
- 目录只记 `kind:"directory"` 标记，不复制子树；`file` 才复制内容到 payload。
- `commitWithAfterImages` 提交时记录每条目的 `afterKind`/`afterHash`；恢复前逐条比对，外部又改过 → `conflicted` 不覆盖（TransactionService.ts:264-310）。
- manifest 每次 persist 都是 tmp + rename 原子写（TransactionService.ts:312-320）。
- manifest 内路径写入前过 `assertSafeRelative`（TransactionService.ts:83-95）：必须 `/` 分隔、无盘符、无 `/` 开头、无空/dot/… 段。

## 3. settings/agents.json（`src/shared/appConfig.ts` + `AppConfigStore.ts`）

```json
{
  "curator": {
    "model": "<string，空串=用引擎默认模型>",
    "effort": "auto | disabled | low | high | max",
    "approval": "ask | auto | yolo（ApprovalMode 见 shared/agent.ts，非法值回退 ask）",
    "skillPaths": ["<vault 相对路径，指向 SKILL.md，最多 500 条>"],
    "steering": "<≤4000 字符，超出截断并追加 `[已截断：steering 超过 4000 字符]`>",
    "steeringBaseVersion": 1
  },
  "scout": { "...": "同上" },
  "distiller": { "...": "同上" },
  "general": { "...": "同上" }
}
```

- 四个 key 固定为 `curator/scout/distiller/general`（`AGENT_IDS`）；读取时对每个 id 单独 `coerceAgentConfig`（AppConfigStore.ts:63-100），非法 effort 经 `normalizeEffort`、非法 approval 回退 `ask`。
- `skillPaths` 逐条过 `isSafeRelativePath`（AppConfigStore.ts:49-60）：≤1024 字符、非绝对、无盘符、无 dot 段；去重、统一 `/` 分隔。
- 写入 `JSON.stringify(agents, null, 2) + '\n'` 原子写（AppConfigStore.ts:214-227）。

## 4. settings/categories.json（`shared/appConfig.ts:normalizeCategoryConfigs`）

```json
[
  {
    "directory": "<一级目录名，单层，≤64 字符，无 /\\:*?\"<>|，非 . / ..>",
    "alias": "<显示名，≤64 字符，空串=用目录名>",
    "area": "skills|raws|archives|projects|tools|wiki|constraints|system|other 或 null（null=沿用自动分类）",
    "indexed": true,
    "order": 0
  }
]
```

- 顶层是**数组**不是对象；最多 200 条（`MAX_CATEGORIES`）；目录名大小写不敏感去重。
- `indexed` 缺省时按 `defaultIndexedFor(directory)` 补：被 `classifyTopLevel` 判为系统区的目录默认 false，其余 true（appConfig.ts:411-414）。
- 「与默认无异」的条目不落盘（`isMeaningfulCategory`，appConfig.ts:422-430）。
- 写入同 agents.json 的序列化格式。

## 5. settings/aliases.json（`shared/appConfig.ts:normalizeAliasMap`）

```json
{
  "projects/web": "我的前端项目",
  "tools": "工具箱"
}
```

- 顶层是**对象**（`Record<path, alias>`）：key = vault 相对目录路径（`/` 分隔归一），value = ≤64 字符显示名；最多 2000 条（FR-AREA）。
- 非法 key（绝对、盘符、dot 段、>1024）丢弃；value 非非空字符串丢弃。
- 空串/空白 value 写入时删除该条（AppConfigStore.patchAliases，AppConfigStore.ts:186-205）。

## 6. settings/providers.json（`src/main/services/ProviderStore.ts`）

```json
{
  "version": 1,
  "providers": [
    {
      "name": "<≤64 字符，`^[A-Za-z0-9][A-Za-z0-9_.-]*$`>",
      "kind": "openai | anthropic",
      "baseUrl": "<http(s)，host 非空，≤512 字符>",
      "models": ["<每个 ≤128 字符，最多 32 个>"],
      "defaultModel": "<必须在 models 中；缺省回退 models[0]>",
      "apiKeyEnv": "<环境变量名 `^[A-Za-z_][A-Za-z0-9_]*$` ≤96 字符；空串=无需鉴权>",
      "contextWindow": 1000000
    }
  ]
}
```

- 最多 50 个 provider（`MAX_PROVIDERS`）；同名（不忽略大小写）只留第一个。
- **不含 key 本身**：key 用 Electron safeStorage 加密存 userData `engine-credentials.json`（新版读不出，用户需重填，PRD Q2）。
- 每条过 `normalizeProviderDraft`（appConfig.ts:200-271），不合法整条丢弃。
- **旧版迁移**：providers.json 不存在时读 `<vault>/reasonix.toml` 托管区（`# >>> mile-knowledge-base managed providers` … `# <<<`）解析 `[[providers]]` 块；非空才写 providers.json，reasonix.toml 原样保留（ProviderStore.ts:44-60、99-145）。新版是否沿用此迁移：未核实（PRD 未提；建议沿用，只读不删）。

## 7. config.json、index/、drafts/

- `.mile-kb/config.json`：仅在 `GLOBAL_WRITE_DENY_PREFIXES` 注释中出现（agentPolicy.ts:21-26，称「配置」），当前源码未见读写实现 → **未核实**。
- `.mile-kb/index/`：注释称「派生数据」（agentPolicy.ts:23-24），旧版索引快照实际写在 userData `index-<vaultId>.json`，未用此目录 → **未核实**；新版 SQLite 索引也不放这里（PRD v1.1：放应用数据目录）。
- `.mile-kb/drafts/`：草稿暂存区，Agent 禁写（agentPolicy.ts:43-44）；`shared/settingsDrafts.ts` 有设置草稿逻辑，草稿文件本体格式 **未核实**（实现阶段 08 前需再确认）。

---

## 应用数据（userData 侧，新版一次性导入用）

旧版 userData 目录推断为 **`%APPDATA%\mile-knowledge-base\`**。依据：`electron-builder.yml:3-4` `appId: io.mile.knowledgebase` + `productName: mile-knowledge-base`；Electron `app.getPath('userData')` 在 Windows 返回 `%APPDATA%\<productName>`（dev 下用 package.json `name`，同为 `mile-knowledge-base`）。新版 identifier 未定时此目录是**只读导入源**，不是新版写入位置。

### settings.json（`ConfigStore.ts` + `shared/settings.ts`）

```json
{
  "theme": "light | dark | system（默认 system；非法→system）",
  "closeBehavior": "ask | tray | quit（默认 ask；非法→ask）",
  "layout": {
    "sidebarWidth": 232,            // 范围 192–320
    "docWidth": 560,                // 范围 320–800
    "docCollapsed": true,           // 默认折叠
    "sidebarCollapsed": false,
    "agnesControlsCollapsed": false, // 运行参数条，默认展开
    "agnesHistoryOpen": true,        // 历史栏，默认打开
    "agnesComposerHeight": 96        // 输入框高，范围 72–400
  },
  "recentVaults": [
    { "id": "<sha256前16位>", "root": "<canonical 绝对路径>", "displayName": "...", "lastOpenedAt": "<ISO>", "createdAt": "<ISO 可选>" }
  ],
  "lastVaultId": "<id 或 null；必须在 recentVaults 中>"
}
```

- 旧字段 `agnesWidth/agnesCollapsed`（旧版右侧抽屉尺寸）**故意不迁移**（ConfigStore.ts:37-45 注释）；`engine/sidecarPath` 残留键读取时忽略。
- recentVaults 最多 12 条（`MAX_RECENT_VAULTS`）；clampWidth 非有限数回落 min。
- vaultId = `sha256(canonicalRoot.toLowerCase())` 前 16 位 hex（VaultService.ts:703-704）。

### recent-documents.json（`RecentDocumentsStore.ts` + `shared/recentDocuments.ts`）

```json
{
  "<vaultId>": [
    { "path": "<vault 相对路径>", "openedAt": "<ISO 8601>" }
  ]
}
```

- 顶层 `Record<vaultId, list>`；每库最多 12 条（`MAX_RECENT_DOCUMENTS`），同路径去重置顶。
- vaultId 键须匹配 `^[A-Za-z0-9_-]{1,128}$`；path 逐条过 `isPlausibleRelativePath`（非绝对/UNC/盘符、无 dot 段、≤1024）。

### audit.json（`AuditLedger.ts`）

```json
{ "version": 1, "records": [ { "id","planId","vaultId","origin","kind","status","createdAt","source"?,"target"?,"beforeHash"?,"afterHash"?,"transactionId"?,"errorCode"?,"errorMessage"?,"completedAt"? } ] }
```

- 最多 2000 条，超出裁最旧；**存 userData 不进 vault**。新版格式自定，导入不需要（操作日志属机器级记录，PRD 未要求导入）。

### conversations/（`ConversationStore.ts`）

布局 `conversations/<vaultId>/index.json` + `<conversationId>.jsonl`（每行一个事件）。**PRD 明确不导入**（D-003）；新版格式阶段 08 定。

### engine-credentials.json

safeStorage 加密的 API key。新版**读不出来**，跳过导入，用户重填（PRD Q2）。
