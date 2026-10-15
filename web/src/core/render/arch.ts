import type { Edge, FileNode, RenderOptions } from '../types.js';
import { capEdges, capNodes, edgeLine, esc } from './util.js';

const DEFAULT_MAX_NODES = 150;
export const ROOT_GROUP = '(root)';

/** Group key: first `depth` directory segments; root-level files -> `(root)`. */
function groupOf(path: string, depth: number): string {
  const dirs = path.split('/').filter(Boolean).slice(0, -1);
  return dirs.length ? dirs.slice(0, Math.max(1, depth)).join('/') : ROOT_GROUP;
}

/** Architecture overview: files collapsed to top-level directories, edges aggregated. */
export function renderArch(edges: Edge[], files: FileNode[], opts: RenderOptions = {}): string {
  const depth = opts.depth ?? 1;
  const maxNodes = opts.maxNodes ?? DEFAULT_MAX_NODES;

  const counts = new Map<string, number>();
  for (const f of files) {
    const g = groupOf(f.path, depth);
    counts.set(g, (counts.get(g) ?? 0) + 1);
  }

  const agg = new Map<string, Edge>();
  for (const e of edges) {
    const from = groupOf(e.from, depth);
    const to = groupOf(e.to, depth);
    if (from === to) continue;
    // Edge endpoints should be scanned files, but guard anyway.
    for (const g of [from, to]) if (!counts.has(g)) counts.set(g, 0);
    const key = `${from}\0${to}`;
    const cur = agg.get(key);
    if (cur) cur.weight += e.weight;
    else agg.set(key, { from, to, weight: e.weight });
  }
  const groupEdges = [...agg.values()].sort(
    (a, b) => a.from.localeCompare(b.from) || a.to.localeCompare(b.to),
  );

  const groups = [...counts.keys()].sort((a, b) =>
    a === ROOT_GROUP ? -1 : b === ROOT_GROUP ? 1 : a.localeCompare(b),
  );
  const capped = capNodes(groups, groupEdges, maxNodes);
  const shown = capEdges(capped.edges);

  const lines = ['flowchart LR'];
  if (capped.omitted > 0) {
    lines.push(`  %% ${capped.omitted} less-connected groups omitted (maxNodes=${maxNodes})`);
  }
  if (shown.omitted > 0) {
    lines.push(`  %% ${shown.omitted} lightest edges omitted (Mermaid renders at most 500)`);
  }
  const ids = new Map<string, string>();
  let n = 0;
  for (const g of groups) {
    if (!capped.kept.has(g)) continue;
    const id = `g${n++}`;
    ids.set(g, id);
    const c = counts.get(g)!;
    const name = g === ROOT_GROUP ? g : `${g}/`;
    lines.push(`  ${id}["${esc(name)}<br/>${c} file${c === 1 ? '' : 's'}"]`);
  }
  for (const e of shown.edges) {
    lines.push(edgeLine(ids.get(e.from)!, ids.get(e.to)!, e.weight, true));
  }
  return lines.join('\n') + '\n';
}
