# kb-index-sqlite 参考：schema 建议与常用查询

> 状态：建议草案，阶段 08 定稿。业务规则以 docs/PRD.md FR-SRCH/FR-LINK/FR-HLTH 为准。
> 驱动核对见 SKILL.md §2（modernc.org/sqlite v1.62.0，driver `sqlite`，FTS5 内置）。

## DSN 与打开

```go
import (
    "database/sql"
    _ "modernc.org/sqlite"
)

dsn := "file:" + filepath.ToSlash(dbPath) +
    "?_pragma=busy_timeout(5000)" +
    "&_pragma=journal_mode(WAL)" +
    "&_pragma=synchronous(NORMAL)" +
    "&_pragma=foreign_keys(1)"
db, err := sql.Open("sqlite", dsn)
db.SetMaxOpenConns(1) // 单写协程
```

DSN 参数细节（modernc v1.62.0 `sqlite.go:336-520`）：

- `_pragma=<stmt>` 重复参数，逐个执行 `pragma <stmt>`；单语句限制。
- shorthand 与 `_pragma` 等价：`_busy_timeout`/`_timeout`、`_journal`/`_journal_mode`、`_foreign_keys`/`_fk`、`_synchronous`/`_sync`、`_auto_vacuum`/`_vacuum`、`_query_only`。
- 应用顺序固定：busy_timeout → auto_vacuum → `_pragma` 列表 → foreign_keys → journal_mode → synchronous → query_only。

## Schema 建议

```sql
PRAGMA journal_mode = WAL;

CREATE TABLE IF NOT EXISTS documents (
  path         TEXT PRIMARY KEY,
  title        TEXT NOT NULL,
  area         TEXT NOT NULL,
  tags         TEXT NOT NULL DEFAULT '',
  extension    TEXT NOT NULL,
  mtime_ms     INTEGER NOT NULL,
  size         INTEGER NOT NULL,
  ignored      INTEGER NOT NULL DEFAULT 0,
  body_indexed INTEGER NOT NULL DEFAULT 1,
  updated_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS documents_area  ON documents(area);
CREATE INDEX IF NOT EXISTS documents_mtime ON documents(mtime_ms);

-- 不挂 content 的 FTS5 虚表（R-2 §10.1）：五列都存预切词 token 串，正文不入库。
-- 不能用 content='documents' 外部分表：预切词列值（token 串）≠ documents 列值（原文），
-- external content 要求两者一致。
CREATE VIRTUAL TABLE IF NOT EXISTS doc_fts USING fts5(
  title, tags, headings, path, body,
  tokenize='unicode61'
);

CREATE TABLE IF NOT EXISTS links (
  from_path   TEXT NOT NULL,
  target      TEXT NOT NULL,
  kind        TEXT NOT NULL,          -- wiki | internal | external | other
  to_path     TEXT,                   -- NULL = 未解析
  candidates  INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (from_path, target)
);
CREATE INDEX IF NOT EXISTS links_to   ON links(to_path);
CREATE INDEX IF NOT EXISTS links_from ON links(from_path);

CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
-- schema_version / index_revision / last_built_at
```

FTS5 content 模式说明（R-2 §4 实测后定稿）：本项目采用**不挂 content**——写入时把切好的 token 串直接 INSERT 到 doc_fts 对应列（FTS5 当作独立列存），查询靠 `doc_fts MATCH` + `rowid` 回 documents 取 path/title/mtime。**不能**用 `content='documents'` 外部分表：预切词列值（token 串）与 documents 列值（原文）不一致，external content 要求虚表列值与主表列值相同。要点是**文件是唯一事实源，FTS 里的正文 token 是派生数据**。

## 常用查询

