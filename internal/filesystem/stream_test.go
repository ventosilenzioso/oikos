package filesystem

import (
	"context"
	"io"
	"testing"
)

type repeatReader struct{ remaining int64 }

func (r *repeatReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > r.remaining {
		n = r.remaining
	}
	for i := int64(0); i < n; i++ {
		p[i] = 'o'
	}
	r.remaining -= n
	return int(n), nil
}

func TestWriteReadStreamsLargeFile(t *testing.T) {
	m := NewManager(t.TempDir())
	wantSize := 16 * 1024 * 1024
	content := &repeatReader{remaining: int64(wantSize)}
	if err := m.Write(context.Background(), "srv", "large.bin", content); err != nil {
		t.Fatal(err)
	}
	r, err := m.Read(context.Background(), "srv", "large.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := io.Copy(io.Discard, r)
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(wantSize) {
		t.Fatalf("size=%d, want=%d", got, wantSize)
	}
}
