package agent

import (
	"context"
	"crypto/tls"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"

	"github.com/oikos/oikos/internal/security"
)

// ComputeBackoff mengembalikan 2^attempt detik, cap 30 detik.
func ComputeBackoff(attempt int) time.Duration {
	d := time.Second << attempt
	if d <= 0 || d > 30*time.Second {
		return 30 * time.Second
	}
	return d
}

// DialPanel membuka koneksi gRPC mTLS ke Panel, menunggu READY dengan backoff.
func DialPanel(ctx context.Context, panelAddr, certPath, keyPath, caPath string) (*grpc.ClientConn, error) {
	tlsCfg, err := security.LoadClientTLS(certPath, keyPath, caPath)
	if err != nil {
		return nil, err
	}
	return dialWithConfig(ctx, panelAddr, tlsCfg)
}

func dialWithConfig(ctx context.Context, target string, tlsCfg *tls.Config, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts := append([]grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))}, extra...)
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}
	if err := waitReady(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func waitReady(ctx context.Context, conn *grpc.ClientConn) error {
	attempt := 0
	conn.Connect()
	for {
		if conn.GetState() == connectivity.Ready {
			return nil
		}
		timer := time.NewTimer(ComputeBackoff(attempt))
		if attempt < 10 {
			attempt++
		}
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			conn.Connect()
		}
	}
}
