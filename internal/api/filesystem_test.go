package api

import (
	"context"
	"io"
	"net"
	"path/filepath"
	"testing"

	fspb "github.com/oikos/oikos/gen/go/filesystem"
	"github.com/oikos/oikos/internal/filesystem"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestFilesystemStreamingUploadDownload(t *testing.T) {
	fs := filesystem.NewManager(filepath.Join(t.TempDir(), "servers"))
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	fspb.RegisterFilesystemServiceServer(srv, NewFilesystemService(fs, 1024*1024, 4))
	go srv.Serve(lis)
	defer srv.Stop()
	conn, err := grpc.NewClient("passthrough:///buf", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := fspb.NewFilesystemServiceClient(conn)
	up, err := client.Upload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{[]byte("hell"), []byte("o world")} {
		if err := up.Send(&fspb.UploadChunk{ServerId: "s1", Path: "hello.txt", Data: data, Last: false}); err != nil {
			t.Fatal(err)
		}
	}
	if err := up.Send(&fspb.UploadChunk{ServerId: "s1", Path: "hello.txt", Last: true}); err != nil {
		t.Fatal(err)
	}
	resp, err := up.CloseAndRecv()
	if err != nil {
		t.Fatal(err)
	}
	if resp.SizeBytes != 11 {
		t.Fatalf("size=%d", resp.SizeBytes)
	}
	dl, err := client.Download(context.Background(), &fspb.DownloadRequest{ServerId: "s1", Path: "hello.txt"})
	if err != nil {
		t.Fatal(err)
	}
	var got []byte
	for {
		chunk, err := dl.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, chunk.Data...)
		if chunk.Last {
			break
		}
	}
	if string(got) != "hello world" {
		t.Fatalf("got=%q", got)
	}
	_, err = client.List(context.Background(), &fspb.FileRequest{ServerId: "s1", Path: "../../etc"})
	if err == nil {
		t.Fatal("traversal harus ditolak")
	}
}
