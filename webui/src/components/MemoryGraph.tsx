import cytoscape, { type Core } from "cytoscape";
import { useEffect, useRef } from "react";
import type { EvidenceState, GraphNode, GraphView } from "../types";

const colors: Record<EvidenceState, string> = {
  candidate: "#d9a441", read: "#4e7ecb", used: "#3f8d6a", dismissed: "#9b9992", unknown: "#8f887d",
};

function kindColor(kind: string) {
  if (kind === "Self") return "#334e48";
  if (kind === "Person") return "#a8684a";
  if (kind === "Episode") return "#c4aa77";
  return "#8d968e";
}

export function MemoryGraph({ view, activation = new Map(), onSelect }: {
  view?: GraphView;
  activation?: Map<string, EvidenceState>;
  onSelect?: (node: GraphNode) => void;
}) {
  const host = useRef<HTMLDivElement>(null);
  const graph = useRef<Core | undefined>(undefined);
  useEffect(() => {
    if (!host.current || !view?.available) return;
    graph.current?.destroy();
    const elements = [
      ...view.nodes.map((node) => ({ data: { ...node, label: node.title || node.label || node.id, activeState: activation.get(node.id) || "unknown" } })),
      ...view.edges.map((edge, index) => ({ data: { ...edge, id: edge.id || `edge-${index}`, label: edge.kind || edge.label || "" } })),
    ];
    const cy = cytoscape({
      container: host.current,
      elements,
      wheelSensitivity: 0.18,
      minZoom: 0.35,
      maxZoom: 2.4,
      style: [
        { selector: "node", style: {
          "background-color": (element) => activation.has(element.id()) ? colors[activation.get(element.id())!] : kindColor(String(element.data("kind") || "")),
          "border-color": (element) => activation.has(element.id()) ? colors[activation.get(element.id())!] : "#f8f5ed",
          "border-width": (element) => activation.has(element.id()) ? 5 : 2,
          "border-opacity": 0.55,
          "label": "data(label)", "font-size": 11, "font-family": "ui-sans-serif, system-ui", "color": "#3d3a35",
          "text-valign": "bottom", "text-margin-y": 9, "text-wrap": "ellipsis", "text-max-width": "110px",
          "width": 28, "height": 28, "shape": "ellipse",
        } },
        { selector: 'node[kind = "Self"]', style: { "width": 48, "height": 48 } },
        { selector: 'node[kind = "Person"]', style: { "width": 42, "height": 42, "shape": "round-rectangle" } },
        { selector: 'node[kind = "Episode"]', style: { "shape": "diamond" } },
        { selector: "edge", style: {
          "width": 1.2, "line-color": "#c7c1b4", "target-arrow-color": "#c7c1b4", "target-arrow-shape": "triangle",
          "curve-style": "bezier", "opacity": 0.62, "label": "data(label)", "font-size": 8, "color": "#8a857c", "text-background-color": "#f5f2e9", "text-background-opacity": 0.8,
        } },
        { selector: ":selected", style: { "overlay-color": "#8d6f4d", "overlay-opacity": 0.12, "overlay-padding": 10 } },
      ],
      layout: { name: "cose", animate: false, padding: 44, nodeRepulsion: () => 6500, idealEdgeLength: () => 95 },
    });
    cy.on("tap", "node", (event) => onSelect?.(event.target.data() as GraphNode));
    graph.current = cy;
    return () => cy.destroy();
  }, [view, activation, onSelect]);
  if (!view?.available) return <div className="graph-placeholder"><span>⟡</span><strong>长期图谱暂时不可用</strong><p>对话仍然可以继续，记忆会在图存储恢复后重新显现。</p></div>;
  return <div className="memory-graph-canvas" ref={host} role="img" aria-label="长期记忆关系星图" />;
}
