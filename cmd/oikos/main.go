package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oikos/oikos/internal/agent"
	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/runtime/docker"
	"github.com/oikos/oikos/internal/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "oikos:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command (gunakan: server, install, daemon, diagnose)")
	}
	switch args[0] {
	case "server":
		return runServer(args[1:])
	case "install":
		return runInstall(args[1:])
	case "daemon":
		return runDaemon(args[1:])
	case "diagnose":
		return runDiagnose(args[1:])
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

var (
	flagConfig  string
	flagRuntime string
)

func bindGlobal(fs *flag.FlagSet) {
	fs.StringVar(&flagConfig, "config", "config.yaml", "path config.yaml")
	fs.StringVar(&flagRuntime, "runtime", "docker", "runtime engine: docker|fake")
}

func openLifecycle() (*orchestrator.Lifecycle, error) {
	return openLifecycleWith(flagRuntime)
}

func openLifecycleWith(engine string) (*orchestrator.Lifecycle, error) {
	cfg, err := config.Load(flagConfig)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Node.DataDir, 0750); err != nil {
		return nil, err
	}
	db, err := store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db"))
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(); err != nil {
		db.Close()
		return nil, err
	}
	var rt runtime.Runtime
	if engine == "fake" {
		rt = runtime.NewFake()
	} else {
		rt, err = docker.New(cfg.Runtime.DockerSocket)
		if err != nil {
			db.Close()
			return nil, err
		}
	}
	// db sengaja tidak di-Close: lifecycle memakainya selama proses berjalan.
	return orchestrator.New(db, rt, orchestrator.NewEventBus()), nil
}

func runServer(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("gunakan: server create|start|stop|delete")
	}
	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("create", flag.ContinueOnError)
		bindGlobal(fs)
		name := fs.String("name", "", "nama server")
		egg := fs.String("egg", "", "egg id")
		startup := fs.String("startup", "", "startup command")
		envFlag := fs.String("env", "", "env tambahan K=V,K=V")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" || *egg == "" || *startup == "" {
			return fmt.Errorf("name, egg, dan startup wajib diisi")
		}
		env := map[string]string{}
		if *envFlag != "" {
			for _, kv := range strings.Split(*envFlag, ",") {
				k, v, ok := strings.Cut(kv, "=")
				if !ok {
					return fmt.Errorf("env %q harus format K=V", kv)
				}
				env[k] = v
			}
		}
		lc, err := openLifecycle()
		if err != nil {
			return err
		}
		id, err := lc.CreateServer(context.Background(), *name, *egg, *startup, env)
		if err != nil {
			return err
		}
		fmt.Println(id)
		return nil
	case "start", "stop", "delete":
		action := args[0]
		fs := flag.NewFlagSet(action, flag.ContinueOnError)
		bindGlobal(fs)
		id := fs.String("id", "", "server id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == "" {
			return fmt.Errorf("id wajib diisi")
		}
		lc, err := openLifecycle()
		if err != nil {
			return err
		}
		ctx := context.Background()
		switch action {
		case "start":
			return lc.StartServer(ctx, *id)
		case "stop":
			return lc.StopServer(ctx, *id)
		case "delete":
			return lc.DeleteServer(ctx, *id)
		}
		return nil
	default:
		return fmt.Errorf("unknown server subcommand: %s", args[0])
	}
}

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	configPath := fs.String("config", "/etc/oikos/config.yaml", "path config yang ditulis")
	panel := fs.String("panel", "", "alamat panel (opsional, override default)")
	token := fs.String("token", "", "pairing token sekali pakai")
	name := fs.String("name", "", "nama node (default hostname)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg := config.Default()
	if *panel != "" {
		cfg.Panel.Address = *panel
	}
	if *panel != "" {
		nodeName := *name
		if nodeName == "" {
			nodeName, _ = os.Hostname()
			if nodeName == "" {
				nodeName = "oikos-node"
			}
		}
		if err := os.MkdirAll(cfg.Node.DataDir, 0750); err != nil {
			return err
		}
		db, err := store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db"))
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.Migrate(); err != nil {
			return err
		}
		id, err := agent.PairWithPanel(context.Background(), cfg.Panel.Address, *token, nodeName, cfg, db, *configPath, nil)
		if err != nil {
			return err
		}
		fmt.Println("pairing ok, node id", id)
		return nil
	}
	if err := agent.PairWithToken(*token, cfg, *configPath); err != nil {
		return err
	}
	fmt.Println("pairing ok, config ditulis ke", *configPath)
	return nil
}
