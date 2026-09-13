package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ sql *sql.DB }

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("pragma: %w", err)
	}
	return &DB{sql: db}, nil
}

func (d *DB) Close() error { return d.sql.Close() }

// migrationCandidates dicoba berurutan: binary dijalankan dari repo root,
// atau go test yang workdir-nya direktori package ini.
var migrationCandidates = []string{
	"migrations/0001_init.sql",
	"../../migrations/0001_init.sql",
}

var networkingMigrationCandidates = []string{
	"migrations/0002_networking.sql",
	"../../migrations/0002_networking.sql",
}

var observabilityMigrationCandidates = []string{
	"migrations/0003_observability.sql",
	"../../migrations/0003_observability.sql",
}

func (d *DB) Migrate() error {
	var data []byte
	var err error
	for _, p := range migrationCandidates {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("baca migrasi (jalankan dari repo root): %w", err)
	}
	if _, err := d.sql.Exec(string(data)); err != nil {
		return fmt.Errorf("aplikasi migrasi: %w", err)
	}
	return nil
}

func (d *DB) MigrateNetworking() error {
	var data []byte
	var err error
	for _, p := range networkingMigrationCandidates {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("baca migrasi networking: %w", err)
	}
	if _, err := d.sql.Exec(string(data)); err != nil {
		return fmt.Errorf("aplikasi migrasi networking: %w", err)
	}
	return nil
}

func (d *DB) MigrateObservability() error {
	var data []byte
	var err error
	for _, p := range observabilityMigrationCandidates {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("baca migrasi observability: %w", err)
	}
	if _, err := d.sql.Exec(string(data)); err != nil {
		return fmt.Errorf("aplikasi migrasi observability: %w", err)
	}
	return nil
}

func (d *DB) CreateEgg(e Egg) error {
	_, err := d.sql.Exec(`INSERT INTO eggs(id,name,dockerfile_path,metadata_path) VALUES(?,?,?,?)`,
		e.ID, e.Name, e.DockerfilePath, e.MetadataPath)
	return err
}

// EnsureEgg mendaftarkan egg bila belum ada (no-op bila sudah terdaftar).
func (d *DB) EnsureEgg(e Egg) error {
	_, err := d.sql.Exec(`INSERT OR IGNORE INTO eggs(id,name,dockerfile_path,metadata_path) VALUES(?,?,?,?)`,
		e.ID, e.Name, e.DockerfilePath, e.MetadataPath)
	return err
}

func (d *DB) CreateServer(s Server) error {
	_, err := d.sql.Exec(`INSERT INTO servers(id,name,egg_id,container_id,status,startup_command,environment) VALUES(?,?,?,?,?,?,?)`,
		s.ID, s.Name, s.EggID, nullIfEmpty(s.ContainerID), s.Status, s.StartupCommand, s.Environment)
	return err
}

func (d *DB) GetServer(id string) (Server, error) {
	var s Server
	var cid sql.NullString
	err := d.sql.QueryRow(`SELECT id,name,egg_id,container_id,status,startup_command,environment FROM servers WHERE id=?`, id).
		Scan(&s.ID, &s.Name, &s.EggID, &cid, &s.Status, &s.StartupCommand, &s.Environment)
	if err != nil {
		return Server{}, err
	}
	s.ContainerID = cid.String
	return s, nil
}

func (d *DB) UpdateServerStatus(id, status string) error {
	_, err := d.sql.Exec(`UPDATE servers SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, id)
	return err
}

func (d *DB) SetServerContainer(id, containerID string) error {
	_, err := d.sql.Exec(`UPDATE servers SET container_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, containerID, id)
	return err
}

func (d *DB) DeleteServer(id string) error {
	_, err := d.sql.Exec(`DELETE FROM servers WHERE id=?`, id)
	return err
}

