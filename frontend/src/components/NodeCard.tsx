import type { NodeView } from "../types";

export function NodeCard({ node }: { node: NodeView }) {
  const badge = node.byzantine ? "bg-warn text-white" : node.leader ? "bg-gold text-ink" : "bg-slate-200 text-slate-700";

  return (
    <article className="rounded-3xl bg-white p-4 shadow-sm ring-1 ring-slate-200">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold text-ink">{node.id}</h3>
        <span className={`rounded-full px-3 py-1 text-xs font-semibold uppercase ${badge}`}>
          {node.byzantine ? node.behavior || "Byzantine" : node.leader ? "Leader" : "Replica"}
        </span>
      </div>
      <div className="mt-3 grid grid-cols-2 gap-3 text-sm text-slate-600">
        <Field label="Phase" value={node.phase} />
        <Field label="Decision" value={node.decision || "pending"} />
        <Field label="Prepare" value={String(node.prepareCount)} />
        <Field label="Commit" value={String(node.commitCount)} />
      </div>
      <div className="mt-3 rounded-2xl bg-slate-50 p-3 text-sm text-slate-600">
        <div className="font-medium text-slate-700">Proposed</div>
        <div>{node.proposedValue || "none"}</div>
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
