CREATE TABLE IF NOT EXISTS backups (
    id TEXT PRIMARY KEY,
    server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    checksum_sha256 TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'creating',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS sftp_credentials (
    id TEXT PRIMARY KEY,
    server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    username TEXT NOT NULL,
    public_key TEXT,
    password_hash TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_backups_server_id ON backups(server_id);
