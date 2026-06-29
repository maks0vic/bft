type Props = {
  value: string;
  nodeCount: string;
  byzantineCount: string;
  byzantineBehavior: string;
  status: string;
  busy: boolean;
  onChange: (value: string) => void;
  onNodeCountChange: (value: string) => void;
  onByzantineCountChange: (value: string) => void;
  onByzantineBehaviorChange: (value: string) => void;
  onStart: () => void;
  onReset: () => void;
};

export function ControlPanel({
  value,
  nodeCount,
  byzantineCount,
  byzantineBehavior,
  status,
  busy,
  onChange,
  onNodeCountChange,
  onByzantineCountChange,
  onByzantineBehaviorChange,
  onStart,
  onReset,
}: Props) {
  return (
    <section className="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-slate-200">
      <div className="grid gap-4 xl:grid-cols-[2fr_1fr_1fr_1fr_auto] xl:items-end">
        <div>
          <label className="mb-2 block text-sm font-semibold uppercase tracking-[0.2em] text-slate-500">
            Proposal Value
          </label>
          <input
            value={value}
            onChange={(event) => onChange(event.target.value)}
            className="w-full rounded-2xl border border-slate-300 bg-slate-50 px-4 py-3 text-base outline-none transition focus:border-accent focus:bg-white"
            placeholder="attack"
          />
        </div>
        <PlainNumberField label="Number of Nodes" value={nodeCount} onChange={onNodeCountChange} />
        <PlainNumberField label="Byzantine Nodes" value={byzantineCount} onChange={onByzantineCountChange} />
        <div>
          <label className="mb-2 block text-sm font-semibold uppercase tracking-[0.2em] text-slate-500">
            Byzantine Behavior
          </label>
          <select
            value={byzantineBehavior}
            onChange={(event) => onByzantineBehaviorChange(event.target.value)}
            className="w-full rounded-2xl border border-slate-300 bg-slate-50 px-4 py-3 text-base outline-none transition focus:border-accent focus:bg-white"
          >
            <option value="conflicting_value">conflicting_value</option>
            <option value="silent">silent</option>
            <option value="invalid_leader_proposal">invalid_leader_proposal</option>
            <option value="stale_view_spam">stale_view_spam</option>
            <option value="malformed_certificate">malformed_certificate</option>
            <option value="equivocating_view_change">equivocating_view_change</option>
          </select>
        </div>
        <div className="flex gap-3 xl:justify-end">
          <button
            onClick={onStart}
            disabled={busy}
            className="rounded-2xl bg-accent px-5 py-3 font-semibold text-white transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:bg-slate-400"
          >
            Start
          </button>
          <button
            onClick={onReset}
            disabled={busy}
            className="rounded-2xl border border-slate-300 bg-white px-5 py-3 font-semibold text-slate-700 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:text-slate-400"
          >
            Reset
          </button>
        </div>
      </div>
      <p className="mt-3 text-sm text-slate-500">{status}</p>
    </section>
  );
}

function PlainNumberField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <div>
      <label className="mb-2 block text-sm font-semibold uppercase tracking-[0.2em] text-slate-500">{label}</label>
      <input
        type="text"
        inputMode="numeric"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="w-full rounded-2xl border border-slate-300 bg-slate-50 px-4 py-3 text-base outline-none transition focus:border-accent focus:bg-white"
      />
    </div>
  );
}
