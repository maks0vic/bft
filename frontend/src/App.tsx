import { useState } from "react";
import { resetSimulation, startSimulation } from "./api";
import { ControlPanel } from "./components/ControlPanel";
import { EventLog } from "./components/EventLog";
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
    <main className="min-h-screen bg-[radial-gradient(circle_at_top,_#f7f2e8,_#e8eee8_38%,_#d7e0db)] px-4 py-6 text-ink md:px-8">
      <div className="mx-auto max-w-7xl space-y-6">
        <header className="rounded-[2rem] bg-[linear-gradient(135deg,_rgba(16,33,43,0.96),_rgba(18,67,78,0.88))] px-6 py-8 text-white shadow-[0_24px_80px_rgba(16,33,43,0.2)] md:px-8">
          <p className="text-sm font-semibold uppercase tracking-[0.3em] text-emerald-200">Byzantine Fault Tolerant Consensus</p>
          <h1 className="mt-3 text-4xl font-semibold tracking-tight md:text-5xl">Mission Board</h1>
          <p className="mt-3 max-w-3xl text-sm leading-6 text-slate-200 md:text-base">
            Follow one fixed cluster view as the coordinator tracks proposal flow, quorum formation, Byzantine interference, and the final decision of each node.
          </p>
        </header>

        <SummaryBar state={state} />
        <ControlPanel value={value} status={status} busy={busy} onChange={setValue} onStart={handleStart} onReset={handleReset} />

        {(stateError || eventError) && (
          <div className="rounded-3xl border border-warn/30 bg-rose-50 px-4 py-3 text-sm text-warn">
            {[stateError, eventError].filter(Boolean).join(" | ")}
          </div>
        )}

        <section className="rounded-[2rem] border border-white/60 bg-white/75 p-5 shadow-[0_18px_50px_rgba(15,23,42,0.08)] backdrop-blur">
          <div className="mb-5 flex items-center justify-between">
            <div>
              <h2 className="text-2xl font-semibold text-ink">Cluster Nodes</h2>
              <p className="mt-1 text-sm text-slate-500">Static board view of the current consensus round.</p>
            </div>
            <span className="rounded-full bg-slate-100 px-4 py-2 text-sm font-medium text-slate-600">
              {state?.nodes.length ?? 0} nodes
            </span>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            {state?.nodes.map((node) => <NodeCard key={node.id} node={node} />)}
          </div>
        </section>

        <EventLog events={events} />
      </div>
    </main>
  );
}
