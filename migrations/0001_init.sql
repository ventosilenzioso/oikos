-- 0001_init.sql

CREATE TABLE nodes (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    panel_url       TEXT NOT NULL,
    cert_path       TEXT NOT NULL,
    key_path        TEXT NOT NULL,
    paired_at       DATETIME NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE eggs (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    dockerfile_path TEXT NOT NULL,
    metadata_path   TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE servers (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    egg_id          TEXT NOT NULL REFERENCES eggs(id),
    container_id    TEXT,
    status          TEXT NOT NULL DEFAULT 'installing',
    startup_command TEXT NOT NULL,
    environment     TEXT NOT NULL DEFAULT '{}',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE resource_limits (
    server_id       TEXT PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
    cpu_limit       INTEGER,
    memory_limit_mb INTEGER,
    disk_limit_mb   INTEGER,
    pid_limit       INTEGER,
    bandwidth_kbps  INTEGER
);

CREATE INDEX idx_servers_status ON servers(status);
