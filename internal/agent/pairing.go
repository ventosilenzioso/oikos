package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"

	nodepb "github.com/oikos/oikos/gen/go/node"
	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/security"
	"github.com/oikos/oikos/internal/store"
)

// PairWithToken performs the initial pairing step: validates the one-time token,
// creates the data directory, and writes the initial config.yaml.
// Full CSR/certificate mTLS exchange is handled by PairWithPanel.
func PairWithToken(token string, cfg *config.Config, configPath string) error {
	if err := security.ValidateFormat(token); err != nil {
		return fmt.Errorf("token pairing: %w", err)
	}
	if err := os.MkdirAll(cfg.Node.DataDir, 0750); err != nil {
		return fmt.Errorf("buat data dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Node.CertPath), 0750); err != nil {
		return fmt.Errorf("buat dir cert: %w", err)
	}
	return writeConfig(cfg, configPath)
}

// PairWithPanel performs full pairing: generates a key and CSR, exchanges it
// with the Panel, saves key/certificate files with mode 0600, records the node
// in SQLite, and writes the config. A nil client dials panelAddr over plaintext
// for production/CLI use; tests inject a bufconn client.
func PairWithPanel(ctx context.Context, panelAddr, token, nodeName string, cfg *config.Config, db *store.DB, configPath string, client nodepb.NodeServiceClient) (string, error) {
	if err := security.ValidateFormat(token); err != nil {
		return "", fmt.Errorf("token pairing: %w", err)
	}
	if _, err := db.GetNode(); err == nil {
		return "", fmt.Errorf("node sudah paired, tolak pairing ulang")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("cek status pairing: %w", err)
	}
	priv, err := security.GenerateKey()
	if err != nil {
		return "", err
	}
	csrPEM, err := security.GenerateCSR(priv, nodeName)
	if err != nil {
		return "", err
	}
	if client == nil {
		conn, err := grpc.NewClient(panelAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return "", fmt.Errorf("dial panel: %w", err)
		}
		defer conn.Close()
		client = nodepb.NewNodeServiceClient(conn)
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := client.Pair(callCtx, &nodepb.PairingRequest{Token: token, NodeName: nodeName, CsrPem: csrPEM})
	if err != nil {
		return "", fmt.Errorf("pair ke panel: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Node.KeyPath), 0750); err != nil {
		return "", fmt.Errorf("buat dir cert: %w", err)
	}
	if err := security.SaveKey(cfg.Node.KeyPath, priv); err != nil {
		return "", err
	}
	if err := security.SaveCert(cfg.Node.CertPath, resp.CertPem); err != nil {
		return "", err
	}
	if err := db.SaveNode(store.Node{
		ID: resp.NodeId, Name: nodeName, PanelURL: panelAddr,
		CertPath: cfg.Node.CertPath, KeyPath: cfg.Node.KeyPath,
		PairedAt: time.Now().UTC(),
	}); err != nil {
		return "", fmt.Errorf("simpan node: %w", err)
	}
	if err := writeConfig(cfg, configPath); err != nil {
		return "", err
	}
	return resp.NodeId, nil
}

func writeConfig(cfg *config.Config, configPath string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("tulis config: %w", err)
	}
	return nil
}
