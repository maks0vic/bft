package model

import "time"

type MessageType string

const (
	MsgPrePrepare MessageType = "PRE_PREPARE"
	MsgPrepare    MessageType = "PREPARE"
	MsgCommit     MessageType = "COMMIT"
)

type EventKind string

const (
	EventConsensusStarted EventKind = "CONSENSUS_STARTED"
	EventMessageSent      EventKind = "MESSAGE_SENT"
	EventMessageReceived  EventKind = "MESSAGE_RECEIVED"
	EventQuorumReached    EventKind = "QUORUM_REACHED"
	EventNodePrepared     EventKind = "NODE_PREPARED"
	EventNodeCommitted    EventKind = "NODE_COMMITTED"
	EventNodeDecided      EventKind = "NODE_DECIDED"
	EventByzantineAction  EventKind = "BYZANTINE_ACTION"
	EventRejected         EventKind = "MESSAGE_REJECTED"
	EventBuffered         EventKind = "MESSAGE_BUFFERED"
	EventReset            EventKind = "SIMULATION_RESET"
)

const (
	BehaviorSilent           = "silent"
	BehaviorConflictingValue = "conflicting_value"
)

type Message struct {
	Type      MessageType `json:"type"`
	View      int         `json:"view"`
	Sequence  int         `json:"sequence"`
	From      string      `json:"from"`
	Value     string      `json:"value"`
	Digest    string      `json:"digest"`
	Signature string      `json:"signature,omitempty"`
}

type Peer struct {
	ID        string `json:"id"`
	Address   string `json:"address"`
	Byzantine bool   `json:"byzantine"`
}

type NodeConfig struct {
	ID        string `json:"id"`
	Address   string `json:"address"`
	Leader    bool   `json:"leader"`
	Byzantine bool   `json:"byzantine"`
	Behavior  string `json:"behavior"`
	Peers     []Peer `json:"peers"`
}

type ConsensusState struct {
	View          int    `json:"view"`
	Sequence      int    `json:"sequence"`
	ProposedValue string `json:"proposed_value"`
	Prepared      bool   `json:"prepared"`
	Committed     bool   `json:"committed"`
	Decided       bool   `json:"decided"`
	Decision      string `json:"decision"`
}

type StateResponse struct {
	ID             string         `json:"id"`
	Leader         bool           `json:"leader"`
	Byzantine      bool           `json:"byzantine"`
	Behavior       string         `json:"behavior"`
	Running        bool           `json:"running"`
	Phase          string         `json:"phase"`
	AcceptedValue  string         `json:"acceptedValue"`
	OutgoingValue  string         `json:"outgoingValue"`
	State          ConsensusState `json:"state"`
	PrepareMatches int            `json:"prepare_matches"`
	CommitMatches  int            `json:"commit_matches"`
	RejectCount    int            `json:"reject_count"`
	LastReject     string         `json:"last_reject,omitempty"`
}

type NodeEvent struct {
	ID          string      `json:"id"`
	Timestamp   time.Time   `json:"timestamp"`
	Kind        EventKind   `json:"kind"`
	NodeID      string      `json:"nodeId,omitempty"`
	From        string      `json:"from,omitempty"`
	To          string      `json:"to,omitempty"`
	MessageType MessageType `json:"messageType,omitempty"`
	Value       string      `json:"value,omitempty"`
	Malicious   bool        `json:"malicious,omitempty"`
	Details     string      `json:"details,omitempty"`
}

type EventsResponse struct {
	ID     string      `json:"id"`
	Events []NodeEvent `json:"events"`
}

type ResetResponse struct {
	Status string `json:"status"`
}

type StartRequest struct {
	Value string `json:"value"`
}

type NodeView struct {
	ID            string `json:"id"`
	Leader        bool   `json:"leader"`
	Byzantine     bool   `json:"byzantine"`
	Behavior      string `json:"behavior"`
	Phase         string `json:"phase"`
	AcceptedValue string `json:"acceptedValue"`
	OutgoingValue string `json:"outgoingValue"`
	Decision      string `json:"decision"`
	PrepareCount  int    `json:"prepareCount"`
	CommitCount   int    `json:"commitCount"`
}

type SimulationState struct {
	SimulationID     string     `json:"simulationId"`
	Quorum           int        `json:"quorum"`
	ConsensusReached bool       `json:"consensusReached"`
	FinalValue       string     `json:"finalValue"`
	Running          bool       `json:"running"`
	View             int        `json:"view"`
	Sequence         int        `json:"sequence"`
	Nodes            []NodeView `json:"nodes"`
}

type CanonicalEvent struct {
	ID             string      `json:"id"`
	GlobalSequence int64       `json:"globalSequence"`
	Timestamp      time.Time   `json:"timestamp"`
	Kind           EventKind   `json:"kind"`
	From           string      `json:"from,omitempty"`
	To             string      `json:"to,omitempty"`
	NodeID         string      `json:"nodeId,omitempty"`
	MessageType    MessageType `json:"messageType,omitempty"`
	Value          string      `json:"value,omitempty"`
	Malicious      bool        `json:"malicious,omitempty"`
	Details        string      `json:"details,omitempty"`
}

type CoordinatorEventsResponse struct {
	Events       []CanonicalEvent       `json:"events"`
	EventsByNode map[string][]NodeEvent `json:"eventsByNode"`
	LastSequence int64                  `json:"lastSequence"`
}
