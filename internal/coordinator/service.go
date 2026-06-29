package coordinator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"time"

	"bft/internal/config"
	"bft/internal/model"
)

type activeRun struct {
	configs []model.NodeConfig
	procs   []*exec.Cmd
	tempDir string
}

type Service struct {
	mu sync.Mutex

	simulationID string
	httpClient   *http.Client
	repoRoot     string
	nodeBasePort int
	processCtx   context.Context

	eventIndex      map[string]int64
	canonicalEvents []model.CanonicalEvent
	globalSequence  int64
	run             *activeRun
}

const reclaimPortWindow = 64

func New(repoRoot string, nodeBasePort int, processCtx context.Context) *Service {
	if processCtx == nil {
		processCtx = context.Background()
	}

	return &Service{
		simulationID: nextSimulationID(),
		httpClient:   &http.Client{Timeout: 2 * time.Second},
		repoRoot:     repoRoot,
		nodeBasePort: nodeBasePort,
		processCtx:   processCtx,
		eventIndex:   make(map[string]int64),
	}
}

func (s *Service) HandleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	states, err := s.fetchStates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.buildSimulationState(states))
}

func (s *Service) HandleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	since := int64(0)
	if raw := r.URL.Query().Get("since"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			http.Error(w, "invalid since value", http.StatusBadRequest)
			return
		}
		since = value
	}

	eventsByNode, err := s.fetchEvents()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	canonical, last := s.ingest(eventsByNode)
	if since > last {
		since = 0
	}
	filtered := make([]model.CanonicalEvent, 0, len(canonical))
	for _, event := range canonical {
		if event.GlobalSequence > since {
			filtered = append(filtered, event)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(model.CoordinatorEventsResponse{
		Events:       filtered,
		EventsByNode: eventsByNode,
		LastSequence: last,
	})
}

func (s *Service) HandleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req model.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cluster, err := config.BuildRuntimeCluster(req, s.nodeBasePort)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.stopActiveRun(); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := s.reclaimNodePortWindow(); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.resetCoordinatorState()

	run, err := s.startRun(cluster.Nodes)
	if err != nil {
		_ = s.cleanupRun(run)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.setRun(run)

	leader, ok := leaderOf(run.configs)
	if !ok {
		_ = s.stopActiveRun()
		http.Error(w, "no leader configured", http.StatusInternalServerError)
		return
	}

	body, err := json.Marshal(req)
	if err != nil {
		_ = s.stopActiveRun()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, cfg := range run.configs {
		if cfg.Leader {
			continue
		}
		resp, err := s.httpClient.Post("http://"+cfg.Address+"/start", "application/json", bytes.NewBuffer(body))
		if err != nil {
			_ = s.stopActiveRun()
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 {
			_ = s.stopActiveRun()
			http.Error(w, fmt.Sprintf("replica arm failed: %s", resp.Status), http.StatusBadGateway)
			return
		}
	}

	resp, err := s.httpClient.Post("http://"+leader.Address+"/start", "application/json", bytes.NewBuffer(body))
	if err != nil {
		_ = s.stopActiveRun()
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		_ = s.stopActiveRun()
		http.Error(w, fmt.Sprintf("leader start failed: %s", resp.Status), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(model.ResetResponse{Status: "started"})
}

func (s *Service) HandleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.stopActiveRun(); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := s.reclaimNodePortWindow(); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.resetCoordinatorState()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(model.ResetResponse{Status: "reset"})
}

func (s *Service) Close() error {
	if err := s.stopActiveRun(); err != nil {
		return err
	}
	return s.reclaimNodePortWindow()
}

func (s *Service) currentConfigs() []model.NodeConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.run == nil {
		return nil
	}

	configs := append([]model.NodeConfig(nil), s.run.configs...)
	return configs
}

func (s *Service) fetchStates() ([]model.StateResponse, error) {
	configs := s.currentConfigs()
	if len(configs) == 0 {
		return []model.StateResponse{}, nil
	}

	states := make([]model.StateResponse, 0, len(configs))
	for _, cfg := range configs {
		resp, err := s.httpClient.Get("http://" + cfg.Address + "/state")
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("unexpected status from %s: %s", cfg.Address, resp.Status)
		}
		var state model.StateResponse
		err = json.NewDecoder(resp.Body).Decode(&state)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool { return states[i].ID < states[j].ID })
	return states, nil
}

func (s *Service) fetchEvents() (map[string][]model.NodeEvent, error) {
	configs := s.currentConfigs()
	if len(configs) == 0 {
		return map[string][]model.NodeEvent{}, nil
	}

	eventsByNode := make(map[string][]model.NodeEvent, len(configs))
	for _, cfg := range configs {
		resp, err := s.httpClient.Get("http://" + cfg.Address + "/events")
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("unexpected status from %s: %s", cfg.Address, resp.Status)
		}
		var events model.EventsResponse
		err = json.NewDecoder(resp.Body).Decode(&events)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		eventsByNode[cfg.ID] = events.Events
	}
	return eventsByNode, nil
}

func (s *Service) ingest(eventsByNode map[string][]model.NodeEvent) ([]model.CanonicalEvent, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nodeIDs := make([]string, 0, len(eventsByNode))
	for nodeID := range eventsByNode {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Strings(nodeIDs)

	for _, nodeID := range nodeIDs {
		events := eventsByNode[nodeID]
		for _, event := range events {
			key := nodeID + ":" + event.ID
			if _, exists := s.eventIndex[key]; exists {
				continue
			}
			s.globalSequence++
			canonical := model.CanonicalEvent{
				ID:             event.ID,
				GlobalSequence: s.globalSequence,
				Timestamp:      event.Timestamp,
				Kind:           event.Kind,
				From:           event.From,
				To:             event.To,
				NodeID:         event.NodeID,
				MessageType:    event.MessageType,
				Value:          event.Value,
				Malicious:      event.Malicious,
				Details:        event.Details,
			}
			s.eventIndex[key] = canonical.GlobalSequence
			s.canonicalEvents = append(s.canonicalEvents, canonical)
		}
	}

	out := make([]model.CanonicalEvent, len(s.canonicalEvents))
	copy(out, s.canonicalEvents)
	return out, s.globalSequence
}

func (s *Service) buildSimulationState(states []model.StateResponse) model.SimulationState {
	quorum := 0
	if len(states) > 0 {
		f := (len(states) - 1) / 3
		quorum = 2*f + 1
	}

	nodes := make([]model.NodeView, 0, len(states))
	finalValue := ""
	consensusReached := len(states) > 0
	running := false
	view := 0
	sequence := 0

	for _, state := range states {
		nodes = append(nodes, model.NodeView{
			ID:            state.ID,
			Leader:        state.Leader,
			Byzantine:     state.Byzantine,
			Behavior:      state.Behavior,
			Phase:         state.Phase,
			AcceptedValue: state.AcceptedValue,
			OutgoingValue: state.OutgoingValue,
			Decision:      state.State.Decision,
			PrepareCount:  state.PrepareMatches,
			CommitCount:   state.CommitMatches,
		})
		if state.Running {
			running = true
		}
		if state.State.View > view {
			view = state.State.View
		}
		if state.State.Sequence > sequence {
			sequence = state.State.Sequence
		}
		if state.Byzantine {
			continue
		}
		if !state.State.Decided || state.State.Decision == "" {
			consensusReached = false
			continue
		}
		if finalValue == "" {
			finalValue = state.State.Decision
			continue
		}
		if finalValue != state.State.Decision {
			consensusReached = false
		}
	}
	if finalValue == "" {
		consensusReached = false
	}

	s.mu.Lock()
	simID := s.simulationID
	s.mu.Unlock()

	return model.SimulationState{
		SimulationID:     simID,
		Quorum:           quorum,
		ConsensusReached: consensusReached,
		FinalValue:       finalValue,
		Running:          running,
		View:             view,
		Sequence:         sequence,
		Nodes:            nodes,
	}
}

func (s *Service) startRun(configs []model.NodeConfig) (*activeRun, error) {
	tempDir, err := os.MkdirTemp("", "bft-runtime-*")
	if err != nil {
		return nil, err
	}

	run := &activeRun{
		configs: append([]model.NodeConfig(nil), configs...),
		tempDir: tempDir,
	}

	for _, cfg := range configs {
		if err := waitForPortReleased(cfg.Address, 2*time.Second); err != nil {
			return run, err
		}

		path := filepath.Join(tempDir, fmt.Sprintf("%s.json", cfg.ID))
		if err := writeConfig(path, cfg); err != nil {
			return run, err
		}

		cmd := exec.CommandContext(s.processCtx, "go", "run", "./cmd/node", "-config", path)
		cmd.Dir = s.repoRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			return run, err
		}
		run.procs = append(run.procs, cmd)
	}

	if err := s.waitForNodes(run.configs, 8*time.Second); err != nil {
		return run, err
	}

	return run, nil
}

func (s *Service) waitForNodes(configs []model.NodeConfig, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ready := true
		for _, cfg := range configs {
			resp, err := s.httpClient.Get("http://" + cfg.Address + "/state")
			if err != nil {
				ready = false
				break
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				ready = false
				break
			}
		}
		if ready {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("node readiness timeout after %s", timeout)
}

func (s *Service) stopActiveRun() error {
	s.mu.Lock()
	run := s.run
	s.run = nil
	s.mu.Unlock()

	if run == nil {
		return nil
	}

	return s.cleanupRun(run)
}

func (s *Service) cleanupRun(run *activeRun) error {
	var firstErr error
	for _, cmd := range run.procs {
		if cmd == nil || cmd.Process == nil {
			continue
		}
		if err := signalProcessGroup(cmd, syscall.SIGTERM); err != nil && !isFinishedProcessError(err) {
			firstErr = pickErr(firstErr, err)
		}
	}

	for i, cmd := range run.procs {
		if cmd == nil {
			continue
		}
		address := ""
		if i < len(run.configs) {
			address = run.configs[i].Address
		}
		if err := waitCmdExit(cmd, 2*time.Second); err != nil {
			if killErr := signalProcessGroup(cmd, syscall.SIGKILL); killErr != nil && !isFinishedProcessError(killErr) {
				firstErr = pickErr(firstErr, killErr)
			}
			if killErr := waitCmdExit(cmd, 2*time.Second); killErr != nil {
				firstErr = pickErr(firstErr, killErr)
			}
		}
		if address != "" {
			if err := waitForPortReleased(address, 2*time.Second); err != nil {
				firstErr = pickErr(firstErr, err)
			}
		}
	}

	if run.tempDir != "" {
		if err := os.RemoveAll(run.tempDir); err != nil {
			firstErr = pickErr(firstErr, err)
		}
	}

	return firstErr
}

func (s *Service) resetCoordinatorState() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.eventIndex = make(map[string]int64)
	s.canonicalEvents = nil
	s.globalSequence = 0
	s.simulationID = nextSimulationID()
}

func (s *Service) setRun(run *activeRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.run = run
}

func leaderOf(configs []model.NodeConfig) (model.NodeConfig, bool) {
	for _, cfg := range configs {
		if cfg.Leader {
			return cfg, true
		}
	}
	return model.NodeConfig{}, false
}

func writeConfig(path string, cfg model.NodeConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func waitCmdExit(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil && !isFinishedProcessError(err) {
			return err
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("process exit timeout after %s", timeout)
	}
}

func signalProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, sig)
}

