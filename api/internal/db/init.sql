CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email            TEXT NOT NULL UNIQUE,
  password_hash    TEXT NOT NULL,
  business_context JSONB,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS analyses (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  status           TEXT NOT NULL DEFAULT 'processing'
                     CHECK (status IN ('processing', 'done', 'error')),
  mode             TEXT NOT NULL
                     CHECK (mode IN ('pre_post', 'reference', 'post_mortem')),

  gcs_uri          TEXT NOT NULL,
  original_name    TEXT,

  user_id          UUID REFERENCES users(id) ON DELETE SET NULL,

  business_context JSONB NOT NULL,
  metrics_input    JSONB,

  gvi_result       JSONB,
  claude_result    JSONB,

  progress_msg     TEXT,
  error_msg        TEXT,

  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at     TIMESTAMPTZ
);

-- migrate: add user_id column to existing installations (no-op if already present)
ALTER TABLE analyses ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_analyses_created_at
  ON analyses (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_analyses_status_updated
  ON analyses (status, updated_at)
  WHERE status = 'processing';

CREATE INDEX IF NOT EXISTS idx_analyses_user_id
  ON analyses (user_id, created_at DESC);

-- migrate: add user_concept for async job runner access
ALTER TABLE analyses ADD COLUMN IF NOT EXISTS user_concept TEXT;
