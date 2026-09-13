CREATE TABLE IF NOT EXISTS tunnels (
    id              TEXT PRIMARY KEY,
    server_id       TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    local_port      INTEGER NOT NULL,
    remote_port     INTEGER,
    protocol        TEXT NOT NULL DEFAULT 'tcp',
    status          TEXT NOT NULL DEFAULT 'pending',
    last_connected  DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, local_port, protocol),
    UNIQUE(remote_port)
);

CREATE TABLE IF NOT EXISTS network_groups (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS network_group_members (
    group_id        TEXT NOT NULL REFERENCES network_groups(id) ON DELETE CASCADE,
    node_id         TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    private_ip      TEXT NOT NULL,
    PRIMARY KEY (group_id, node_id),
    UNIQUE(group_id, private_ip)
);

CREATE INDEX IF NOT EXISTS idx_tunnels_server_id ON tunnels(server_id);
CREATE INDEX IF NOT EXISTS idx_tunnels_status ON tunnels(status);
