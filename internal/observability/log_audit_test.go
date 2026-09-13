package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperationalLoggingUsesStructuredLogger(t *testing.T) {
	root := filepath.Join("..")
	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		for _, token := range []string{"fmt.Print(", "fmt.Println(", "fmt.Printf(", "log.Print(", "log.Println(", "log.Printf("} {
			if strings.Contains(text, token) {
				violations = append(violations, path+": "+token)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("operational log violations: %v", violations)
	}
}
