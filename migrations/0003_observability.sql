CREATE TABLE IF NOT EXISTS events (
    id         TEXT PRIMARY KEY,
    type       TEXT NOT NULL,
    server_id  TEXT REFERENCES servers(id) ON DELETE CASCADE,
    severity   TEXT NOT NULL DEFAULT 'info',
    message    TEXT NOT NULL,
    metadata   TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at);
CREATE INDEX IF NOT EXISTS idx_events_server_id ON events(server_id);

ALTER TABLE servers ADD COLUMN restart_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE servers ADD COLUMN last_crash_at DATETIME;
