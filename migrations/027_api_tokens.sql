CREATE TABLE IF NOT EXISTS api_tokens (
	id UUID PRIMARY KEY,
	name TEXT NOT NULL,
	prefix TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	created_by TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS api_tokens_created_at_idx ON api_tokens (created_at DESC);
