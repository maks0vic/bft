import type { SimulationState } from "../types";

type Props = {
  state: SimulationState | null;
};

export function SummaryBar({ state }: Props) {
  return (
    <section className="grid gap-3 md:grid-cols-4">
      <Metric label="Simulation" value={state?.simulationId ?? "loading"} />
      <Metric label="Status" value={state ? (state.running ? "Running" : state.consensusReached ? "Consensus reached" : "Idle") : "..."} />
      <Metric label="Final Value" value={state?.finalValue || "n/a"} />
      <Metric label="Quorum" value={state ? String(state.quorum) : "..."} />
    </section>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-3xl bg-white p-4 shadow-sm ring-1 ring-slate-200">
      <div className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">{label}</div>
      <div className="mt-2 text-xl font-semibold text-ink">{value}</div>
    </div>
  );
}
