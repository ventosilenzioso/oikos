package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type NodeConfig struct {
	DataDir  string `yaml:"data_dir"`
	CertPath string `yaml:"cert_path"`
	KeyPath  string `yaml:"key_path"`
}

type PanelConfig struct {
	Address string `yaml:"address"`
}

type RuntimeConfig struct {
	Engine       string `yaml:"engine"`
	DockerSocket string `yaml:"docker_socket"`
}

type APIConfig struct {
	LocalGRPCPort int `yaml:"local_grpc_port"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

type Config struct {
	Node    NodeConfig    `yaml:"node"`
	Panel   PanelConfig   `yaml:"panel"`
	Runtime RuntimeConfig `yaml:"runtime"`
	API     APIConfig     `yaml:"api"`
	Log     LogConfig     `yaml:"log"`
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
