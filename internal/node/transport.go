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
	for idx, peer := range n.Config.Peers {
		selected, byzantineAction, details := n.outboundMessageForPeer(msg, idx)
		go n.SendToPeer(peer, selected, byzantineAction, details)
	}
}

func (n *Node) BroadcastPossiblyConflicting(msg model.Message) {
	n.Broadcast(msg)
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

func (n *Node) outboundMessageForPeer(msg model.Message, peerIndex int) (model.Message, bool, string) {
	if !n.Config.Byzantine {
		return msg, false, ""
	}

	selected := msg
	switch n.Config.Behavior {
	case model.BehaviorConflictingValue:
		if (msg.Type == model.MsgPrepare || msg.Type == model.MsgCommit) && peerIndex%2 == 1 {
			selected.Value = msg.Value + "_tampered"
			selected.Digest = Digest(selected.Value)
			return selected, true, "conflicting_value"
		}
	case model.BehaviorEquivocatingViewChange:
		if msg.Type == model.MsgViewChange && peerIndex%2 == 1 {
			if selected.Value == "" {
				selected.Value = n.requestedValue + "_equivocated"
			} else {
				selected.Value = selected.Value + "_equivocated"
			}
			selected.Digest = Digest(selected.Value)
			return selected, true, "equivocating_view_change"
		}
	case model.BehaviorMalformedCertificate:
		if msg.Type == model.MsgViewChange || msg.Type == model.MsgNewView {
			selected.PreparedView = msg.PreparedView + 7
			selected.Digest = "malformed-digest"
			return selected, true, "malformed_certificate"
		}
	}

	return msg, false, ""
}
