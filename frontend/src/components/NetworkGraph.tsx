import { useMemo } from "react";
import ReactFlow, { Background, Edge, MarkerType, Node } from "reactflow";
import "reactflow/dist/style.css";
import type { CanonicalEvent, SimulationState } from "../types";

type Props = {
  state: SimulationState | null;
  recentEvents: CanonicalEvent[];
};

export function NetworkGraph({ state, recentEvents }: Props) {
  const { nodes, edges } = useMemo(() => {
    if (!state) {
      return { nodes: [] as Node[], edges: [] as Edge[] };
    }

    const circle = 190;
    const centerX = 260;
    const centerY = 190;
    const nodeViews = state.nodes;

    const graphNodes: Node[] = nodeViews.map((node, index) => {
      const angle = (Math.PI * 2 * index) / Math.max(nodeViews.length, 1);
      const x = centerX + Math.cos(angle) * circle;
      const y = centerY + Math.sin(angle) * circle;
      const active = recentEvents.some((event) => event.from === node.id || event.to === node.id || event.nodeId === node.id);

      return {
        id: node.id,
        position: { x, y },
        data: { label: `${node.id}\n${node.phase}` },
        style: {
          width: 150,
          borderRadius: 24,
          padding: 12,
          whiteSpace: "pre-line",
          textAlign: "center",
          fontWeight: 600,
          border: `2px solid ${node.byzantine ? "#d94841" : node.leader ? "#d8a319" : active ? "#0d7a5f" : "#cbd5e1"}`,
          background: node.byzantine ? "#fff1f1" : "#ffffff",
          boxShadow: active ? "0 0 0 6px rgba(13,122,95,0.12)" : "0 8px 18px rgba(15,23,42,0.07)",
        },
      };
    });

    const edgeMap = new Map<string, Edge>();
    for (const source of nodeViews) {
      for (const target of nodeViews) {
        if (source.id === target.id) {
          continue;
        }
        const flash = recentEvents.some((event) => event.from === source.id && event.to === target.id);
        const id = `${source.id}-${target.id}`;
        edgeMap.set(id, {
          id,
          source: source.id,
          target: target.id,
          type: "smoothstep",
          animated: flash,
          markerEnd: { type: MarkerType.ArrowClosed, color: flash ? "#0d7a5f" : "#94a3b8" },
          style: { stroke: flash ? "#0d7a5f" : "#94a3b8", strokeWidth: flash ? 3 : 1.5 },
        });
      }
    }
    return { nodes: graphNodes, edges: Array.from(edgeMap.values()) };
  }, [recentEvents, state]);

  return (
    <section className="rounded-3xl bg-white p-4 shadow-sm ring-1 ring-slate-200">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-ink">Network Graph</h2>
        <span className="text-sm text-slate-500">Edge flash follows new events</span>
      </div>
      <div className="h-[480px] overflow-hidden rounded-3xl bg-slate-50">
        <ReactFlow nodes={nodes} edges={edges} fitView nodesDraggable={false} nodesConnectable={false} elementsSelectable={false}>
          <Background color="#d6e3dd" gap={24} />
        </ReactFlow>
      </div>
    </section>
  );
}
