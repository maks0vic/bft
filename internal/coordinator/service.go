package coordinator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"bft/internal/model"
)

type Service struct {
	mu sync.Mutex

	simulationID string
	nodeConfigs  []model.NodeConfig
	httpClient   *http.Client

	eventIndex      map[string]int64
	canonicalEvents []model.CanonicalEvent
	globalSequence  int64
}

func New(nodeConfigs []model.NodeConfig) *Service {
	configs := append([]model.NodeConfig(nil), nodeConfigs...)
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].ID < configs[j].ID
	})

	return &Service{
		simulationID: nextSimulationID(),
		nodeConfigs:  configs,
		httpClient:   &http.Client{Timeout: 2 * time.Second},
		eventIndex:   make(map[string]int64),
	}
}

func (s *Service) Leader() (model.NodeConfig, bool) {
	for _, cfg := range s.nodeConfigs {
		if cfg.Leader {
			return cfg, true
		}
	}
	return model.NodeConfig{}, false
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

	leader, ok := s.Leader()
	if !ok {
		http.Error(w, "no leader configured", http.StatusInternalServerError)
		return
	}

	var req model.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Value == "" {
		http.Error(w, "value is required", http.StatusBadRequest)
		return
	}

	if err := s.resetAll(); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	body, err := json.Marshal(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := s.httpClient.Post("http://"+leader.Address+"/start", "application/json", bytes.NewBuffer(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_ = json.NewEncoder(w).Encode(model.ResetResponse{Status: "started"})
}

func (s *Service) HandleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.resetAll(); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(model.ResetResponse{Status: "reset"})
}

func (s *Service) fetchStates() ([]model.StateResponse, error) {
	states := make([]model.StateResponse, 0, len(s.nodeConfigs))
	for _, cfg := range s.nodeConfigs {
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
	eventsByNode := make(map[string][]model.NodeEvent, len(s.nodeConfigs))
	for _, cfg := range s.nodeConfigs {
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
	consensusReached := true
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
			ProposedValue: state.State.ProposedValue,
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

func (s *Service) resetAll() error {
	for _, cfg := range s.nodeConfigs {
		req, err := http.NewRequest(http.MethodPost, "http://"+cfg.Address+"/reset", nil)
		if err != nil {
			return err
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("unexpected reset status from %s: %s", cfg.Address, resp.Status)
		}
	}

	s.mu.Lock()
	s.eventIndex = make(map[string]int64)
	s.canonicalEvents = nil
	s.globalSequence = 0
	s.simulationID = nextSimulationID()
	s.mu.Unlock()

	return nil
}

func nextSimulationID() string {
	return fmt.Sprintf("sim-%d", time.Now().UnixNano())
}
