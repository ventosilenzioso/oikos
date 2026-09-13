package egg

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadDir(dir string) (Egg, error) {
	data, err := os.ReadFile(filepath.Join(dir, "egg.yaml"))
	if err != nil {
		return Egg{}, fmt.Errorf("baca egg.yaml: %w", err)
	}
	var e Egg
	if err := yaml.Unmarshal(data, &e); err != nil {
		return Egg{}, fmt.Errorf("parse egg.yaml: %w", err)
	}
	return e, nil
}
