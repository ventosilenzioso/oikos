CREATE TABLE IF NOT EXISTS plugins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    binary_path TEXT NOT NULL,
    config_path TEXT NOT NULL DEFAULT '',
    sha256 TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 0,
    subscribed_events TEXT NOT NULL DEFAULT '[]',
    allowed_routes TEXT NOT NULL DEFAULT '[]',
    installed_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_plugins_enabled ON plugins(enabled);
