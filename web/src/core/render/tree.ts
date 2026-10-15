import type { FileNode, RenderOptions } from '../types.js';
import { esc } from './util.js';

interface Dir {
  name: string;
  dirs: Map<string, Dir>;
  files: string[];
}

const DEFAULT_MAX_NODES = 200;

function countFiles(d: Dir): number {
  let n = d.files.length;
  for (const c of d.dirs.values()) n += countFiles(c);
  return n;
}

function build(files: FileNode[]): Dir {
  const root: Dir = { name: '.', dirs: new Map(), files: [] };
  for (const f of files) {
    const parts = f.path.split('/').filter(Boolean);
    if (!parts.length) continue;
    let cur = root;
    for (const part of parts.slice(0, -1)) {
      let next = cur.dirs.get(part);
      if (!next) {
        next = { name: part, dirs: new Map(), files: [] };
        cur.dirs.set(part, next);
      }
      cur = next;
    }
    cur.files.push(parts[parts.length - 1]);
  }
  return root;
}

/** Number of nodes emitted if directories deeper than `limit` are collapsed. */
function nodeCount(d: Dir, level: number, limit: number): number {
  let n = 1;
  if (level >= limit) return n + (countFiles(d) > 0 ? 1 : 0);
  n += d.files.length;
  for (const c of d.dirs.values()) n += nodeCount(c, level + 1, limit);
  return n;
}

export function renderTree(files: FileNode[], opts: RenderOptions = {}): string {
  const maxNodes = opts.maxNodes ?? DEFAULT_MAX_NODES;
  const root = build(files);

  let limit = opts.depth ?? Infinity;
  if (nodeCount(root, 0, limit) > maxNodes) {
    // auto-collapse: shrink depth until under cap (min 1)
    let deepest = 0;
    const measure = (d: Dir, l: number): void => {
      deepest = Math.max(deepest, l);
      for (const c of d.dirs.values()) measure(c, l + 1);
    };
    measure(root, 0);
    limit = Math.min(limit, deepest);
    while (limit > 1 && nodeCount(root, 0, limit) > maxNodes) limit--;
  }

  const lines = ['flowchart LR'];
  let id = 0;
  const next = (): string => `n${id++}`;

  function emit(d: Dir, level: number, myId: string): void {
    if (level >= limit) {
      const n = countFiles(d);
      if (n > 0) {
        const c = next();
        lines.push(`  ${c}["…(${n} file${n === 1 ? '' : 's'})"]`);
        lines.push(`  ${myId} --> ${c}`);
      }
      return;
    }
    for (const c of [...d.dirs.values()].sort((a, b) => a.name.localeCompare(b.name))) {
      const cid = next();
      lines.push(`  ${cid}["${esc(c.name)}/"]`);
      lines.push(`  ${myId} --> ${cid}`);
      emit(c, level + 1, cid);
    }
    for (const f of [...d.files].sort()) {
      const fid = next();
      lines.push(`  ${fid}["${esc(f)}"]`);
      lines.push(`  ${myId} --> ${fid}`);
    }
  }

  const rootId = next();
  lines.push(`  ${rootId}["${esc(root.name)}"]`);
  emit(root, 0, rootId);
  return lines.join('\n') + '\n';
}
