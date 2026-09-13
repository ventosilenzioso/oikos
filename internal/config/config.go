package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type NodeConfig struct {
	DataDir  string `yaml:"data_dir"`
	CertPath string `yaml:"cert_path"`
	KeyPath  string `yaml:"key_path"`
}

type PanelConfig struct {
	Address string `yaml:"address"`
	CAPath  string `yaml:"ca_path"`
}

type RuntimeConfig struct {
	Engine           string `yaml:"engine"`
	DockerSocket     string `yaml:"docker_socket"`
	FRPBinary        string `yaml:"frp_binary"`
	FRPConfig        string `yaml:"frp_config"`
	FRPServerAddr    string `yaml:"frp_server_addr"`
	FRPServerPort    int    `yaml:"frp_server_port"`
	FRPToken         string `yaml:"frp_token"`
	FRPRemotePortMin int    `yaml:"frp_remote_port_min"`
	FRPRemotePortMax int    `yaml:"frp_remote_port_max"`
}

type APIConfig struct {
	LocalGRPCPort int `yaml:"local_grpc_port"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

type Duration time.Duration

func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var raw string
	if err := unmarshal(&raw); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) Duration() time.Duration { return time.Duration(d) }

type ObservabilityConfig struct {
	BindAddr           string   `yaml:"bind_addr"`
	MetricsPath        string   `yaml:"metrics_path"`
	HealthPath         string   `yaml:"health_path"`
	EventRetentionDays int      `yaml:"event_retention_days"`
	MetricsInterval    Duration `yaml:"metrics_interval"`
	HealthInterval     Duration `yaml:"health_interval"`
}

type Config struct {
	Node          NodeConfig          `yaml:"node"`
	Panel         PanelConfig         `yaml:"panel"`
	Runtime       RuntimeConfig       `yaml:"runtime"`
	API           APIConfig           `yaml:"api"`
	Log           LogConfig           `yaml:"log"`
	Observability ObservabilityConfig `yaml:"observability"`
}

func Load(path string) (*Config, error) {
	c := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("baca config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return c, nil
}
