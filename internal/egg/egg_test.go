package egg

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEgg(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "egg.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadAndValidateOK(t *testing.T) {
	dir := writeEgg(t, "name: \"Minecraft Vanilla\"\nslug: \"minecraft-vanilla\"\ndescription: \"test\"\nbuild:\n  dockerfile: \"./Dockerfile\"\nstartup:\n  command: \"java -Xmx{{MAX_MEMORY}}M -jar server.jar nogui\"\n  stop_signal: \"SIGTERM\"\nvariables:\n  - name: \"Maximum Memory\"\n    env: \"MAX_MEMORY\"\n    default: \"2048\"\n    editable: true\nports:\n  - name: \"game\"\n    default: 25565\n    protocol: \"tcp\"\n")
	e, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(e); err != nil {
		t.Fatal(err)
	}
	if e.Slug != "minecraft-vanilla" {
		t.Fatalf("slug = %q", e.Slug)
	}
}

func TestValidateRejectsMissingCommand(t *testing.T) {
	e := Egg{Name: "x", Slug: "x"}
	if err := Validate(e); err == nil {
		t.Fatal("harus error untuk startup.command kosong")
	}
}

func TestRenderStartup(t *testing.T) {
	got := RenderStartup("java -Xmx{{MAX_MEMORY}}M -jar s.jar", map[string]string{"MAX_MEMORY": "2048"})
	if got != "java -Xmx2048M -jar s.jar" {
		t.Fatalf("hasil = %q", got)
	}
}
