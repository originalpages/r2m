import type { Edge, FileNode, RenderOptions } from '../types.js';
import { capEdges, capNodes, edgeLine, esc } from './util.js';

const DEFAULT_MAX_NODES = 150;

interface Dir {
  name: string;
  dirs: Map<string, Dir>;
  files: string[]; // full paths
}

function build(paths: string[]): Dir {
  const root: Dir = { name: '', dirs: new Map(), files: [] };
  for (const p of paths) {
    const parts = p.split('/').filter(Boolean);
    let cur = root;
    for (const part of parts.slice(0, -1)) {
      let next = cur.dirs.get(part);
      if (!next) cur.dirs.set(part, (next = { name: part, dirs: new Map(), files: [] }));
      cur = next;
    }
    cur.files.push(p);
  }
  return root;
}

/** File→file dependency graph, clustered by directory. Only files with edges are shown. */
export function renderDeps(edges: Edge[], _files: FileNode[], opts: RenderOptions = {}): string {
  const maxNodes = opts.maxNodes ?? DEFAULT_MAX_NODES;
  const nodes = [...new Set(edges.flatMap((e) => [e.from, e.to]))].sort();
  const capped = capNodes(nodes, edges, maxNodes);
  const shown = capEdges(capped.edges);

  const lines = ['flowchart LR'];
  if (!nodes.length) {
    lines.push('  %% no internal dependencies found');
    return lines.join('\n') + '\n';
  }
  if (capped.omitted > 0) {
    lines.push(`  %% ${capped.omitted} less-connected files omitted (maxNodes=${maxNodes})`);
  }

  if (shown.omitted > 0) {
    lines.push(`  %% ${shown.omitted} lightest edges omitted (Mermaid renders at most 500)`);
  }

  const ids = new Map<string, string>();
  let n = 0;
  let s = 0;

  function emit(d: Dir, indent: string): void {
    for (const c of [...d.dirs.values()].sort((a, b) => a.name.localeCompare(b.name))) {
      // Collapse single-child chains (a/b/c with no files in a, b) into one subgraph.
      let label = c.name;
      let cur = c;
      while (cur.files.length === 0 && cur.dirs.size === 1) {
        cur = cur.dirs.values().next().value!;
        label += `/${cur.name}`;
      }
      lines.push(`${indent}subgraph s${s++}["${esc(label)}/"]`);
      emit(cur, indent + '  ');
      lines.push(`${indent}end`);
    }
    for (const f of [...d.files].sort()) {
      const id = `n${n++}`;
      ids.set(f, id);
      lines.push(`${indent}${id}["${esc(f.slice(f.lastIndexOf('/') + 1))}"]`);
    }
  }

  emit(build([...capped.kept].sort()), '  ');
  for (const e of shown.edges) {
    lines.push(edgeLine(ids.get(e.from)!, ids.get(e.to)!, e.weight));
  }
  return lines.join('\n') + '\n';
}
