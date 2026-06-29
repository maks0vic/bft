package node

import (
	"fmt"
	"log"

	"bft/internal/model"
)

func proposalFromMessage(msg model.Message) *proposalState {
	return &proposalState{
		View:     msg.View,
		Sequence: msg.Sequence,
		Value:    msg.Value,
		Digest:   msg.Digest,
	}
}

func (n *Node) StartConsensus(value string) {
	n.mu.Lock()
	if n.consensusStart {
		n.mu.Unlock()
		log.Printf("[%s] consensus already started", n.Config.ID)
		return
	}
	n.requestedValue = value
	n.beginRoundLocked()

	msg := model.Message{
		Type:     model.MsgPrePrepare,
		View:     0,
		Sequence: 1,
		From:     n.Config.ID,
		Value:    value,
		Digest:   Digest(value),
	}
	n.appendEventLocked(model.EventConsensusStarted, &msg, "", "leader_start", false)
	n.mu.Unlock()

	log.Printf("[%s] leader starting consensus with value=%s", n.Config.ID, value)
	n.Broadcast(msg)
	n.ProcessMessage(msg)
}

func (n *Node) ProcessMessage(msg model.Message) {
	switch msg.Type {
	case model.MsgPrePrepare:
		n.handlePrePrepare(msg)
	case model.MsgPrepare:
		n.handlePrepare(msg)
	case model.MsgCommit:
		n.handleCommit(msg)
	case model.MsgViewChange:
		n.handleViewChange(msg)
	case model.MsgNewView:
		n.handleNewView(msg)
	default:
		n.mu.Lock()
		n.recordRejectLocked("unknown message type")
		n.appendEventLocked(model.EventRejected, &msg, "", "unknown message type", false)
		n.mu.Unlock()
		log.Printf("[%s] rejected unknown message type from %s", n.Config.ID, msg.From)
	}
}

func (n *Node) handlePrePrepare(msg model.Message) {
	n.mu.Lock()
	if n.shouldStaySilentLocked() {
		n.appendEventLocked(model.EventByzantineAction, &msg, "", "silent_byzantine", true)
		n.mu.Unlock()
		log.Printf("[%s] Byzantine silent node ignoring PRE_PREPARE", n.Config.ID)
		return
	}
	if !n.validatePrePrepareLeaderLocked(msg) {
		n.mu.Unlock()
		return
	}
	if !n.validateBasicLocked(msg) {
		n.mu.Unlock()
		return
	}
	if existing := n.AcceptedCertificate; existing != nil && (existing.Message.View != msg.View || existing.Message.Sequence != msg.Sequence || existing.Message.Digest != msg.Digest) {
		n.recordRejectLocked("conflicting pre-prepare")
		n.appendEventLocked(model.EventRejected, &msg, "", "conflicting pre-prepare", false)
		n.mu.Unlock()
		log.Printf("[%s] rejected conflicting PRE_PREPARE from %s", n.Config.ID, msg.From)
		return
	}

	n.AcceptedCertificate = &acceptedProposalCertificate{Message: msg}
	n.CandidateProposal = proposalFromMessage(msg)
	n.State.ProposedValue = n.CandidateProposal.Value
	n.noteProposalAcceptedLocked()

	prepareValue := msg.Value
	if n.Config.Byzantine && n.Config.Behavior == model.BehaviorConflictingValue {
		prepareValue = msg.Value + "_tampered"
	}

	prepare := model.Message{
		Type:     model.MsgPrepare,
		View:     msg.View,
		Sequence: msg.Sequence,
		From:     n.Config.ID,
		Value:    prepareValue,
		Digest:   Digest(prepareValue),
	}
	matchCount := n.storeEvidenceLocked(n.PrepareEvidence, prepare)
	shouldCommit := false
	var commit model.Message
	if n.PreparedCertificate == nil && n.AcceptedCertificate != nil && matchCount >= n.quorum() {
		n.PreparedCertificate = n.buildQuorumCertificateLocked(n.PrepareEvidence, prepare.View, prepare.Sequence, prepare.Digest)
		n.State.Prepared = n.PreparedCertificate != nil
		n.appendEventLocked(model.EventQuorumReached, &prepare, "", fmt.Sprintf("prepare quorum reached certificate digest=%s quorum=%d", prepare.Digest, matchCount), false)
		n.appendEventLocked(model.EventNodePrepared, &prepare, "", fmt.Sprintf("prepared certificate formed digest=%s quorum=%d", prepare.Digest, matchCount), false)
		if n.PreparedCertificate != nil && n.CandidateProposal != nil {
			n.PreparedProposal = &proposalState{
				View:     n.CandidateProposal.View,
				Sequence: n.CandidateProposal.Sequence,
				Value:    n.CandidateProposal.Value,
				Digest:   n.CandidateProposal.Digest,
			}
			n.notePreparedLocked()
		}
		commitValue := n.PreparedProposal.Value
		if n.Config.Byzantine && n.Config.Behavior == model.BehaviorConflictingValue {
			commitValue = n.PreparedProposal.Value + "_tampered"
		}
		commit = model.Message{
			Type:     model.MsgCommit,
			View:     msg.View,
			Sequence: msg.Sequence,
			From:     n.Config.ID,
			Value:    commitValue,
			Digest:   Digest(commitValue),
		}
		n.storeEvidenceLocked(n.CommitEvidence, commit)
		shouldCommit = true
	}
	n.mu.Unlock()

	log.Printf("[%s] broadcasting PREPARE value=%s", n.Config.ID, prepareValue)
	n.BroadcastPossiblyConflicting(prepare)
	if shouldCommit {
		log.Printf("[%s] prepared with %d matching PREPARE messages, broadcasting COMMIT", n.Config.ID, matchCount)
		n.BroadcastPossiblyConflicting(commit)
	}
}

