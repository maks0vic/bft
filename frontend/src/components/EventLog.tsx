import type { CanonicalEvent } from "../types";

export function EventLog({ events }: { events: CanonicalEvent[] }) {
  return (
    <section className="rounded-[2rem] border border-white/60 bg-white/80 p-5 shadow-[0_18px_50px_rgba(15,23,42,0.08)] backdrop-blur">
      <div className="mb-5 flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-ink">Mission Log</h2>
          <p className="mt-1 text-sm text-slate-500">Structured cluster activity from newest to oldest.</p>
        </div>
        <span className="rounded-full bg-slate-100 px-4 py-2 text-sm font-medium text-slate-600">{events.length} events</span>
      </div>
      <div className="max-h-[460px] space-y-3 overflow-y-auto pr-1">
        {events.length === 0 ? (
          <div className="rounded-3xl bg-slate-50 p-5 text-sm text-slate-500">No events yet.</div>
        ) : (
          events.slice().reverse().map((event) => (
            <div
              key={event.id}
              className={`rounded-3xl border p-4 text-sm ${event.malicious ? "border-warn/30 bg-rose-50" : "border-slate-200 bg-slate-50/90"}`}
            >
              <div className="flex items-start justify-between gap-4">
                <div>
                  <span className="font-semibold text-ink">{event.kind}</span>
                  <div className="mt-1 text-xs uppercase tracking-[0.18em] text-slate-400">{event.messageType || "system event"}</div>
                </div>
                <span className="text-xs text-slate-500">#{event.globalSequence}</span>
              </div>
              <div className="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-slate-600">
                {event.from && <span>from {event.from}</span>}
                {event.to && <span>to {event.to}</span>}
                {event.nodeId && !event.from && <span>node {event.nodeId}</span>}
                {event.value && <span>value={event.value}</span>}
                {event.details && <span>detail={event.details}</span>}
              </div>
            </div>
          ))
        )}
      </div>
    </section>
  );
}
