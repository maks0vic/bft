package node

import (
	"log"

	"bft/internal/model"
)

func (n *Node) StartConsensus(value string) {
	n.mu.Lock()
	if n.consensusStart {
		n.mu.Unlock()
		log.Printf("[%s] consensus already started", n.Config.ID)
		return
	}
	n.consensusStart = true

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
	if existing := n.PrePrepare; existing != nil && (existing.View != msg.View || existing.Sequence != msg.Sequence || existing.Digest != msg.Digest) {
		n.recordRejectLocked("conflicting pre-prepare")
		n.appendEventLocked(model.EventRejected, &msg, "", "conflicting pre-prepare", false)
		n.mu.Unlock()
		log.Printf("[%s] rejected conflicting PRE_PREPARE from %s", n.Config.ID, msg.From)
		return
	}

	n.PrePrepare = &msg
	n.consensusStart = true
	n.State.ProposedValue = msg.Value

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
	n.PrepareMsgs[n.Config.ID] = prepare
	matchCount := n.matchingPrepareCountLocked()
	shouldCommit := false
	var commit model.Message
	if !n.State.Prepared && matchCount >= n.quorum() {
		n.State.Prepared = true
		n.appendEventLocked(model.EventQuorumReached, &prepare, "", "prepare quorum reached", false)
		n.appendEventLocked(model.EventNodePrepared, &prepare, "", "node prepared", false)
		commitValue := n.State.ProposedValue
		if n.Config.Byzantine && n.Config.Behavior == model.BehaviorConflictingValue {
			commitValue = n.State.ProposedValue + "_tampered"
		}
		commit = model.Message{
			Type:     model.MsgCommit,
			View:     msg.View,
			Sequence: msg.Sequence,
			From:     n.Config.ID,
			Value:    commitValue,
			Digest:   Digest(commitValue),
		}
		n.CommitMsgs[n.Config.ID] = commit
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
	if n.State.ProposedValue == "" {
		n.PrepareMsgs[msg.From] = msg
		n.appendEventLocked(model.EventBuffered, &msg, "", "waiting_for_preprepare", false)
		n.mu.Unlock()
		log.Printf("[%s] buffering PREPARE from %s until PRE_PREPARE arrives", n.Config.ID, msg.From)
		return
	}
	if msg.Value != n.State.ProposedValue {
		n.recordRejectLocked("prepare value mismatch")
		n.appendEventLocked(model.EventRejected, &msg, "", "prepare value mismatch", false)
		n.mu.Unlock()
		log.Printf("[%s] rejected PREPARE value mismatch from %s", n.Config.ID, msg.From)
		return
	}

	n.PrepareMsgs[msg.From] = msg
	matchCount := n.matchingPrepareCountLocked()
	if n.State.Prepared || matchCount < n.quorum() {
		n.mu.Unlock()
		return
	}

	n.State.Prepared = true
	n.appendEventLocked(model.EventQuorumReached, &msg, "", "prepare quorum reached", false)
	n.appendEventLocked(model.EventNodePrepared, &msg, "", "node prepared", false)

	commitValue := n.State.ProposedValue
	if n.Config.Byzantine && n.Config.Behavior == model.BehaviorConflictingValue {
		commitValue = n.State.ProposedValue + "_tampered"
	}

	commit := model.Message{
		Type:     model.MsgCommit,
		View:     msg.View,
		Sequence: msg.Sequence,
		From:     n.Config.ID,
		Value:    commitValue,
		Digest:   Digest(commitValue),
	}
	n.CommitMsgs[n.Config.ID] = commit
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
	if n.State.ProposedValue == "" {
		n.CommitMsgs[msg.From] = msg
		n.appendEventLocked(model.EventBuffered, &msg, "", "waiting_for_preprepare", false)
		n.mu.Unlock()
		log.Printf("[%s] buffering COMMIT from %s until PRE_PREPARE arrives", n.Config.ID, msg.From)
		return
	}
	if msg.Value != n.State.ProposedValue {
		n.recordRejectLocked("commit value mismatch")
		n.appendEventLocked(model.EventRejected, &msg, "", "commit value mismatch", false)
		n.mu.Unlock()
		log.Printf("[%s] rejected COMMIT value mismatch from %s", n.Config.ID, msg.From)
		return
	}

	n.CommitMsgs[msg.From] = msg
	matchCount := n.matchingCommitCountLocked()
	if n.State.Decided || matchCount < n.quorum() {
		n.mu.Unlock()
		return
	}

	n.State.Committed = true
	n.appendEventLocked(model.EventNodeCommitted, &msg, "", "node committed", false)
	n.appendEventLocked(model.EventQuorumReached, &msg, "", "commit quorum reached", false)
	n.State.Decided = true
	n.State.Decision = n.State.ProposedValue
	n.consensusStart = false
	decision := n.State.Decision
	n.appendEventLocked(model.EventNodeDecided, &msg, "", "decision="+decision, false)
	n.mu.Unlock()

	log.Printf("[%s] DECIDED value=%s with %d matching COMMIT messages", n.Config.ID, decision, matchCount)
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