func (n *Node) handlePrepare(msg model.Message) {
	n.mu.Lock()
	if n.shouldStaySilentLocked() {
		n.appendEventLocked(model.EventByzantineAction, &msg, "", "silent_byzantine", true)
		n.mu.Unlock()
		log.Printf("[%s] Byzantine silent node ignoring PREPARE", n.Config.ID)
		return
	}
	if !n.validateBasicLocked(msg) {
		n.mu.Unlock()
		return
	}
	if n.CandidateProposal == nil {
		n.storeEvidenceLocked(n.PrepareEvidence, msg)
		n.appendEventLocked(model.EventBuffered, &msg, "", "waiting_for_preprepare", false)
		n.mu.Unlock()
		log.Printf("[%s] buffering PREPARE from %s until PRE_PREPARE arrives", n.Config.ID, msg.From)
		return
	}
	if msg.View != n.CandidateProposal.View || msg.Sequence != n.CandidateProposal.Sequence || msg.Digest != n.CandidateProposal.Digest || msg.Value != n.CandidateProposal.Value {
		n.recordRejectLocked("prepare value mismatch")
		n.appendEventLocked(model.EventRejected, &msg, "", "prepare value mismatch", false)
		n.mu.Unlock()
		log.Printf("[%s] rejected PREPARE value mismatch from %s", n.Config.ID, msg.From)
		return
	}

	matchCount := n.storeEvidenceLocked(n.PrepareEvidence, msg)
	if n.PreparedCertificate != nil || matchCount < n.quorum() || n.AcceptedCertificate == nil {
		n.mu.Unlock()
		return
	}

	n.PreparedCertificate = n.buildQuorumCertificateLocked(n.PrepareEvidence, msg.View, msg.Sequence, msg.Digest)
	n.State.Prepared = n.PreparedCertificate != nil
	n.appendEventLocked(model.EventQuorumReached, &msg, "", fmt.Sprintf("prepare quorum reached certificate digest=%s quorum=%d", msg.Digest, matchCount), false)
	n.appendEventLocked(model.EventNodePrepared, &msg, "", fmt.Sprintf("prepared certificate formed digest=%s quorum=%d", msg.Digest, matchCount), false)
	if n.PreparedCertificate != nil && n.CandidateProposal != nil {
		n.PreparedProposal = &proposalState{
			View:     n.CandidateProposal.View,
			Sequence: n.CandidateProposal.Sequence,
			Value:    n.CandidateProposal.Value,
			Digest:   n.CandidateProposal.Digest,
		}
		n.notePreparedLocked()
	}

	commitValue := n.PreparedProposal.Value
	if n.Config.Byzantine && n.Config.Behavior == model.BehaviorConflictingValue {
		commitValue = n.PreparedProposal.Value + "_tampered"
	}

	commit := model.Message{
		Type:     model.MsgCommit,
		View:     msg.View,
		Sequence: msg.Sequence,
		From:     n.Config.ID,
		Value:    commitValue,
		Digest:   Digest(commitValue),
	}
	n.storeEvidenceLocked(n.CommitEvidence, commit)
	n.mu.Unlock()

	log.Printf("[%s] prepared with %d matching PREPARE messages, broadcasting COMMIT", n.Config.ID, matchCount)
	n.BroadcastPossiblyConflicting(commit)
}

