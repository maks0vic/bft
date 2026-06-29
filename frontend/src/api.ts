import type { EventsResponse, SimulationState } from "./types";

const API_BASE = "/api";

export type StartSimulationInput = {
  value: string;
  nodeCount: number;
  byzantineCount: number;
  byzantineBehavior: string;
};

export async function fetchState(): Promise<SimulationState> {
  const response = await fetch(`${API_BASE}/state`);
  if (!response.ok) {
    throw new Error(`Failed to fetch state: ${response.status}`);
  }
  return response.json();
}

export async function fetchEvents(since: number): Promise<EventsResponse> {
  const url = new URL(`${window.location.origin}${API_BASE}/events`);
  if (since > 0) {
    url.searchParams.set("since", String(since));
  }
  const response = await fetch(url.pathname + url.search);
  if (!response.ok) {
    throw new Error(`Failed to fetch events: ${response.status}`);
  }
  return response.json();
}

export async function startSimulation(input: StartSimulationInput): Promise<void> {
  const response = await fetch(`${API_BASE}/start`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    const detail = await response.text();
    throw new Error(detail || `Failed to start simulation: ${response.status}`);
  }
}

export async function resetSimulation(): Promise<void> {
  const response = await fetch(`${API_BASE}/reset`, { method: "POST" });
  if (!response.ok) {
    throw new Error(`Failed to reset simulation: ${response.status}`);
  }
}
