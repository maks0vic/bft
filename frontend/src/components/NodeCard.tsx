import type { NodeView } from "../types";

export function NodeCard({ node }: { node: NodeView }) {
  const accent = node.byzantine
    ? "border-warn/30 bg-[linear-gradient(180deg,_#fff5f4,_#fff)]"
    : node.leader
      ? "border-gold/40 bg-[linear-gradient(180deg,_#fff8e7,_#fff)]"
      : "border-emerald-200 bg-[linear-gradient(180deg,_#f8fdfb,_#fff)]";
  const badge = node.byzantine ? "bg-warn text-white" : node.leader ? "bg-gold text-ink" : "bg-slate-200 text-slate-700";
  const phaseTone =
    node.phase === "decided"
      ? "bg-emerald-100 text-emerald-800"
      : node.phase === "prepared" || node.phase === "committed"
        ? "bg-amber-100 text-amber-800"
        : node.phase === "proposed"
          ? "bg-sky-100 text-sky-800"
          : "bg-slate-100 text-slate-600";

  return (
    <article className={`rounded-[1.75rem] border p-5 shadow-[0_12px_35px_rgba(15,23,42,0.07)] ${accent}`}>
      <div className="flex items-start justify-between gap-4">
        <div>
          <h3 className="text-xl font-semibold text-ink">{node.id}</h3>
          <div className="mt-2 flex flex-wrap gap-2">
            <span className={`rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] ${badge}`}>
              {node.byzantine ? node.behavior || "Byzantine" : node.leader ? "Leader" : "Replica"}
            </span>
            <span className={`rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] ${phaseTone}`}>
              {node.phase}
            </span>
          </div>
        </div>
        <div className="text-right">
          <div className="text-xs uppercase tracking-[0.2em] text-slate-400">Decision</div>
          <div className="mt-1 text-lg font-semibold text-ink">{node.decision || "pending"}</div>
        </div>
      </div>

      <div className="mt-5 grid grid-cols-2 gap-3 text-sm text-slate-600">
        <Field label="Prepare Count" value={String(node.prepareCount)} />
        <Field label="Commit Count" value={String(node.commitCount)} />
      </div>

      <div className="mt-5 rounded-3xl bg-white/80 p-4 text-sm text-slate-600 ring-1 ring-black/5">
        <div className="grid gap-4 md:grid-cols-2">
          <div>
            <div className="text-xs uppercase tracking-[0.18em] text-slate-400">Accepted Proposal</div>
            <div className="mt-1 font-medium text-ink">{node.acceptedValue || "none"}</div>
          </div>
          <div>
            <div className="text-xs uppercase tracking-[0.18em] text-slate-400">Outgoing Vote</div>
            <div className="mt-1 font-medium text-ink">{node.outgoingValue || "none"}</div>
          </div>
          <div>
            <div className="text-xs uppercase tracking-[0.18em] text-slate-400">Trust Profile</div>
            <div className="mt-1 font-medium text-ink">{node.byzantine ? "Potentially malicious" : "Honest replica"}</div>
          </div>
        </div>
      </div>
    </article>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs uppercase tracking-[0.18em] text-slate-400">{label}</div>
      <div className="mt-1 font-medium text-ink">{value}</div>
    </div>
  );
}
