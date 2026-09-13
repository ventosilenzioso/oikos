package store

import "time"

type Server struct {
	ID             string
	Name           string
	EggID          string
	ContainerID    string
	Status         string
	StartupCommand string
	Environment    string
}

type Egg struct {
	ID             string
	Name           string
	DockerfilePath string
	MetadataPath   string
}

type ResourceLimits struct {
	ServerID      string
	CPULimit      int64
	MemoryLimitMB int64
	DiskLimitMB   int64
	PIDLimit      int64
	BandwidthKbps int64
}

type Node struct {
	ID       string
	Name     string
	PanelURL string
	CertPath string
	KeyPath  string
	PairedAt time.Time
}