func (n *Node) handleCommit(msg model.Message) {
	n.mu.Lock()
	if n.shouldStaySilentLocked() {
		n.appendEventLocked(model.EventByzantineAction, &msg, "", "silent_byzantine", true)
		n.mu.Unlock()
		log.Printf("[%s] Byzantine silent node ignoring COMMIT", n.Config.ID)
		return
	}
	if !n.validateBasicLocked(msg) {
		n.mu.Unlock()
		return
	}
	if n.PreparedProposal == nil {
		n.storeEvidenceLocked(n.CommitEvidence, msg)
		n.appendEventLocked(model.EventBuffered, &msg, "", "waiting_for_prepared_proposal", false)
		n.mu.Unlock()
		log.Printf("[%s] buffering COMMIT from %s until prepared proposal exists", n.Config.ID, msg.From)
		return
	}
	if msg.View != n.PreparedProposal.View || msg.Sequence != n.PreparedProposal.Sequence || msg.Digest != n.PreparedProposal.Digest || msg.Value != n.PreparedProposal.Value {
		n.recordRejectLocked("commit value mismatch")
		n.appendEventLocked(model.EventRejected, &msg, "", "commit value mismatch", false)
		n.mu.Unlock()
		log.Printf("[%s] rejected COMMIT value mismatch from %s", n.Config.ID, msg.From)
		return
	}

	matchCount := n.storeEvidenceLocked(n.CommitEvidence, msg)
	if n.State.Decided || matchCount < n.quorum() || n.PreparedCertificate == nil {
		if n.PreparedCertificate == nil {
			n.appendEventLocked(model.EventBuffered, &msg, "", "waiting_for_prepared_certificate", false)
		}
		n.mu.Unlock()
		return
	}

	n.CommittedCertificate = n.buildQuorumCertificateLocked(n.CommitEvidence, msg.View, msg.Sequence, msg.Digest)
	if n.CommittedCertificate == nil {
		n.mu.Unlock()
		return
	}

	n.DecidedProposal = &proposalState{
		View:     n.PreparedProposal.View,
		Sequence: n.PreparedProposal.Sequence,
		Value:    n.PreparedProposal.Value,
		Digest:   n.PreparedProposal.Digest,
	}
	n.State.Committed = true
	n.appendEventLocked(model.EventNodeCommitted, &msg, "", fmt.Sprintf("committed certificate formed digest=%s quorum=%d", msg.Digest, matchCount), false)
	n.appendEventLocked(model.EventQuorumReached, &msg, "", fmt.Sprintf("commit quorum reached certificate digest=%s quorum=%d", msg.Digest, matchCount), false)
	n.State.Decided = true
	n.State.Decision = n.DecidedProposal.Value
	n.finishConsensusLocked()
	decision := n.State.Decision
	n.appendEventLocked(model.EventNodeDecided, &msg, "", "decision="+decision, false)
	n.mu.Unlock()

	log.Printf("[%s] DECIDED value=%s with %d matching COMMIT messages", n.Config.ID, decision, matchCount)
}

