import { useEffect, useState } from "react";
import { fetchState } from "../api";
import type { SimulationState } from "../types";

export function useStatePoll(refreshKey: number) {
  const [state, setState] = useState<SimulationState | null>(null);
  const [error, setError] = useState<string>("");

  useEffect(() => {
    let cancelled = false;

    async function poll() {
      try {
        const next = await fetchState();
        if (!cancelled) {
          setState(next);
          setError("");
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Unknown state error");
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

  return { state, error, setState };
}
