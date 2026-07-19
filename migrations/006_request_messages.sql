-- Conversation messages on requests (student ↔ admin)

CREATE TABLE IF NOT EXISTS request_messages (
    id           UUID PRIMARY KEY,
    request_id   UUID NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    author_role  TEXT NOT NULL CHECK (author_role IN ('student', 'admin')),
    body         TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS request_messages_request_idx ON request_messages (request_id, created_at);
