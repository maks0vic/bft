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
	"syscall"
	"time"

	"bft/internal/model"
)

var launcherHTTPClient = &http.Client{Timeout: 2 * time.Second}

func main() {
	coordinatorAddr := flag.String("coordinator-addr", "localhost:9000", "coordinator listen address")
	nodeBasePort := flag.Int("node-base-port", 8001, "starting port for generated node processes")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	processCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var procs []*exec.Cmd
	coordinatorCmd := exec.CommandContext(processCtx, "go", "run", "./cmd/coordinator", "-addr", *coordinatorAddr, "-node-base-port", fmt.Sprintf("%d", *nodeBasePort))
	coordinatorCmd.Dir = repoRoot()
	startCmd("coordinator", coordinatorCmd, &procs)

	defer func() {
		cancel()
		for _, cmd := range procs {
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
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

func startCmd(prefix string, cmd *exec.Cmd, procs *[]*exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