```sql
-- 检索（bm25 权重：title 12 / tags 8 / headings 6 / path 4 / body 1）
SELECT d.path, d.title, d.area, d.mtime_ms, d.size, d.tags, d.ignored,
       bm25(doc_fts, 12.0, 8.0, 6.0, 4.0, 1.0) AS rank
FROM doc_fts
JOIN documents d ON d.rowid = doc_fts.rowid
WHERE doc_fts MATCH ?            -- '知识 识库'（Go 侧预切词，空格 = AND；OR 用 'a OR b'）
  AND d.ignored = 0              -- includeSystem 时去掉
  AND d.area IN (?, ?)           -- areas 过滤，可省
  AND d.extension IN (?, ?)      -- 扩展名过滤，可省
  AND d.mtime_ms >= ?            -- updatedAfter，可省
ORDER BY rank                    -- relevance；updated 用 d.mtime_ms DESC；path 用 d.path
LIMIT ?;                         -- 默认 50，最多 200

-- 匹配理由（matchedFields）：offsets()/snippet()/highlight() 未编译进 modernc，不能用（R-2 §5）。
-- 也不要用 column-filter UNION / 逐行 count（22ms / 25ms 太慢）。
-- 推荐：检出 rowid 后一次拿回五列预切词串，Go 侧 contains 匹配（50 行 ~1ms）：
SELECT rowid, title, tags, headings, path, body FROM doc_fts WHERE rowid IN (?, ?, ...);
-- Go 侧对每个查询 token 按 unicode61 口径（含 - . + # 会再拆）做空格包夹 contains → matchedFields

-- 反向链接
SELECT DISTINCT from_path FROM links
WHERE to_path = ? AND from_path != ?
ORDER BY from_path;

-- 未解析链接
SELECT from_path, target, candidates FROM links
WHERE to_path IS NULL AND kind IN ('wiki','internal')
LIMIT 200;

-- 孤立文档
SELECT path FROM documents d
WHERE ignored = 0 AND body_indexed = 1
  AND path NOT IN (SELECT DISTINCT to_path FROM links WHERE to_path IS NOT NULL)
  AND path NOT IN (SELECT DISTINCT from_path FROM links WHERE to_path IS NOT NULL)
  AND lower(path) NOT LIKE '%/readme.%' AND lower(path) NOT LIKE '%/skill.md'
LIMIT 200;

-- 同名不同目录
SELECT lower(substr(path, instr(path,'/')+1)) AS base, count(*)
FROM documents WHERE ignored = 0
GROUP BY base HAVING count(*) > 1 LIMIT 200;
```

## 增量写入流程（单写协程内）

```sql
BEGIN;
-- 1) 文档行 upsert
INSERT INTO documents(path,title,area,tags,extension,mtime_ms,size,ignored,body_indexed,updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(path) DO UPDATE SET title=excluded.title, area=excluded.area, tags=excluded.tags,
  extension=excluded.extension, mtime_ms=excluded.mtime_ms, size=excluded.size,
  ignored=excluded.ignored, body_indexed=excluded.body_indexed, updated_at=excluded.updated_at;

-- 2) FTS 行：先删再插（FTS5 不支持 UPDATE 到既有 rowid 的所有写法）
DELETE FROM doc_fts WHERE rowid = ?;
INSERT INTO doc_fts(rowid, title, tags, headings, path, body)
VALUES (?,?,?,?,?,?);

-- 3) 链接边：按 from_path 整组替换
DELETE FROM links WHERE from_path = ?;
INSERT INTO links(from_path,target,kind,to_path,candidates) VALUES (?,?,?,?,?);
COMMIT;
```

受影响边重算：文档标题/删除变化时，先 `SELECT from_path FROM links WHERE target IN (?,...)` 拿到引用方集合，只对这些 from_path 重跑 resolveLink 并更新 to_path/candidates，不全库重投影。

## 删除/重建

- 文档删除：同事务里 `DELETE FROM documents WHERE path=?` + `DELETE FROM doc_fts WHERE rowid=?` + `DELETE FROM links WHERE from_path=?`，并把指向它的入边标为未解析重算。
- schema_version 升级：`meta` 里 `schema_version` 不符 → `DROP` 全部表重建或直接删库文件重建（推荐，派生数据）。
