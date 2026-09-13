package security

import "testing"

func TestGenerateAndValidate(t *testing.T) {
	tok, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateFormat(tok); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFormat(""); err == nil {
		t.Fatal("token kosong harus ditolak")
	}
	if err := ValidateFormat("xyz"); err == nil {
		t.Fatal("token pendek harus ditolak")
	}
}