func (n *Node) InitiateViewChange(targetView int, trigger string) {
	n.mu.Lock()
	if n.State.Decided || targetView <= n.State.View || targetView <= n.ViewChangeTarget {
		n.mu.Unlock()
		return
	}

	n.ViewChangeTarget = targetView
	n.State.TimedOut = false
	n.State.TimeoutReason = ""
	n.consensusStart = true
	n.scheduleProgressTimeoutLocked("waiting_for_new_view")

	viewChange := model.Message{
		Type:         model.MsgViewChange,
		View:         targetView,
		Sequence:     n.State.Sequence,
		From:         n.Config.ID,
		PreparedView: n.currentPreparedViewLocked(),
	}
	if n.PreparedProposal != nil {
		viewChange.Value = n.PreparedProposal.Value
		viewChange.Digest = n.PreparedProposal.Digest
	}
	if n.ViewChangeEvidence[targetView] == nil {
		n.ViewChangeEvidence[targetView] = make(map[string]model.Message)
	}
	n.ViewChangeEvidence[targetView][n.Config.ID] = viewChange
	n.mu.Unlock()

	log.Printf("[%s] starting view change to view=%d trigger=%s", n.Config.ID, targetView, trigger)
	n.Broadcast(viewChange)
}

func (n *Node) handleViewChange(msg model.Message) {
	n.mu.Lock()
	if n.shouldStaySilentLocked() {
		n.appendEventLocked(model.EventByzantineAction, &msg, "", "silent_byzantine", true)
		n.mu.Unlock()
		return
	}
	if !n.validateViewChangeLocked(msg) {
		n.mu.Unlock()
		return
	}
	if msg.View > n.ViewChangeTarget {
		n.ViewChangeTarget = msg.View
		n.consensusStart = true
		n.State.TimedOut = false
		n.State.TimeoutReason = ""
		n.scheduleProgressTimeoutLocked("waiting_for_new_view")
	}
	if n.ViewChangeEvidence[msg.View] == nil {
		n.ViewChangeEvidence[msg.View] = make(map[string]model.Message)
	}
	n.ViewChangeEvidence[msg.View][msg.From] = msg

	expectedLeader := n.expectedLeaderIDLocked(msg.View)
	if n.Config.ID != expectedLeader || n.NewViewSent[msg.View] || len(n.ViewChangeEvidence[msg.View]) < n.quorum() {
		n.mu.Unlock()
		return
	}

	value, preparedView := n.newViewValueLocked()
	newView := model.Message{
		Type:         model.MsgNewView,
		View:         msg.View,
		Sequence:     n.State.Sequence,
		From:         n.Config.ID,
		Value:        value,
		PreparedView: preparedView,
	}
	if value != "" {
		newView.Digest = Digest(value)
	}
	n.NewViewSent[msg.View] = true
	n.mu.Unlock()

	log.Printf("[%s] broadcasting NEW_VIEW for view=%d value=%s", n.Config.ID, msg.View, value)
	n.Broadcast(newView)
	n.ProcessMessage(newView)
}

func (n *Node) handleNewView(msg model.Message) {
	n.mu.Lock()
	if n.shouldStaySilentLocked() {
		n.appendEventLocked(model.EventByzantineAction, &msg, "", "silent_byzantine", true)
		n.mu.Unlock()
		return
	}
	if !n.validateNewViewLocked(msg) {
		n.mu.Unlock()
		return
	}

	n.advanceToViewLocked(msg.View)
	n.scheduleProposalTimeoutLocked()
	n.mu.Unlock()

	if msg.Value == "" {
		return
	}

	prePrepare := model.Message{
		Type:     model.MsgPrePrepare,
		View:     msg.View,
		Sequence: msg.Sequence,
		From:     msg.From,
		Value:    msg.Value,
		Digest:   msg.Digest,
	}
	n.ProcessMessage(prePrepare)
}

