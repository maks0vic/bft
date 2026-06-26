# BFT

This project is a local Go simulator for Byzantine Fault Tolerant consensus in distributed systems, with a React dashboard for visualizing node state and message history.

It models four HTTP-based nodes, including one configurable Byzantine replica, and demonstrates whether honest nodes can still agree on the same value using a simplified PBFT-inspired flow.

## What It Does

The system simulates:

- separate node processes running as Go HTTP servers
- one leader proposing a value
- replica nodes exchanging `PRE_PREPARE`, `PREPARE`, and `COMMIT` messages
- Byzantine behavior such as `silent` and `conflicting_value`
- a central coordinator API that aggregates state and events for the frontend
- a React dashboard that visualizes node status, event flow, and consensus outcome

The main question the simulator answers is:

> Can honest nodes still agree on the same value when one node behaves maliciously?
> 
>
## How To Start Everything

### 1. Start backend stack

From the project root:

```bash
cd /Users/smaksovic/GolandProjects/bft
go run ./cmd/launcher
```

Expected behavior:

- nodes start on ports `8001`-`8004`
- coordinator starts on port `9000`
- launcher stays running and waits for frontend/coordinator actions

### 2. Start frontend

In another terminal:

```bash
cd /Users/smaksovic/GolandProjects/bft/frontend
npm install
npm run dev
```

Open:

