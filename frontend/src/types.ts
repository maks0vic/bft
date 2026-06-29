export type EventKind =
  | "CONSENSUS_STARTED"
  | "MESSAGE_SENT"
  | "MESSAGE_RECEIVED"
  | "QUORUM_REACHED"
  | "NODE_PREPARED"
  | "NODE_COMMITTED"
  | "NODE_DECIDED"
  | "BYZANTINE_ACTION"
  | "MESSAGE_REJECTED"
  | "MESSAGE_BUFFERED"
  | "TIMEOUT";

export type NodeView = {
  id: string;
  leader: boolean;
  byzantine: boolean;
  behavior: string;
  phase: string;
  acceptedValue: string;
  outgoingValue: string;
  decision: string;
  prepareCount: number;
  commitCount: number;
};

export type SimulationState = {
  simulationId: string;
  quorum: number;
  consensusReached: boolean;
  finalValue: string;
  running: boolean;
  view: number;
  sequence: number;
  nodes: NodeView[];
};

export type CanonicalEvent = {
  id: string;
  globalSequence: number;
  timestamp: string;
  kind: EventKind;
  from?: string;
  to?: string;
  nodeId?: string;
  messageType?: string;
  value?: string;
  malicious?: boolean;
  details?: string;
};

export type NodeEvent = {
  id: string;
  timestamp: string;
  kind: EventKind;
  nodeId?: string;
  from?: string;
  to?: string;
  messageType?: string;
  value?: string;
  malicious?: boolean;
  details?: string;
};

export type EventsResponse = {
  events: CanonicalEvent[];
  eventsByNode: Record<string, NodeEvent[]>;
  lastSequence: number;
};
