package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"bft/internal/config"
	"bft/internal/model"
)

var launcherHTTPClient = &http.Client{Timeout: 2 * time.Second}

func main() {
	configDir := flag.String("config-dir", "configs", "directory with node configs")
	coordinatorAddr := flag.String("coordinator-addr", "localhost:9000", "coordinator listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	nodes, err := loadConfigItems(*configDir)
	if err != nil {
		log.Fatalf("load configs: %v", err)
	}

	processCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var procs []*exec.Cmd
	for _, item := range nodes {
		cmd := exec.CommandContext(processCtx, "go", "run", "./cmd/node", "-config", item.Path)
		cmd.Dir = repoRoot()
		startCmd(item.Config.ID, cmd, &procs)
	}

	coordinatorCmd := exec.CommandContext(processCtx, "go", "run", "./cmd/coordinator", "-config-dir", *configDir, "-addr", *coordinatorAddr)
	coordinatorCmd.Dir = repoRoot()
	startCmd("coordinator", coordinatorCmd, &procs)

	defer func() {
		cancel()
		for _, cmd := range procs {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = cmd.Wait()
		}
	}()

	if err := waitForCoordinator(*coordinatorAddr, 8*time.Second); err != nil {
		log.Fatalf("coordinator did not become ready: %v", err)
	}

	state, err := fetchState(*coordinatorAddr)
	if err != nil {
		log.Fatalf("fetch initial coordinator state: %v", err)
	}

	log.Printf("[launcher] stack is ready: coordinator=http://%s simulation=%s running=%t", *coordinatorAddr, state.SimulationID, state.Running)
	log.Printf("[launcher] waiting for coordinator POST /start")

	<-ctx.Done()
}

type configItem struct {
	Path   string
	Config model.NodeConfig
}

func loadConfigItems(dir string) ([]configItem, error) {
	pattern := filepath.Join(dir, "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no config files found in %s", dir)
	}

	items := make([]configItem, 0, len(paths))
	for _, path := range paths {
		cfg, err := config.Load(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
		items = append(items, configItem{Path: path, Config: cfg})
	}
	return items, nil
}

func startCmd(prefix string, cmd *exec.Cmd, procs *[]*exec.Cmd) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("stdout pipe for %s: %v", prefix, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("stderr pipe for %s: %v", prefix, err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatalf("start %s: %v", prefix, err)
	}
	*procs = append(*procs, cmd)
	go streamWithPrefix(prefix, stdout)
	go streamWithPrefix(prefix, stderr)
}

func waitForCoordinator(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := fetchState(address)
		if err == nil {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("readiness timeout after %s", timeout)
}

func fetchState(address string) (model.SimulationState, error) {
	resp, err := launcherHTTPClient.Get("http://" + address + "/state")
	if err != nil {
		return model.SimulationState{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return model.SimulationState{}, fmt.Errorf("unexpected status from %s: %s", address, resp.Status)
	}

	var state model.SimulationState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return model.SimulationState{}, err
	}
	return state, nil
}

func streamWithPrefix(prefix string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fmt.Printf("[%s] %s\n", prefix, scanner.Text())
	}
}

func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
