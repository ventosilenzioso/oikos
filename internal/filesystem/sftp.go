package filesystem

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/store"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type CredentialStore interface {
	GetSFTPCredential(string) (store.SFTPCredential, error)
}

// SFTPServer provides an isolated SSH listener. File operations are delegated
// to Manager; the transport accepts only registered public-key credentials.
type SFTPServer struct {
	cfg      config.SFTPConfig
	manager  Manager
	creds    CredentialStore
	listener net.Listener
	config   *ssh.ServerConfig
}

func (s *SFTPServer) Addr() net.Addr {
	if s.listener == nil {
		return nil
	}
	return s.listener.Addr()
}

func NewSFTPServer(cfg config.SFTPConfig, manager Manager, creds CredentialStore) (*SFTPServer, error) {
	if manager == nil || creds == nil {
		return nil, fmt.Errorf("manager dan credential store wajib")
	}
	key, err := loadOrCreateHostKey(cfg.HostKeyPath)
	if err != nil {
		return nil, err
	}
	serverCfg := &ssh.ServerConfig{PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		c, err := creds.GetSFTPCredential(conn.User())
		if err != nil {
			return nil, err
		}
		allowed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(c.PublicKey))
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(allowed.Marshal(), key.Marshal()) {
			return nil, fmt.Errorf("public key tidak cocok")
		}
		return &ssh.Permissions{Extensions: map[string]string{"server_id": c.ServerID}}, nil
	}}
	serverCfg.AddHostKey(key)
	return &SFTPServer{cfg: cfg, manager: manager, creds: creds, config: serverCfg}, nil
}

func (s *SFTPServer) ListenAndServe(ctx context.Context) error {
	lis, err := net.Listen("tcp", s.cfg.BindAddr)
	if err != nil {
		return err
	}
	s.listener = lis
	go func() { <-ctx.Done(); _ = lis.Close() }()
	for {
		conn, err := lis.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		go s.handle(conn)
	}
}
func (s *SFTPServer) Shutdown(context.Context) error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}
func (s *SFTPServer) handle(conn net.Conn) {
	defer conn.Close()
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.config)
	if err != nil {
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)
	serverID := ""
	if sshConn.Permissions != nil {
		serverID = sshConn.Permissions.Extensions["server_id"]
	}
	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "session only")
			continue
		}
		channel, requests, err := newChan.Accept()
		if err != nil {
			continue
		}
		go func() {
			for req := range requests {
				if req.Type == "subsystem" && len(req.Payload) >= 4 && string(req.Payload[4:]) == "sftp" {
					_ = req.Reply(true, nil)
					handlers := sftp.Handlers{FileGet: sftpReader{manager: s.manager, serverID: serverID}, FilePut: sftpWriter{manager: s.manager, serverID: serverID}, FileCmd: sftpCommander{manager: s.manager, serverID: serverID}, FileList: sftpLister{manager: s.manager, serverID: serverID}}
					rs := sftp.NewRequestServer(channel, handlers, sftp.WithStartDirectory("/"))
					_ = rs.Serve()
					_ = rs.Close()
					return
				}
				_ = req.Reply(false, nil)
			}
		}()
	}
}

type sftpReader struct {
	manager  Manager
	serverID string
}

func (h sftpReader) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	path, err := h.manager.ResolvePath(h.serverID, strings.TrimPrefix(r.Filepath, "/"))
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

type sftpWriter struct {
	manager  Manager
	serverID string
}

func (h sftpWriter) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	path, err := h.manager.ResolvePath(h.serverID, strings.TrimPrefix(r.Filepath, "/"))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
}

type sftpCommander struct {
	manager  Manager
	serverID string
}

func (h sftpCommander) Filecmd(r *sftp.Request) error {
	path := strings.TrimPrefix(r.Filepath, "/")
	switch r.Method {
	case "Mkdir":
		return h.manager.CreateDir(context.Background(), h.serverID, path)
	case "Remove":
		return h.manager.Delete(context.Background(), h.serverID, path, false)
	case "Rmdir":
		return h.manager.Delete(context.Background(), h.serverID, path, true)
	case "Rename":
		return h.manager.Rename(context.Background(), h.serverID, path, strings.TrimPrefix(r.Target, "/"))
	default:
		return nil
	}
}

type sftpLister struct {
	manager  Manager
	serverID string
}

func (h sftpLister) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	items, err := h.manager.List(context.Background(), h.serverID, strings.TrimPrefix(r.Filepath, "/"))
	if err != nil {
		return nil, err
	}
	infos := make([]os.FileInfo, 0, len(items))
	for _, item := range items {
		infos = append(infos, sftpFileInfo{item})
	}
	return sftpFileList(infos), nil
}

type sftpFileList []os.FileInfo

func (l sftpFileList) ListAt(out []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}
	n := copy(out, l[offset:])
	return n, nil
}

type sftpFileInfo struct{ FileInfo }

func (i sftpFileInfo) Name() string       { return i.FileInfo.Name }
func (i sftpFileInfo) Size() int64        { return i.FileInfo.SizeBytes }
func (i sftpFileInfo) Mode() os.FileMode  { return parseMode(i.FileInfo.Mode) }
func (i sftpFileInfo) ModTime() time.Time { return i.FileInfo.ModifiedAt }
func (i sftpFileInfo) IsDir() bool        { return i.FileInfo.IsDir }
func (i sftpFileInfo) Sys() any           { return nil }
func parseMode(mode string) os.FileMode {
	if strings.HasPrefix(mode, "d") {
		return os.ModeDir
	}
	return 0600
}

func loadOrCreateHostKey(path string) (ssh.Signer, error) {
	if data, err := os.ReadFile(path); err == nil {
		return ssh.ParsePrivateKey(data)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, err
	}
	return ssh.NewSignerFromKey(priv)
}
