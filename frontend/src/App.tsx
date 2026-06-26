import { useMemo, useState } from "react";
import { resetSimulation, startSimulation } from "./api";
import { ControlPanel } from "./components/ControlPanel";
import { EventLog } from "./components/EventLog";
import { NetworkGraph } from "./components/NetworkGraph";
import { NodeCard } from "./components/NodeCard";
import { SummaryBar } from "./components/SummaryBar";
import { useEventPoll } from "./hooks/useEventPoll";
import { useStatePoll } from "./hooks/useStatePoll";

export default function App() {
  const [value, setValue] = useState("attack");
  const [refreshKey, setRefreshKey] = useState(0);
  const [status, setStatus] = useState("Ready.");
  const [busy, setBusy] = useState(false);

  const { state, error: stateError } = useStatePoll(refreshKey);
  const { events, error: eventError, setEvents, setEventsByNode, setLastSequence } = useEventPoll(refreshKey);

  const recentEvents = useMemo(() => events.slice(-8), [events]);

  async function handleStart() {
    setBusy(true);
    try {
      setEvents([]);
      setEventsByNode({});
      setLastSequence(0);
      setRefreshKey((current) => current + 1);
      await startSimulation(value);
      setStatus(`Simulation started with value "${value}".`);
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Failed to start simulation.");
    } finally {
      setBusy(false);
    }
  }

  async function handleReset() {
    setBusy(true);
    try {
      await resetSimulation();
      setEvents([]);
      setEventsByNode({});
      setLastSequence(0);
      setRefreshKey((current) => current + 1);
      setStatus("Simulation reset.");
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Failed to reset simulation.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="min-h-screen bg-[radial-gradient(circle_at_top,_#f8fbf9,_#e7efe9_45%,_#dbe6df)] px-4 py-6 text-ink md:px-8">
      <div className="mx-auto max-w-7xl space-y-6">
        <header className="space-y-2">
          <p className="text-sm font-semibold uppercase tracking-[0.26em] text-accent">Byzantine Fault Tolerant Consensus</p>
          <h1 className="text-4xl font-semibold tracking-tight">Coordinator Dashboard</h1>
          <p className="max-w-3xl text-slate-600">
            Watch a 4-node PBFT-inspired simulation, inspect each node’s state, and follow the event stream that drives the graph.
          </p>
        </header>

        <SummaryBar state={state} />
        <ControlPanel value={value} status={status} busy={busy} onChange={setValue} onStart={handleStart} onReset={handleReset} />

        {(stateError || eventError) && (
          <div className="rounded-3xl border border-warn/30 bg-rose-50 px-4 py-3 text-sm text-warn">
            {[stateError, eventError].filter(Boolean).join(" | ")}
          </div>
        )}

        <div className="grid gap-6 xl:grid-cols-[1.3fr_0.7fr]">
          <NetworkGraph key={state?.simulationId ?? "idle"} state={state} recentEvents={recentEvents} />
          <EventLog events={events} />
        </div>

        <section>
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-ink">Node States</h2>
            <span className="text-sm text-slate-500">{state?.nodes.length ?? 0} nodes</span>
          </div>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            {state?.nodes.map((node) => <NodeCard key={node.id} node={node} />)}
          </div>
        </section>
      </div>
    </main>
  );
}