func (n *Node) shouldStaySilentLocked() bool {
	return n.Config.Byzantine && n.Config.Behavior == model.BehaviorSilent
}

func (n *Node) validateBasicLocked(msg model.Message) bool {
	if msg.View != n.State.View {
		n.recordRejectLocked("unexpected view")
		n.appendEventLocked(model.EventRejected, &msg, "", "unexpected view", false)
		log.Printf("[%s] rejected %s from %s due to unexpected view", n.Config.ID, msg.Type, msg.From)
		return false
	}
	if msg.Sequence != n.State.Sequence {
		n.recordRejectLocked("unexpected sequence")
		n.appendEventLocked(model.EventRejected, &msg, "", "unexpected sequence", false)
		log.Printf("[%s] rejected %s from %s due to unexpected sequence", n.Config.ID, msg.Type, msg.From)
		return false
	}
	if msg.Digest != Digest(msg.Value) {
		n.recordRejectLocked("digest mismatch")
		n.appendEventLocked(model.EventRejected, &msg, "", "digest mismatch", false)
		log.Printf("[%s] rejected %s from %s due to digest mismatch", n.Config.ID, msg.Type, msg.From)
		return false
	}
	return true
}

func (n *Node) validatePrePrepareLeaderLocked(msg model.Message) bool {
	expectedLeader := n.expectedLeaderIDLocked(n.State.View)
	if msg.From != expectedLeader {
		n.recordRejectLocked("unexpected leader")
		n.appendEventLocked(model.EventRejected, &msg, "", "unexpected leader", false)
		log.Printf("[%s] rejected %s from %s due to unexpected leader, expected %s", n.Config.ID, msg.Type, msg.From, expectedLeader)
		return false
	}
	return true
}

func (n *Node) validateViewChangeLocked(msg model.Message) bool {
	if msg.View <= n.State.View {
		n.recordRejectLocked("stale view change")
		n.appendEventLocked(model.EventRejected, &msg, "", "stale view change", false)
		return false
	}
	if msg.Sequence != n.State.Sequence {
		n.recordRejectLocked("unexpected sequence")
		n.appendEventLocked(model.EventRejected, &msg, "", "unexpected sequence", false)
		return false
	}
	if msg.Value == "" && msg.Digest != "" {
		n.recordRejectLocked("view change digest mismatch")
		n.appendEventLocked(model.EventRejected, &msg, "", "view change digest mismatch", false)
		return false
	}
	if msg.Value != "" && msg.Digest != Digest(msg.Value) {
		n.recordRejectLocked("view change digest mismatch")
		n.appendEventLocked(model.EventRejected, &msg, "", "view change digest mismatch", false)
		return false
	}
	return true
}

func (n *Node) validateNewViewLocked(msg model.Message) bool {
	if msg.View <= n.State.View {
		n.recordRejectLocked("stale new view")
		n.appendEventLocked(model.EventRejected, &msg, "", "stale new view", false)
		return false
	}
	if msg.Sequence != n.State.Sequence {
		n.recordRejectLocked("unexpected sequence")
		n.appendEventLocked(model.EventRejected, &msg, "", "unexpected sequence", false)
		return false
	}
	if msg.From != n.expectedLeaderIDLocked(msg.View) {
		n.recordRejectLocked("unexpected new view leader")
		n.appendEventLocked(model.EventRejected, &msg, "", "unexpected new view leader", false)
		return false
	}
	if msg.Value != "" && msg.Digest != Digest(msg.Value) {
		n.recordRejectLocked("new view digest mismatch")
		n.appendEventLocked(model.EventRejected, &msg, "", "new view digest mismatch", false)
		return false
	}
	return true
}
