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

const (
	proposalTimeoutWindow = 2 * time.Second
	progressTimeoutWindow = 2 * time.Second
)

type acceptedProposalCertificate struct {
	Message model.Message
}

type quorumCertificate struct {
	Message    model.Message
	Senders    []string
	QuorumSize int
}

type proposalState struct {
	View     int
	Sequence int
	Value    string
	Digest   string
}

type Node struct {
	Config model.NodeConfig

	mu sync.Mutex

	State model.ConsensusState

	AcceptedCertificate  *acceptedProposalCertificate
	PreparedCertificate  *quorumCertificate
	CommittedCertificate *quorumCertificate
	CandidateProposal    *proposalState
	PreparedProposal     *proposalState
	DecidedProposal      *proposalState

	PrepareEvidence map[string]map[string]model.Message
	CommitEvidence  map[string]map[string]model.Message

	RejectCount    int
	LastReject     string
	consensusStart bool
	Events         []model.NodeEvent
	eventCounter   int64
	timeoutEpoch   int64
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

func evidenceBucketKey(view int, sequence int, digest string) string {
	return fmt.Sprintf("%d:%d:%s", view, sequence, digest)
}

func (n *Node) candidateDigestLocked() string {
	if n.CandidateProposal != nil {
		return n.CandidateProposal.Digest
	}
	return ""
}

func (n *Node) preparedDigestLocked() string {
	if n.PreparedProposal != nil {
		return n.PreparedProposal.Digest
	}
	return ""
}

func (n *Node) decidedValueLocked() string {
	if n.DecidedProposal != nil {
		return n.DecidedProposal.Value
	}
	return ""
}

func (n *Node) evidenceCountLocked(evidence map[string]map[string]model.Message, view int, sequence int, digest string) int {
	if digest == "" {
		return 0
	}
	bucket := evidence[evidenceBucketKey(view, sequence, digest)]
	return len(bucket)
}

func (n *Node) storeEvidenceLocked(evidence map[string]map[string]model.Message, msg model.Message) int {
	key := evidenceBucketKey(msg.View, msg.Sequence, msg.Digest)
	if evidence[key] == nil {
		evidence[key] = make(map[string]model.Message)
	}
	evidence[key][msg.From] = msg
	return len(evidence[key])
}

func (n *Node) matchingPrepareCountLocked() int {
	return n.evidenceCountLocked(n.PrepareEvidence, n.State.View, n.State.Sequence, n.candidateDigestLocked())
}

func (n *Node) matchingCommitCountLocked() int {
	digest := n.preparedDigestLocked()
	if digest == "" {
		digest = n.candidateDigestLocked()
	}
	return n.evidenceCountLocked(n.CommitEvidence, n.State.View, n.State.Sequence, digest)
}

func (n *Node) buildQuorumCertificateLocked(evidence map[string]map[string]model.Message, view int, sequence int, digest string) *quorumCertificate {
	key := evidenceBucketKey(view, sequence, digest)
	bucket := evidence[key]
	if len(bucket) < n.quorum() {
		return nil
	}

	senders := make([]string, 0, len(bucket))
	for sender := range bucket {
		senders = append(senders, sender)
	}
	sort.Strings(senders)

	message := bucket[senders[0]]
	return &quorumCertificate{
		Message:    message,
		Senders:    senders,
		QuorumSize: len(senders),
	}
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
	case n.State.TimedOut:
		return "timed_out"
	case n.State.Committed:
		return "committed"
	case n.State.Prepared:
		return "prepared"
	case n.CandidateProposal != nil:
		return "proposed"
	default:
		return "idle"
	}
}

func (n *Node) beginRoundLocked() {
	n.consensusStart = true
	n.State.TimedOut = false
	n.State.TimeoutReason = ""
	n.timeoutEpoch++
}

func (n *Node) PrimeConsensusRound() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.beginRoundLocked()
	n.scheduleProposalTimeoutLocked()
}

func (n *Node) noteProposalAcceptedLocked() {
	n.State.TimedOut = false
	n.State.TimeoutReason = ""
	n.scheduleProgressTimeoutLocked("waiting_for_prepare_quorum")
}

func (n *Node) notePreparedLocked() {
	n.State.TimedOut = false
	n.State.TimeoutReason = ""
	n.scheduleProgressTimeoutLocked("waiting_for_commit_quorum")
}

func (n *Node) finishConsensusLocked() {
	n.consensusStart = false
	n.timeoutEpoch++
}

func (n *Node) scheduleProposalTimeoutLocked() {
	epoch := n.timeoutEpoch
	view := n.State.View
	sequence := n.State.Sequence
	go n.fireTimeoutAfter(proposalTimeoutWindow, epoch, view, sequence, "waiting_for_preprepare", func() bool {
		n.mu.Lock()
		defer n.mu.Unlock()
		return n.consensusStart && !n.State.Decided && n.CandidateProposal == nil && n.State.View == view && n.State.Sequence == sequence
	})
}

func (n *Node) scheduleProgressTimeoutLocked(reason string) {
	epoch := n.timeoutEpoch
	view := n.State.View
	sequence := n.State.Sequence
	go n.fireTimeoutAfter(progressTimeoutWindow, epoch, view, sequence, reason, func() bool {
		n.mu.Lock()
		defer n.mu.Unlock()
		if !n.consensusStart || n.State.Decided || n.State.View != view || n.State.Sequence != sequence {
			return false
		}
		switch reason {
		case "waiting_for_prepare_quorum":
			return n.PreparedCertificate == nil
		case "waiting_for_commit_quorum":
			return n.PreparedCertificate != nil && n.CommittedCertificate == nil
		default:
			return false
		}
	})
}

func (n *Node) fireTimeoutAfter(delay time.Duration, epoch int64, view int, sequence int, reason string, shouldTrigger func() bool) {
	time.Sleep(delay)
	if !shouldTrigger() {
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if epoch != n.timeoutEpoch || !n.consensusStart || n.State.Decided || n.State.View != view || n.State.Sequence != sequence {
		return
	}
	n.State.TimedOut = true
	n.State.TimeoutReason = reason
	n.appendEventLocked(model.EventTimeout, nil, "", reason, n.Config.Byzantine)
}

func (n *Node) outgoingValueLocked() string {
	acceptedValue := ""
	if n.CandidateProposal != nil {
		acceptedValue = n.CandidateProposal.Value
	}
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
	acceptedValue := ""
	if n.CandidateProposal != nil {
		acceptedValue = n.CandidateProposal.Value
	}
	n.State.Decision = n.decidedValueLocked()

	return model.StateResponse{
		ID:             n.Config.ID,
		Leader:         n.Config.Leader,
		Byzantine:      n.Config.Byzantine,
		Behavior:       n.Config.Behavior,
		Running:        n.consensusStart && !n.State.Decided,
		Phase:          n.phaseLocked(),
		AcceptedValue:  acceptedValue,
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
	n.AcceptedCertificate = nil
	n.PreparedCertificate = nil
	n.CommittedCertificate = nil
	n.CandidateProposal = nil
	n.PreparedProposal = nil
	n.DecidedProposal = nil
	n.PrepareEvidence = make(map[string]map[string]model.Message)
	n.CommitEvidence = make(map[string]map[string]model.Message)
	n.RejectCount = 0
	n.LastReject = ""
	n.consensusStart = false
	n.Events = nil
	n.eventCounter = 0
	n.timeoutEpoch++
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
