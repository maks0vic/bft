type Props = {
  value: string;
  status: string;
  busy: boolean;
  onChange: (value: string) => void;
  onStart: () => void;
  onReset: () => void;
};

export function ControlPanel({ value, status, busy, onChange, onStart, onReset }: Props) {
  return (
    <section className="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-slate-200">
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div className="flex-1">
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
        <div className="flex gap-3">
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
