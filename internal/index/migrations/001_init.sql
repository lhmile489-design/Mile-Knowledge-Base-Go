-- 001_init.sql — search index schema (docs/数据库设计.md §4.5).
-- Every text column that FTS5 searches holds pre-tokenised, space separated
-- tokens: FTS5's own tokenizer treats a run of CJK as one token, so the
-- bigrams have to be built in Go first (kb-index-sqlite skill §3).

CREATE TABLE IF NOT EXISTS documents (
  path         TEXT PRIMARY KEY,   -- canonical vault-relative path, forward slashes
  title        TEXT NOT NULL,
  area         TEXT NOT NULL,      -- skills/raws/.../system/other
  tags         TEXT NOT NULL DEFAULT '',
  extension    TEXT NOT NULL,
  mtime_ms     INTEGER NOT NULL,
  size         INTEGER NOT NULL,
  ignored      INTEGER NOT NULL DEFAULT 0,   -- system area
  body_indexed INTEGER NOT NULL DEFAULT 1,   -- 0 = path only (binary/oversize)
  search_excluded INTEGER NOT NULL DEFAULT 0, -- user turned the directory out of search (D-013)
  updated_at   TEXT NOT NULL,
  token_count  INTEGER NOT NULL DEFAULT 0   -- sum of the five token columns (M25 tokenCount)
);
CREATE INDEX IF NOT EXISTS documents_area  ON documents(area);
CREATE INDEX IF NOT EXISTS documents_mtime ON documents(mtime_ms);

-- No content= option: the token stream is not the document text, so external
-- content tables cannot be used.
CREATE VIRTUAL TABLE IF NOT EXISTS doc_fts USING fts5(
  title, tags, headings, path, body,
  tokenize='unicode61'
);

-- Body copy for excerpts (Q-C: its own table so documents rows stay small).
-- Written only when the body was read (text extension, ≤2 MiB, valid UTF-8).
CREATE TABLE IF NOT EXISTS doc_body (
  path      TEXT PRIMARY KEY,
  body_text TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS links (
  from_path  TEXT NOT NULL,
  target     TEXT NOT NULL,        -- original link target text
  kind       TEXT NOT NULL,        -- wiki | internal | external | other
  to_path    TEXT,                 -- NULL = unresolved
  candidates INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (from_path, target)
);
CREATE INDEX IF NOT EXISTS links_to   ON links(to_path);
CREATE INDEX IF NOT EXISTS links_from ON links(from_path);

CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
-- meta keys: schema_version, index_revision, last_built_at
