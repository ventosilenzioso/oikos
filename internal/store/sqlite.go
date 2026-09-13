package store

import (
	"database/sql"
	"fmt"
	"os"

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

func (d *DB) CreateEgg(e Egg) error {
	_, err := d.sql.Exec(`INSERT INTO eggs(id,name,dockerfile_path,metadata_path) VALUES(?,?,?,?)`,
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
