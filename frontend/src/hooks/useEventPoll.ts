import { useEffect, useState } from "react";
import { fetchEvents } from "../api";
import type { CanonicalEvent, EventsResponse } from "../types";

export function useEventPoll(refreshKey: number) {
  const [events, setEvents] = useState<CanonicalEvent[]>([]);
  const [eventsByNode, setEventsByNode] = useState<EventsResponse["eventsByNode"]>({});
  const [lastSequence, setLastSequence] = useState(0);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    let currentLast = 0;

    setEvents([]);
    setEventsByNode({});
    setLastSequence(0);

    async function poll() {
      try {
        const response = await fetchEvents(currentLast);
        if (cancelled) {
          return;
        }
        if (response.events.length > 0) {
          setEvents((prev) => [...prev, ...response.events]);
        }
        setEventsByNode(response.eventsByNode);
        currentLast = response.lastSequence;
        setLastSequence(response.lastSequence);
        setError("");
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Unknown event error");
        }
      }
    }

    poll();
    const timer = window.setInterval(poll, 1000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [refreshKey]);

  return { events, eventsByNode, lastSequence, error, setEvents, setEventsByNode, setLastSequence };
}
