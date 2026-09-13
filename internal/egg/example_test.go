package egg

import "testing"

// TestExampleEgg memastikan egg contoh yang di-ship lolos validasi.
func TestExampleEgg(t *testing.T) {
	e, err := LoadDir("../../eggs/minecraft-vanilla")
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(e); err != nil {
		t.Fatal(err)
	}
	rendered := RenderStartup(e.Startup.Command, map[string]string{
		"MIN_MEMORY": "1024",
		"MAX_MEMORY": "2048",
	})
	if rendered != "java -Xms1024M -Xmx2048M -jar server.jar nogui" {
		t.Fatalf("render salah: %q", rendered)
	}
}
