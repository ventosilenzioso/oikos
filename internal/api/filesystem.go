package api

import (
	"context"
	"errors"
	"io"
	"strings"

	fspb "github.com/oikos/oikos/gen/go/filesystem"
	"github.com/oikos/oikos/internal/filesystem"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type FilesystemService struct {
	fspb.UnimplementedFilesystemServiceServer
	manager   filesystem.Manager
	maxUpload int64
	chunkSize int
}

func NewFilesystemService(m filesystem.Manager, maxUpload int64, chunkSize int) *FilesystemService {
	if chunkSize <= 0 {
		chunkSize = 1024 * 1024
	}
	return &FilesystemService{manager: m, maxUpload: maxUpload, chunkSize: chunkSize}
}
func mapFilesystemError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, filesystem.ErrPathOutsideRoot) || errors.Is(err, filesystem.ErrPermission) {
		return status.Error(codes.PermissionDenied, err.Error())
	}
	if errors.Is(err, filesystem.ErrNotFound) || errors.Is(err, filesystem.ErrServerNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	if errors.Is(err, filesystem.ErrQuotaExceeded) {
		return status.Error(codes.ResourceExhausted, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}
func (s *FilesystemService) List(ctx context.Context, req *fspb.FileRequest) (*fspb.ListResponse, error) {
	items, err := s.manager.List(ctx, req.ServerId, req.Path)
	if err != nil {
		return nil, mapFilesystemError(err)
	}
	out := &fspb.ListResponse{}
	for _, i := range items {
		out.Files = append(out.Files, &fspb.FileInfo{Name: i.Name, Path: i.Path, IsDir: i.IsDir, SizeBytes: i.SizeBytes, ModifiedUnix: i.ModifiedAt.Unix(), Mode: i.Mode})
	}
	return out, nil
}
func (s *FilesystemService) Delete(ctx context.Context, req *fspb.FileRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, mapFilesystemError(s.manager.Delete(ctx, req.ServerId, req.Path, false))
}
func (s *FilesystemService) Rename(ctx context.Context, req *fspb.RenameRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, mapFilesystemError(s.manager.Rename(ctx, req.ServerId, req.OldPath, req.NewPath))
}
func (s *FilesystemService) Copy(ctx context.Context, req *fspb.RenameRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, mapFilesystemError(s.manager.Copy(ctx, req.ServerId, req.OldPath, req.NewPath))
}
func (s *FilesystemService) CreateDir(ctx context.Context, req *fspb.FileRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, mapFilesystemError(s.manager.CreateDir(ctx, req.ServerId, req.Path))
}
func (s *FilesystemService) Upload(stream grpc.ClientStreamingServer[fspb.UploadChunk, fspb.UploadResponse]) error {
	var serverID, path string
	var reader *io.PipeReader
	var writer *io.PipeWriter
	var total int64
	first := true
	ctx := stream.Context()
	errCh := make(chan error, 1)
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if first {
			serverID, path = chunk.ServerId, chunk.Path
			if serverID == "" || path == "" {
				return status.Error(codes.InvalidArgument, "server_id dan path wajib")
			}
			reader, writer = io.Pipe()
			go func() { errCh <- s.manager.Write(ctx, serverID, path, reader) }()
			first = false
		}
		total += int64(len(chunk.Data))
		if s.maxUpload > 0 && total > s.maxUpload {
			_ = reader.CloseWithError(filesystem.ErrQuotaExceeded)
			return mapFilesystemError(filesystem.ErrQuotaExceeded)
		}
		if _, err := writer.Write(chunk.Data); err != nil {
			return err
		}
		if chunk.Last {
			break
		}
	}
	if first {
		return status.Error(codes.InvalidArgument, "upload kosong")
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := <-errCh; err != nil {
		return mapFilesystemError(err)
	}
	return stream.SendAndClose(&fspb.UploadResponse{SizeBytes: total})
}
func (s *FilesystemService) Download(req *fspb.DownloadRequest, stream fspb.FilesystemService_DownloadServer) error {
	r, err := s.manager.Read(stream.Context(), req.ServerId, req.Path)
	if err != nil {
		return mapFilesystemError(err)
	}
	defer r.Close()
	buf := make([]byte, s.chunkSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&fspb.DownloadChunk{Data: append([]byte(nil), buf[:n]...)}); sendErr != nil {
				return sendErr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return stream.Send(&fspb.DownloadChunk{Last: true})
}

var _ = strings.Builder{}
