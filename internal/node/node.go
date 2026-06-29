package node

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	"bft/internal/model"
)

type Node struct {
	Config model.NodeConfig

	mu sync.Mutex

	State      model.ConsensusState
	PrePrepare *model.Message

	PrepareMsgs map[string]model.Message
	CommitMsgs  map[string]model.Message

	RejectCount    int
	LastReject     string
	consensusStart bool
	Events         []model.NodeEvent
	eventCounter   int64
}

func NewNode(cfg model.NodeConfig) *Node {
	n := &Node{Config: cfg}
	n.resetLocked()
	return n
}

func Digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (n *Node) quorum() int {
	totalNodes := len(n.Config.Peers) + 1
	f := (totalNodes - 1) / 3
	return 2*f + 1
}

func (n *Node) matchingPrepareCountLocked() int {
	count := 0
	for _, msg := range n.PrepareMsgs {
		if msg.Value == n.State.ProposedValue {
			count++
		}
	}
	return count
}

func (n *Node) matchingCommitCountLocked() int {
	count := 0
	for _, msg := range n.CommitMsgs {
		if msg.Value == n.State.ProposedValue {
			count++
		}
	}
	return count
}

func (n *Node) recordRejectLocked(reason string) {
	n.RejectCount++
	n.LastReject = reason
}

func (n *Node) appendEventLocked(kind model.EventKind, msg *model.Message, to string, details string, malicious bool) model.NodeEvent {
	n.eventCounter++
	event := model.NodeEvent{
		ID:        fmt.Sprintf("%s-e%d", n.Config.ID, n.eventCounter),
		Timestamp: time.Now().UTC(),
		Kind:      kind,
		NodeID:    n.Config.ID,
		To:        to,
		Malicious: malicious,
		Details:   details,
	}
	if msg != nil {
		event.From = msg.From
		event.MessageType = msg.Type
		event.Value = msg.Value
	}
	n.Events = append(n.Events, event)
	return event
}

func (n *Node) phaseLocked() string {
	switch {
	case n.State.Decided:
		return "decided"
	case n.State.Committed:
		return "committed"
	case n.State.Prepared:
		return "prepared"
	case n.State.ProposedValue != "":
		return "proposed"
	default:
		return "idle"
	}
}

func (n *Node) outgoingValueLocked() string {
	acceptedValue := n.State.ProposedValue
	if acceptedValue == "" {
		return ""
	}

	if !n.Config.Byzantine {
		return acceptedValue
	}

	switch n.Config.Behavior {
	case model.BehaviorSilent:
		return ""
	case model.BehaviorConflictingValue:
		return acceptedValue + "_tampered"
	default:
		return acceptedValue
	}
}

func (n *Node) stateResponseLocked() model.StateResponse {
	return model.StateResponse{
		ID:             n.Config.ID,
		Leader:         n.Config.Leader,
		Byzantine:      n.Config.Byzantine,
		Behavior:       n.Config.Behavior,
		Running:        n.consensusStart && !n.State.Decided,
		Phase:          n.phaseLocked(),
		AcceptedValue:  n.State.ProposedValue,
		OutgoingValue:  n.outgoingValueLocked(),
		State:          n.State,
		PrepareMatches: n.matchingPrepareCountLocked(),
		CommitMatches:  n.matchingCommitCountLocked(),
		RejectCount:    n.RejectCount,
		LastReject:     n.LastReject,
	}
}

func (n *Node) Reset() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.resetLocked()
}

func (n *Node) expectedLeaderIDLocked(view int) string {
	ids := make([]string, 0, len(n.Config.Peers)+1)
	ids = append(ids, n.Config.ID)
	for _, peer := range n.Config.Peers {
		ids = append(ids, peer.ID)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return ""
	}
	return ids[view%len(ids)]
}

func (n *Node) resetLocked() {
	n.State = model.ConsensusState{
		View:     0,
		Sequence: 1,
	}
	n.PrePrepare = nil
	n.PrepareMsgs = make(map[string]model.Message)
	n.CommitMsgs = make(map[string]model.Message)
	n.RejectCount = 0
	n.LastReject = ""
	n.consensusStart = false
	n.Events = nil
	n.eventCounter = 0
}

func (n *Node) isByzantinePeer(id string) bool {
	if id == n.Config.ID {
		return n.Config.Byzantine
	}
	for _, peer := range n.Config.Peers {
		if peer.ID == id {
			return peer.Byzantine
		}
	}
	return false
}
