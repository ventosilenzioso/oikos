package egg

import "strings"

func RenderStartup(cmd string, vars map[string]string) string {
	out := cmd
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}
