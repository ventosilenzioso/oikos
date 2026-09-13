CREATE TABLE IF NOT EXISTS plugins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    binary_path TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 0,
    subscribed_events TEXT NOT NULL DEFAULT '[]',
    installed_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_plugins_enabled ON plugins(enabled);
