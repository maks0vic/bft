package node

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"bft/internal/model"
)

var httpClient = &http.Client{
	Timeout: 3 * time.Second,
}

func (n *Node) Broadcast(msg model.Message) {
	for _, peer := range n.Config.Peers {
		go n.SendToPeer(peer, msg, false, "")
	}
}

func (n *Node) BroadcastPossiblyConflicting(msg model.Message) {
	if !n.Config.Byzantine || n.Config.Behavior != model.BehaviorConflictingValue {
		n.Broadcast(msg)
		return
	}

	for idx, peer := range n.Config.Peers {
		selected := msg
		details := ""
		if idx%2 == 1 {
			selected.Value = msg.Value + "_tampered"
			selected.Digest = Digest(selected.Value)
			details = "conflicting_value"
		}
		go n.SendToPeer(peer, selected, details != "", details)
	}
}

func (n *Node) SendToPeer(peer model.Peer, msg model.Message, byzantineAction bool, details string) {
	msg.Signature = signatureForMessage(msg)

	n.mu.Lock()
	n.appendEventLocked(model.EventMessageSent, &msg, peer.ID, details, n.Config.Byzantine)
	if byzantineAction {
		n.appendEventLocked(model.EventByzantineAction, &msg, peer.ID, details, true)
	}
	n.mu.Unlock()

	body, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[%s] marshal error: %v", n.Config.ID, err)
		return
	}

	resp, err := httpClient.Post("http://"+peer.Address+"/message", "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[%s] send error to %s: %v", n.Config.ID, peer.Address, err)
		return
	}
	defer resp.Body.Close()
}
