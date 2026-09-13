package config

import "time"

func Default() *Config {
	return &Config{
		Node: NodeConfig{
			DataDir:  "/var/lib/oikos",
			CertPath: "/etc/oikos/certs/node.crt",
			KeyPath:  "/etc/oikos/certs/node.key",
		},
		Panel: PanelConfig{Address: "panel.example.com:9091", CAPath: "/etc/oikos/certs/ca.crt"},
		Runtime: RuntimeConfig{
			Engine: "docker", DockerSocket: "unix:///var/run/docker.sock",
			FRPBinary: "/usr/local/bin/frpc", FRPConfig: "/var/lib/oikos/frpc.toml",
			FRPServerPort: 7000, FRPRemotePortMin: 30000, FRPRemotePortMax: 40000,
		},
		API: APIConfig{LocalGRPCPort: 9190},
		Log: LogConfig{Level: "info"},
		Observability: ObservabilityConfig{
			BindAddr: "127.0.0.1:9191", MetricsPath: "/metrics", HealthPath: "/healthz",
			EventRetentionDays: 30, MetricsInterval: Duration(15 * time.Second), HealthInterval: Duration(15 * time.Second),
		},
	}
}
