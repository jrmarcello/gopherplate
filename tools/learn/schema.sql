-- schema.sql: learning-loop SQLite store (REQ-18, REQ-19, REQ-19a).
-- All statements use IF NOT EXISTS so re-running on an existing DB is a no-op.

-- ===== Base tables =====

CREATE TABLE IF NOT EXISTS events (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    spec_path     TEXT NOT NULL,
    session_id    TEXT NOT NULL,
    completed_at  TEXT NOT NULL,
    commit_sha    TEXT,
    tasks_count   INTEGER,
    outcome       TEXT NOT NULL DEFAULT 'success'
);

CREATE TABLE IF NOT EXISTS patterns (
    id             INTEGER PRIMARY KEY,
    kind           TEXT NOT NULL,
    signature      TEXT NOT NULL UNIQUE,
    frequency      INTEGER NOT NULL,
    first_seen_at  TEXT NOT NULL,
    last_seen_at   TEXT NOT NULL,
    score          REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS candidates (
    id             INTEGER PRIMARY KEY,
    pattern_id     INTEGER NOT NULL REFERENCES patterns(id),
    contexts_json  TEXT NOT NULL,
    score          REAL NOT NULL,
    created_at     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS decisions (
    id                   INTEGER PRIMARY KEY,
    candidate_signature  TEXT NOT NULL,
    action               TEXT NOT NULL CHECK(action IN ('new-skill','new-memory','update','discard','pending-approval','applied','rolled-back')),
    target_path          TEXT,
    rationale            TEXT NOT NULL DEFAULT '',
    diff                 TEXT,
    decided_at           TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS skill_index (
    path              TEXT PRIMARY KEY,
    title             TEXT NOT NULL,
    body              TEXT NOT NULL,
    tags              TEXT,
    frontmatter_json  TEXT NOT NULL,
    indexed_at        TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS memory_index (
    path              TEXT PRIMARY KEY,
    title             TEXT NOT NULL,
    body              TEXT NOT NULL,
    frontmatter_json  TEXT NOT NULL,
    indexed_at        TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS skill_usage (
    path          TEXT PRIMARY KEY REFERENCES skill_index(path),
    usage_count   INTEGER NOT NULL DEFAULT 0,
    last_used_at  TEXT
);

CREATE TABLE IF NOT EXISTS memory_usage (
    path          TEXT PRIMARY KEY REFERENCES memory_index(path),
    usage_count   INTEGER NOT NULL DEFAULT 0,
    last_used_at  TEXT
);

CREATE TABLE IF NOT EXISTS nudge_state (
    id             INTEGER PRIMARY KEY CHECK(id=1),
    counter        INTEGER NOT NULL DEFAULT 0,
    last_nudge_at  TEXT
);

CREATE TABLE IF NOT EXISTS config_kv (
    key    TEXT PRIMARY KEY,
    value  TEXT NOT NULL
);

INSERT OR IGNORE INTO nudge_state (id, counter) VALUES (1, 0);

-- ===== FTS5 virtual tables (external content) =====

CREATE VIRTUAL TABLE IF NOT EXISTS skill_fts USING fts5(
    path UNINDEXED,
    title,
    body,
    tags,
    content='skill_index',
    content_rowid='rowid'
);

CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
    path UNINDEXED,
    title,
    body,
    content='memory_index',
    content_rowid='rowid'
);

CREATE VIRTUAL TABLE IF NOT EXISTS pattern_fts USING fts5(
    kind UNINDEXED,
    signature,
    content='patterns',
    content_rowid='id'
);

-- ===== FTS5 sync triggers (external content table pattern) =====

CREATE TRIGGER IF NOT EXISTS skill_index_ai AFTER INSERT ON skill_index BEGIN
    INSERT INTO skill_fts(rowid, path, title, body, tags)
    VALUES (new.rowid, new.path, new.title, new.body, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS skill_index_ad AFTER DELETE ON skill_index BEGIN
    INSERT INTO skill_fts(skill_fts, rowid, path, title, body, tags)
    VALUES ('delete', old.rowid, old.path, old.title, old.body, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS skill_index_au AFTER UPDATE ON skill_index BEGIN
    INSERT INTO skill_fts(skill_fts, rowid, path, title, body, tags)
    VALUES ('delete', old.rowid, old.path, old.title, old.body, old.tags);
    INSERT INTO skill_fts(rowid, path, title, body, tags)
    VALUES (new.rowid, new.path, new.title, new.body, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS memory_index_ai AFTER INSERT ON memory_index BEGIN
    INSERT INTO memory_fts(rowid, path, title, body)
    VALUES (new.rowid, new.path, new.title, new.body);
END;

CREATE TRIGGER IF NOT EXISTS memory_index_ad AFTER DELETE ON memory_index BEGIN
    INSERT INTO memory_fts(memory_fts, rowid, path, title, body)
    VALUES ('delete', old.rowid, old.path, old.title, old.body);
END;

CREATE TRIGGER IF NOT EXISTS memory_index_au AFTER UPDATE ON memory_index BEGIN
    INSERT INTO memory_fts(memory_fts, rowid, path, title, body)
    VALUES ('delete', old.rowid, old.path, old.title, old.body);
    INSERT INTO memory_fts(rowid, path, title, body)
    VALUES (new.rowid, new.path, new.title, new.body);
END;

CREATE TRIGGER IF NOT EXISTS patterns_ai AFTER INSERT ON patterns BEGIN
    INSERT INTO pattern_fts(rowid, kind, signature)
    VALUES (new.id, new.kind, new.signature);
END;

CREATE TRIGGER IF NOT EXISTS patterns_ad AFTER DELETE ON patterns BEGIN
    INSERT INTO pattern_fts(pattern_fts, rowid, kind, signature)
    VALUES ('delete', old.id, old.kind, old.signature);
END;

CREATE TRIGGER IF NOT EXISTS patterns_au AFTER UPDATE ON patterns BEGIN
    INSERT INTO pattern_fts(pattern_fts, rowid, kind, signature)
    VALUES ('delete', old.id, old.kind, old.signature);
    INSERT INTO pattern_fts(rowid, kind, signature)
    VALUES (new.id, new.kind, new.signature);
END;
