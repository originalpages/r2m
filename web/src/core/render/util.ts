import type { Edge } from '../types.js';

/**
 * Escape text for use inside a double-quoted Mermaid label. `#` must go first
 * (it introduces entity codes); backticks would otherwise start a markdown string.
 */
export function esc(s: string): string {
  return s
    .replace(/#/g, '#35;')
    .replace(/&/g, '#38;')
    .replace(/"/g, '#quot;')
    .replace(/`/g, '#96;')
    .replace(/</g, '#lt;')
    .replace(/>/g, '#gt;');
}

/** Mermaid refuses to render more than 500 edges by default (GitHub, mermaid.live, the web page). */
export const MAX_EDGES = 500;

/** Keep the `max` heaviest edges (ties by name). Returns the kept edges in their original order. */
export function capEdges(edges: Edge[], max = MAX_EDGES): { edges: Edge[]; omitted: number } {
  if (edges.length <= max) return { edges, omitted: 0 };
  const ranked = [...edges].sort(
    (a, b) =>
      b.weight - a.weight || a.from.localeCompare(b.from) || a.to.localeCompare(b.to),
  );
  const keep = new Set(ranked.slice(0, max));
  return { edges: edges.filter((e) => keep.has(e)), omitted: edges.length - max };
}

/**
 * Keep the `max` most-connected nodes (by summed edge weight, ties by name).
 * Returns the kept node set and the edges between kept nodes.
 */
export function capNodes(
  nodes: string[],
  edges: Edge[],
  max: number,
): { kept: Set<string>; edges: Edge[]; omitted: number } {
  if (nodes.length <= max) return { kept: new Set(nodes), edges, omitted: 0 };
  const degree = new Map<string, number>();
  for (const e of edges) {
    degree.set(e.from, (degree.get(e.from) ?? 0) + e.weight);
    degree.set(e.to, (degree.get(e.to) ?? 0) + e.weight);
  }
  const ranked = [...nodes].sort(
    (a, b) => (degree.get(b) ?? 0) - (degree.get(a) ?? 0) || a.localeCompare(b),
  );
  const kept = new Set(ranked.slice(0, Math.max(0, max)));
  return {
    kept,
    edges: edges.filter((e) => kept.has(e.from) && kept.has(e.to)),
    omitted: nodes.length - kept.size,
  };
}

export function edgeLine(from: string, to: string, weight: number, alwaysLabel = false): string {
  return weight > 1 || alwaysLabel ? `  ${from} -->|${weight}| ${to}` : `  ${from} --> ${to}`;
}
