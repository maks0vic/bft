import type { SimulationState } from "../types";

type Props = {
  state: SimulationState | null;
};

export function SummaryBar({ state }: Props) {
  return (
    <section className="grid gap-3 md:grid-cols-2 xl:grid-cols-6">
      <Metric label="Simulation" value={state?.simulationId ?? "loading"} tone="neutral" />
      <Metric
        label="Status"
        value={state ? (state.stalled ? "Stalled" : state.running ? "Running" : state.consensusReached ? "Consensus reached" : "Idle") : "..."}
        tone={state?.stalled ? "warn" : state?.running ? "info" : state?.consensusReached ? "success" : "neutral"}
      />
      <Metric label="Final Value" value={state?.finalValue || "n/a"} tone={state?.finalValue ? "success" : "neutral"} />
      <Metric label="Leader" value={state?.currentLeaderId || "n/a"} tone="neutral" />
      <Metric label="View" value={state ? String(state.view) : "..."} tone="info" />
      <Metric label="Quorum" value={state ? String(state.quorum) : "..."} tone="warn" />
    </section>
  );
}

function Metric({ label, value, tone }: { label: string; value: string; tone: "neutral" | "success" | "warn" | "info" }) {
  const tones = {
    neutral: "border-slate-200 bg-white",
    success: "border-emerald-200 bg-emerald-50/80",
    warn: "border-amber-200 bg-amber-50/80",
    info: "border-sky-200 bg-sky-50/80",
  };

  return (
    <div className={`rounded-[1.75rem] border p-4 shadow-sm ${tones[tone]}`}>
      <div className="text-xs font-semibold uppercase tracking-[0.22em] text-slate-400">{label}</div>
      <div className="mt-3 text-2xl font-semibold text-ink">{value}</div>
    </div>
  );
}
