package config

func Default() *Config {
	return &Config{
		Node: NodeConfig{
			DataDir:  "/var/lib/oikos",
			CertPath: "/etc/oikos/certs/node.crt",
			KeyPath:  "/etc/oikos/certs/node.key",
		},
		Panel:   PanelConfig{Address: "panel.example.com:9091"},
		Runtime: RuntimeConfig{Engine: "docker", DockerSocket: "unix:///var/run/docker.sock"},
		API:     APIConfig{LocalGRPCPort: 9190},
		Log:     LogConfig{Level: "info"},
	}
}
