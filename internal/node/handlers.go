package node

import (
	"encoding/json"
	"log"
	"net/http"

	"bft/internal/model"
)

func (n *Node) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/start", n.handleStart)
	mux.HandleFunc("/message", n.handleMessage)
	mux.HandleFunc("/state", n.handleState)
	mux.HandleFunc("/events", n.handleEvents)
	mux.HandleFunc("/reset", n.handleReset)
}

func (n *Node) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

	if !n.Config.Leader {
		n.PrimeConsensusRound(req.Value)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(model.ResetResponse{Status: "armed"})
		return
	}

	go n.StartConsensus(req.Value)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(model.ResetResponse{Status: "started"})
}

func (n *Node) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg model.Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[%s] received %s from %s value=%s", n.Config.ID, msg.Type, msg.From, msg.Value)
	n.mu.Lock()
	n.appendEventLocked(model.EventMessageReceived, &msg, n.Config.ID, "", n.isByzantinePeer(msg.From))
	n.mu.Unlock()
	n.maybeBroadcastStaleViewSpam(msg)
	n.ProcessMessage(msg)

	w.WriteHeader(http.StatusOK)
}

func (n *Node) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(n.stateResponseLocked())
}

func (n *Node) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	events := make([]model.NodeEvent, len(n.Events))
	copy(events, n.Events)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(model.EventsResponse{
		ID:     n.Config.ID,
		Events: events,
	})
}

func (n *Node) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	n.Reset()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(model.ResetResponse{Status: "reset"})
}

func (n *Node) maybeBroadcastStaleViewSpam(trigger model.Message) {
	n.mu.Lock()
	if !n.Config.Byzantine || n.Config.Behavior != model.BehaviorStaleViewSpam {
		n.mu.Unlock()
		return
	}

	staleView := n.State.View
	if staleView > 0 {
		staleView--
	}
	staleSequence := n.State.Sequence - 1
	if staleSequence < 0 {
		staleSequence = 0
	}
	spam := model.Message{
		Type:      model.MsgPrepare,
		View:      staleView,
		Sequence:  staleSequence,
		From:      n.Config.ID,
		Value:     n.requestedValue,
		Digest:    Digest(n.requestedValue),
		Signature: "",
	}
	n.appendEventLocked(model.EventByzantineAction, &spam, "", "stale_view_spam", true)
	n.mu.Unlock()

	n.Broadcast(spam)
}
