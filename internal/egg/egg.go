package egg

type EggBuild struct {
	Dockerfile string `yaml:"dockerfile"`
}

type EggStartup struct {
	Command      string `yaml:"command"`
	StopSignal   string `yaml:"stop_signal"`
	StopCommand  string `yaml:"stop_command"`
	ReadinessLog string `yaml:"readiness_log"`
}

type EggVariable struct {
	Name     string `yaml:"name"`
	Env      string `yaml:"env"`
	Default  string `yaml:"default"`
	Editable bool   `yaml:"editable"`
}

type EggPort struct {
	Name     string `yaml:"name"`
	Default  int    `yaml:"default"`
	Protocol string `yaml:"protocol"`
}

type Egg struct {
	Name        string        `yaml:"name"`
	Slug        string        `yaml:"slug"`
	Description string        `yaml:"description"`
	Build       EggBuild      `yaml:"build"`
	Startup     EggStartup    `yaml:"startup"`
	Variables   []EggVariable `yaml:"variables"`
	Ports       []EggPort     `yaml:"ports"`
}
