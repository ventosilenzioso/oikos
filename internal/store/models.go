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
	RestartCount   int
	LastCrashAt    time.Time
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

type Tunnel struct {
	ID            string
	ServerID      string
	LocalPort     int
	RemotePort    int
	Protocol      string
	Status        string
	LastConnected time.Time
}

type NetworkGroup struct {
	ID   string
	Name string
}

type NetworkGroupMember struct {
	GroupID   string
	NodeID    string
	PrivateIP string
}

type Backup struct {
	ID          string
	ServerID    string
	FilePath    string
	SizeBytes   int64
	ChecksumSHA string
	Status      string
	CreatedAt   time.Time
	CompletedAt time.Time
}

type SFTPCredential struct {
	ID           string
	ServerID     string
	Username     string
	PublicKey    string
	PasswordHash string
}