func (d *DB) ListServers() ([]Server, error) {
	rows, err := d.sql.Query(`SELECT id,name,egg_id,container_id,status,startup_command,environment FROM servers ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Server
	for rows.Next() {
		var s Server
		var cid sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.EggID, &cid, &s.Status, &s.StartupCommand, &s.Environment); err != nil {
			return nil, err
		}
		s.ContainerID = cid.String
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) SaveNode(n Node) error {
	_, err := d.sql.Exec(`INSERT OR REPLACE INTO nodes(id,name,panel_url,cert_path,key_path,paired_at) VALUES(?,?,?,?,?,?)`,
		n.ID, n.Name, n.PanelURL, n.CertPath, n.KeyPath, n.PairedAt.UTC().Format(time.RFC3339))
	return err
}

func (d *DB) GetNode() (Node, error) {
	var n Node
	var pairedAt string
	err := d.sql.QueryRow(`SELECT id,name,panel_url,cert_path,key_path,paired_at FROM nodes LIMIT 1`).
		Scan(&n.ID, &n.Name, &n.PanelURL, &n.CertPath, &n.KeyPath, &pairedAt)
	if err != nil {
		return Node{}, err
	}
	n.PairedAt, err = time.Parse(time.RFC3339, pairedAt)
	if err != nil {
		return Node{}, fmt.Errorf("parse paired_at: %w", err)
	}
	return n, nil
}

func (d *DB) SaveTunnel(t Tunnel) error {
	_, err := d.sql.Exec(`INSERT INTO tunnels(id,server_id,local_port,remote_port,protocol,status,last_connected) VALUES(?,?,?,?,?,?,NULLIF(?,''))
		ON CONFLICT(server_id,local_port,protocol) DO UPDATE SET id=excluded.id, remote_port=excluded.remote_port, status=excluded.status`,
		t.ID, t.ServerID, t.LocalPort, nullableInt(t.RemotePort), t.Protocol, t.Status, formatTime(t.LastConnected))
	return err
}

func (d *DB) GetTunnelByMapping(serverID string, localPort int, protocol string) (Tunnel, error) {
	var t Tunnel
	var remote sql.NullInt64
	var last sql.NullString
	err := d.sql.QueryRow(`SELECT id,server_id,local_port,remote_port,protocol,status,last_connected FROM tunnels WHERE server_id=? AND local_port=? AND protocol=?`, serverID, localPort, protocol).
		Scan(&t.ID, &t.ServerID, &t.LocalPort, &remote, &t.Protocol, &t.Status, &last)
	if err != nil {
		return Tunnel{}, err
	}
	if remote.Valid {
		t.RemotePort = int(remote.Int64)
	}
	if last.Valid {
		t.LastConnected, err = time.Parse(time.RFC3339, last.String)
	}
	return t, err
}

func (d *DB) ListTunnels(serverID string) ([]Tunnel, error) {
	rows, err := d.sql.Query(`SELECT id,server_id,local_port,remote_port,protocol,status,last_connected FROM tunnels WHERE server_id=? ORDER BY local_port`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tunnel
	for rows.Next() {
		var t Tunnel
		var remote sql.NullInt64
		var last sql.NullString
		if err := rows.Scan(&t.ID, &t.ServerID, &t.LocalPort, &remote, &t.Protocol, &t.Status, &last); err != nil {
			return nil, err
		}
		if remote.Valid {
			t.RemotePort = int(remote.Int64)
		}
		if last.Valid {
			t.LastConnected, err = time.Parse(time.RFC3339, last.String)
			if err != nil {
				return nil, err
			}
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) SaveNetworkGroup(g NetworkGroup) error {
	_, err := d.sql.Exec(`INSERT INTO network_groups(id,name) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name`, g.ID, g.Name)
	return err
}

func (d *DB) ReplaceNetworkGroupMembers(groupID string, members []NetworkGroupMember) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM network_group_members WHERE group_id=?`, groupID); err != nil {
		tx.Rollback()
		return err
	}
	for _, member := range members {
		if _, err := tx.Exec(`INSERT INTO network_group_members(group_id,node_id,private_ip) VALUES(?,?,?)`, groupID, member.NodeID, member.PrivateIP); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) ListNetworkGroupMembers(groupID string) ([]NetworkGroupMember, error) {
	rows, err := d.sql.Query(`SELECT group_id,node_id,private_ip FROM network_group_members WHERE group_id=? ORDER BY node_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NetworkGroupMember
	for rows.Next() {
		var m NetworkGroupMember
		if err := rows.Scan(&m.GroupID, &m.NodeID, &m.PrivateIP); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *DB) DeleteNetworkGroupMember(groupID, nodeID string) error {
	_, err := d.sql.Exec(`DELETE FROM network_group_members WHERE group_id=? AND node_id=?`, groupID, nodeID)
	return err
}

type Event struct {
	ID        string
	Type      string
	ServerID  string
	Severity  string
	Message   string
	Metadata  string
	CreatedAt time.Time
}

type EventFilter struct {
	ServerID, Type string
	From, To       time.Time
	Limit          int
}

func (d *DB) SaveEvent(e Event) error {
	if e.ID == "" || e.Type == "" || e.Message == "" {
		return fmt.Errorf("event id, type, dan message wajib diisi")
	}
	if e.Severity == "" {
		e.Severity = "info"
	}
	if e.Metadata == "" {
		e.Metadata = "{}"
	}
	var valid int
	if err := d.sql.QueryRow(`SELECT json_valid(?)`, e.Metadata).Scan(&valid); err != nil {
		return err
	}
	if valid != 1 {
		return fmt.Errorf("metadata event bukan JSON valid")
	}
	_, err := d.sql.Exec(`INSERT INTO events(id,type,server_id,severity,message,metadata,created_at) VALUES(?,?,?,?,?,?,?)`, e.ID, e.Type, nullIfEmpty(e.ServerID), e.Severity, e.Message, e.Metadata, e.CreatedAt.UTC().Format(time.RFC3339))
	return err
}

func (d *DB) ListEvents(f EventFilter) ([]Event, error) {
	query := `SELECT id,type,server_id,severity,message,metadata,created_at FROM events WHERE 1=1`
	args := []any{}
	if f.ServerID != "" {
		query += ` AND server_id=?`
		args = append(args, f.ServerID)
	}
	if f.Type != "" {
		query += ` AND type=?`
		args = append(args, f.Type)
	}
	if !f.From.IsZero() {
		query += ` AND created_at>=?`
		args = append(args, f.From.UTC().Format(time.RFC3339))
	}
	if !f.To.IsZero() {
		query += ` AND created_at<=?`
		args = append(args, f.To.UTC().Format(time.RFC3339))
	}
	query += ` ORDER BY created_at DESC`
	if f.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, f.Limit)
	} else {
		query += ` LIMIT 1000`
	}
	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var sid, created string
		if err := rows.Scan(&e.ID, &e.Type, &sid, &e.Severity, &e.Message, &e.Metadata, &created); err != nil {
			return nil, err
		}
		e.ServerID = sid
		e.CreatedAt, err = time.Parse(time.RFC3339, created)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) PruneEvents(ctx context.Context, before time.Time) error {
	_, err := d.sql.ExecContext(ctx, `DELETE FROM events WHERE created_at < ?`, before.UTC().Format(time.RFC3339))
	return err
}

func (d *DB) DeleteTunnelByMapping(serverID string, localPort int, protocol string) error {
	_, err := d.sql.Exec(`DELETE FROM tunnels WHERE server_id=? AND local_port=? AND protocol=?`, serverID, localPort, protocol)
	return err
}

func (d *DB) UpdateTunnelStatus(id, status string) error {
	_, err := d.sql.Exec(`UPDATE tunnels SET status=?, last_connected=CASE WHEN ?='connected' THEN CURRENT_TIMESTAMP ELSE last_connected END WHERE id=?`, status, status, id)
	return err
}

func nullableInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func (d *DB) SetResourceLimits(l ResourceLimits) error {
	_, err := d.sql.Exec(`INSERT INTO resource_limits(server_id,cpu_limit,memory_limit_mb,disk_limit_mb,pid_limit,bandwidth_kbps)
		VALUES(?,?,?,?,?,?) ON CONFLICT(server_id) DO UPDATE SET cpu_limit=excluded.cpu_limit, memory_limit_mb=excluded.memory_limit_mb,
		disk_limit_mb=excluded.disk_limit_mb, pid_limit=excluded.pid_limit, bandwidth_kbps=excluded.bandwidth_kbps`,
		l.ServerID, l.CPULimit, l.MemoryLimitMB, l.DiskLimitMB, l.PIDLimit, l.BandwidthKbps)
	return err
}

func (d *DB) GetResourceLimits(serverID string) (ResourceLimits, error) {
	var l ResourceLimits
	err := d.sql.QueryRow(`SELECT server_id,cpu_limit,memory_limit_mb,disk_limit_mb,pid_limit,bandwidth_kbps FROM resource_limits WHERE server_id=?`, serverID).
		Scan(&l.ServerID, &l.CPULimit, &l.MemoryLimitMB, &l.DiskLimitMB, &l.PIDLimit, &l.BandwidthKbps)
	return l, err
}

func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
