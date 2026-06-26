import type { CanonicalEvent } from "../types";

export function EventLog({ events }: { events: CanonicalEvent[] }) {
  return (
    <section className="rounded-3xl bg-white p-4 shadow-sm ring-1 ring-slate-200">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-ink">Event Log</h2>
        <span className="text-sm text-slate-500">{events.length} events</span>
      </div>
      <div className="max-h-[420px] space-y-3 overflow-y-auto pr-1">
        {events.length === 0 ? (
          <div className="rounded-2xl bg-slate-50 p-4 text-sm text-slate-500">No events yet.</div>
        ) : (
          events.slice().reverse().map((event) => (
            <div
              key={event.id}
              className={`rounded-2xl border p-3 text-sm ${event.malicious ? "border-warn/30 bg-rose-50" : "border-slate-200 bg-slate-50"}`}
            >
              <div className="flex items-center justify-between gap-3">
                <span className="font-semibold text-ink">{event.kind}</span>
                <span className="text-xs text-slate-500">#{event.globalSequence}</span>
              </div>
              <div className="mt-2 text-slate-600">
                {event.from && <span>from {event.from} </span>}
                {event.to && <span>to {event.to} </span>}
                {event.value && <span>value={event.value} </span>}
                {event.details && <span>({event.details})</span>}
              </div>
            </div>
          ))
        )}
      </div>
    </section>
  );
}
