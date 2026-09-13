package agent

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	nodepb "github.com/oikos/oikos/gen/go/node"
	serverpb "github.com/oikos/oikos/gen/go/server"
	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/runtime"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestDaemonEndToEnd(t *testing.T) {
	tok := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	panel := newMockPanel(t, tok)

	plainLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	plainSrv := grpc.NewServer()
	nodepb.RegisterNodeServiceServer(plainSrv, panel)
	go plainSrv.Serve(plainLis)
	defer plainSrv.Stop()

	srvCertPEM, srvKey := panel.issue("panel.test")
	tlsLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tlsSrv := grpc.NewServer(grpc.Creds(mustServerTLS(t, srvCertPEM, srvKey, panel.caPEM)))
	nodepb.RegisterNodeServiceServer(tlsSrv, panel)
	go tlsSrv.Serve(tlsLis)
	defer tlsSrv.Stop()

	cfg, db, configPath := testConfigDB(t)
	pairCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	nodeID, err := PairWithPanel(pairCtx, plainLis.Addr().String(), tok, "node-e2e", cfg, db, configPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	lc := orchestrator.New(db, runtime.NewFake(), orchestrator.NewEventBus())
	runCtx, stop := context.WithCancel(context.Background())
	defer stop()
	envJSON, _ := json.Marshal(map[string]string{})
	panel.queue(&nodepb.NodeCommand{CommandId: "d1", Action: "create", Args: map[string]string{
		"name": "daemon-srv", "egg_id": "egg-1", "startup_command": "run", "environment": string(envJSON)}})
	localPort := freePort(t)
	done := make(chan error, 1)
	go func() {
		done <- Run(runCtx, DaemonArgs{
			PanelAddr: tlsLis.Addr().String(),
			CertPath:  cfg.Node.CertPath,
			KeyPath:   cfg.Node.KeyPath,
			CAPath:    panel.caPath(t),
			NodeID:    nodeID,
			LocalPort: localPort,
		}, lc)
	}()
	deadline := time.Now().Add(25 * time.Second)
	for {
		hb := panel.heartbeats()
		servers, err := db.ListServers()
		if err != nil {
			t.Fatal(err)
		}
		if len(hb) > 0 && len(servers) == 1 && servers[0].Name == "daemon-srv" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("daemon e2e macet: hb=%d servers=%+v", len(hb), servers)
		}
		time.Sleep(20 * time.Millisecond)
	}
	conn, err := grpc.NewClient(
		"127.0.0.1:"+strconv.Itoa(localPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sclient := serverpb.NewServerServiceClient(conn)
	lctx, lcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer lcancel()
	listed, err := sclient.List(lctx, &serverpb.ListServersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Servers) != 1 || listed.Servers[0].Name != "daemon-srv" {
		t.Fatalf("list api lokal salah: %+v", listed.Servers)
	}
	stop()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("Run = %v, mau context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run tidak berhenti setelah cancel")
	}
}

func TestRunInvokesHandoffOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	called := make(chan struct{}, 1)
	rt := &handoffRuntime{Runtime: runtime.NewFake()}
	lc := orchestrator.New(nil, rt, orchestrator.NewEventBus())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, DaemonArgs{LocalPort: freePort(t), Handoff: func() { called <- struct{}{} }}, lc)
	}()
	cancel()
	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("handoff was not invoked during cancellation")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
	if rt.stopCalls != 0 || rt.restartCalls != 0 || rt.deleteCalls != 0 {
		t.Fatalf("runtime lifecycle calls = stop:%d restart:%d delete:%d", rt.stopCalls, rt.restartCalls, rt.deleteCalls)
	}
}

type handoffRuntime struct {
	runtime.Runtime
	stopCalls, restartCalls, deleteCalls int
}

func (r *handoffRuntime) Stop(context.Context, string, int) error { r.stopCalls++; return nil }
func (r *handoffRuntime) Restart(context.Context, string) error   { r.restartCalls++; return nil }
func (r *handoffRuntime) Delete(context.Context, string, bool) error {
	r.deleteCalls++
	return nil
}