func (s *Service) reclaimNodePortWindow() error {
	var firstErr error
	for port := s.nodeBasePort; port < s.nodeBasePort+reclaimPortWindow; port++ {
		if err := reclaimNodeListenerOnPort(port); err != nil {
			firstErr = pickErr(firstErr, err)
		}
	}
	return firstErr
}

func reclaimNodeListenerOnPort(port int) error {
	cmd := exec.Command("lsof", "-nP", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-Fpc")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if ok := errorAs(err, &exitErr); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	currentPID := 0
	currentCmd := ""
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			pid, parseErr := strconv.Atoi(line[1:])
			if parseErr != nil {
				currentPID = 0
				continue
			}
			currentPID = pid
		case 'c':
			currentCmd = line[1:]
			if currentPID != 0 && currentCmd == "node" {
				if err := syscall.Kill(currentPID, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
					return err
				}
			}
		}
	}

	address := fmt.Sprintf("localhost:%d", port)
	return waitForPortReleased(address, 2*time.Second)
}

func errorAs(err error, target interface{}) bool {
	switch v := target.(type) {
	case **exec.ExitError:
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			*v = exitErr
		}
		return ok
	default:
		return false
	}
}

func waitForPortReleased(address string, timeout time.Duration) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 200*time.Millisecond)
		if err != nil {
			return nil
		}
		_ = conn.Close()
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("port %s was not released within %s", address, timeout)
}

func isFinishedProcessError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*exec.ExitError); ok {
		return true
	}
	return err.Error() == "os: process already finished" || err.Error() == "waitid: no child processes"
}

func pickErr(current error, next error) error {
	if current != nil {
		return current
	}
	return next
}

func nextSimulationID() string {
	return fmt.Sprintf("sim-%d", time.Now().UnixNano())
}