[`http://localhost:5173`](http://localhost:5173)

### 3. Use the UI

The dashboard provides:

- `Start`
- `Reset`
- proposal value input
- node graph
- event log
- summary bar

## Architecture

There are 3 main runtime parts:

1. `node` services
Each node runs as its own HTTP server.

Responsibilities:

- hold local consensus state
- receive and process protocol messages
- expose local `/state`
- expose local `/events`
- support `/reset`

2. `coordinator`
The coordinator is the single backend the frontend talks to.

Responsibilities:

- aggregate `/state` from all nodes
- aggregate `/events` from all nodes
- expose unified `GET /state`
- expose unified `GET /events`
- accept `POST /start`
- accept `POST /reset`

3. `frontend`
The React app polls the coordinator and renders:

- summary status
- node cards
- graph of nodes and message activity
- event log

## Lifecycle

The launcher is now a passive supervisor.

When you start the launcher:

- it starts all node processes
- it starts the coordinator
- it waits
- it does not start a simulation automatically

The simulation starts only when the coordinator receives:

```http
POST /start
```

Reset behavior:

```http
POST /reset
```

Reset does not restart OS processes. It resets the running backend stack to idle mode:

- nodes clear consensus state
- nodes clear event history
- coordinator clears merged event history
- coordinator issues a fresh `simulationId`
- the whole system returns to waiting mode

After reset:

- `running=false`
- `finalValue=""`
- node phases are `idle`
- prepare/commit counts are `0`
- `/events` is empty

## Project Structure

```text
bft/
├── cmd/
│   ├── client/
│   │   └── main.go
│   ├── coordinator/
│   │   └── main.go
│   ├── launcher/
│   │   └── main.go
│   └── node/
│       └── main.go
├── configs/
│   ├── node1.json
│   ├── node2.json
│   ├── node3.json
│   └── node4.json
├── frontend/
│   ├── src/
│   ├── package.json
│   └── vite.config.ts
├── internal/
│   ├── config/
│   ├── coordinator/
│   ├── model/
│   └── node/
├── go.mod
└── README.md
```

## Go Version

This project uses:

```text
Go 1.26.2
```

## Frontend Stack

The frontend uses:

- Vite
- React
- TypeScript
- Tailwind CSS
- React Flow

## Configuration

Each node is configured with its own JSON file in `configs/`.

Example fields:

- `id`
- `address`
- `leader`
- `byzantine`
- `behavior`
- `peers`

The default demo uses 4 nodes:

- `node1` is leader
- `node4` is Byzantine
- `node4` uses `conflicting_value`

## Available Binaries

### `cmd/launcher`

Starts:

- all node processes
- the coordinator

Then waits until stopped.

Run:

```bash
go run ./cmd/launcher
```

Optional flags:

```bash
go run ./cmd/launcher -config-dir configs -coordinator-addr localhost:9000
```

### `cmd/node`

Runs a single node server.

Example:

```bash
go run ./cmd/node -config configs/node1.json
```

### `cmd/coordinator`

Runs the coordinator API directly.

Example:

```bash
go run ./cmd/coordinator -config-dir configs -addr localhost:9000
```

### `cmd/client`

Sends a start request directly to the leader.

This is mostly useful for low-level manual testing now that the coordinator owns the frontend-facing control flow.

Example:

```bash
go run ./cmd/client -leader localhost:8001 -value attack
```

## Ports

Default ports:

- `8001` node1
- `8002` node2
- `8003` node3
- `8004` node4
- `9000` coordinator
- `5173` frontend dev server

## Coordinator API

Base URL:

[`http://localhost:9000`](http://localhost:9000)

### `GET /state`

Returns the current cluster snapshot for the dashboard.

Example shape:

```json
{
  "simulationId": "sim-001",
  "quorum": 3,
  "consensusReached": true,
  "finalValue": "attack",
  "running": false,
  "view": 0,
  "sequence": 1,
  "nodes": [
    {
      "id": "node1",
      "leader": true,
      "byzantine": false,
      "behavior": "",
      "phase": "decided",
      "proposedValue": "attack",
      "decision": "attack",
      "prepareCount": 3,
      "commitCount": 3
    }
  ]
}
```

### `GET /events`

Returns:

- canonical merged events
- grouped node-local events
- `lastSequence` cursor for polling

Supports:

```http
GET /events?since=42
```

Example shape:

```json
{
  "events": [
    {
      "id": "node1-e1",
      "globalSequence": 1,
      "timestamp": "2026-04-10T22:16:27Z",
      "kind": "CONSENSUS_STARTED",
      "from": "node1",
      "nodeId": "node1",
      "messageType": "PRE_PREPARE",
      "value": "attack"
    }
  ],
  "eventsByNode": {
    "node1": []
  },
  "lastSequence": 1
}
```

### `POST /start`

Starts a new simulation through the coordinator.

Body:

```json
{
  "value": "attack"
}
```

Important:

- `POST /start` auto-resets the previous simulation first
- this means repeated Start presses begin a fresh run

### `POST /reset`

Clears:

- node state
- node events
- coordinator event aggregation

Then returns the stack to idle waiting mode.

## Node API

Each node exposes:

- `POST /start`
- `POST /message`
- `GET /state`
- `GET /events`
- `POST /reset`

These are used internally by the coordinator and the simulation itself.

### Node `/state`

Node-local snapshot only.

Includes:

- `id`
- `leader`
- `byzantine`
- `behavior`
- `running`
- `phase`
- `state`
- `prepare_matches`
- `commit_matches`
- `reject_count`

### Node `/events`

Node-local ordered append-only history for the current run.

Properties:

- oldest to newest
- stable IDs
- immutable during the run
- cleared by reset

## Frontend Polling Model

The React app polls:

- `/state` every second
- `/events` every second

For `/events`, it tracks `lastSequence` and asks only for new events.

The backend also handles restart safety:

- if the client sends an old `since` cursor from a previous run, the coordinator falls back safely and serves the new run’s events

## Event Model

Important event kinds include:

- `CONSENSUS_STARTED`
- `MESSAGE_SENT`
- `MESSAGE_RECEIVED`
- `QUORUM_REACHED`
- `NODE_PREPARED`
- `NODE_COMMITTED`
- `NODE_DECIDED`
- `BYZANTINE_ACTION`
- `MESSAGE_REJECTED`
- `MESSAGE_BUFFERED`

Event rules:

- node event IDs are stable
- node events are append-only within a run
- coordinator assigns strictly increasing `globalSequence`
- reset clears all event history and starts `lastSequence` back at `0`

## Current Demo Scenario

Default behavior:

- 4 nodes
- quorum = `3`
- 1 Byzantine replica
- Byzantine behavior = `conflicting_value`

Expected outcome:

- honest nodes still decide the same value

## How To Stop Everything

If the launcher is running in a terminal, stop the whole backend stack with:

```bash
Ctrl + C
```

The launcher is expected to keep running while the backend is alive.

Why:

- launcher keeps node processes alive
- launcher keeps coordinator alive
- frontend depends on the coordinator staying up

## If Ports Are Already In Use

If you see errors like:

```text
bind: address already in use
```

it usually means old launcher/node/coordinator processes are still running.

### Inspect ports

```bash
lsof -nP -iTCP:8001 -iTCP:8002 -iTCP:8003 -iTCP:8004 -iTCP:9000 -sTCP:LISTEN
```

### Kill processes by port

```bash
lsof -ti -iTCP:8001 -iTCP:8002 -iTCP:8003 -iTCP:8004 -iTCP:9000 | xargs kill
```

If needed:

```bash
lsof -ti -iTCP:8001 -iTCP:8002 -iTCP:8003 -iTCP:8004 -iTCP:9000 | xargs kill -9
```

### Kill by process name

```bash
pkill -f '/cmd/node'
pkill -f '/cmd/coordinator'
pkill -f '/cmd/launcher'
```

Then start again:

```bash
go run ./cmd/launcher
```

## Typical Development Workflow

1. Start backend:

```bash
go run ./cmd/launcher
```

2. Start frontend:

```bash
cd frontend
npm run dev
```

3. Open dashboard

4. Press `Start`

5. Press `Reset`

6. Press `Start` again for a new run

## Known Behavior

- launcher does not start simulations on its own
- coordinator is the single control point for the frontend
- reset returns the system to idle waiting mode
- the launcher terminal staying open is expected

## Notes

- This is a demo/learning project, not a production BFT implementation.
- No automated tests are currently included because testing was intentionally deferred until implementation is complete.
