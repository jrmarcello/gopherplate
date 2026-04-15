-- +goose Up

-- pg_trgm powers GIN indexes for ILIKE '%term%' substring search on name/email.
-- Without these indexes, the List handlers' ILIKE filters fall back to a
-- sequential scan on every call. Tradeoff: trigram indexes are ~5-10x the size
-- of a B-tree and slow writes slightly — acceptable for read-heavy list paths.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_users_name_trgm  ON users USING gin (name  gin_trgm_ops);
CREATE INDEX idx_users_email_trgm ON users USING gin (email gin_trgm_ops);
CREATE INDEX idx_roles_name_trgm  ON roles USING gin (name  gin_trgm_ops);

-- The existing idx_users_active_created partial index is only used when the
-- query filters by active = true. The common List path (no ActiveOnly filter)
-- ordered by created_at DESC falls back to a Seq Scan + Sort; this unconditional
-- index covers that path.
CREATE INDEX idx_users_created_at ON users(created_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_users_created_at;
DROP INDEX IF EXISTS idx_roles_name_trgm;
DROP INDEX IF EXISTS idx_users_email_trgm;
DROP INDEX IF EXISTS idx_users_name_trgm;
-- Keep pg_trgm extension: other migrations may depend on it.
