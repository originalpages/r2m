import type { DiagramType, FileNode, ImportRef, RenderOptions, ResolveOptions } from './types.js';
import { getParser } from './parse/index.js';
import { resolveImports } from './resolve.js';
import { renderTree } from './render/tree.js';
import { renderDeps } from './render/deps.js';
import { renderArch } from './render/arch.js';

const CONCURRENCY = 16;

/** Basenames of project config files that affect import resolution. */
export const CONFIG_FILES = ['go.mod', 'tsconfig.json', 'jsconfig.json'];

const dirOf = (p: string): string => p.slice(0, Math.max(0, p.lastIndexOf('/')));
const baseOf = (p: string): string => p.slice(p.lastIndexOf('/') + 1);

/** JSON with comments and trailing commas (tsconfig flavour). Comment-like text in strings is kept. */
function parseJsonc(text: string): unknown {
  const noComments = text.replace(
    /("(?:\\.|[^"\\])*")|\/\/[^\n]*|\/\*[\s\S]*?\*\//g,
    (_m, str: string | undefined) => str ?? '',
  );
  return JSON.parse(noComments.replace(/,(\s*[}\]])/g, '$1'));
}

async function loadResolveOptions(
  files: FileNode[],
  readFile: (path: string) => Promise<string>,
): Promise<ResolveOptions> {
  const opts: ResolveOptions = { goModules: [], tsconfigs: [] };
  for (const { path } of files) {
    const name = baseOf(path);
    if (!CONFIG_FILES.includes(name)) continue;
    try {
      const text = await readFile(path);
      if (name === 'go.mod') {
        const m = /^[ \t]*module[ \t]+"?([^\s"]+)"?/m.exec(text);
        if (m) opts.goModules!.push({ dir: dirOf(path), module: m[1] });
      } else {
        const co = ((parseJsonc(text) as { compilerOptions?: Record<string, unknown> } | null)
          ?.compilerOptions ?? {}) as { baseUrl?: unknown; paths?: unknown };
        const paths: Record<string, string[]> = {};
        if (co.paths && typeof co.paths === 'object') {
          for (const [k, v] of Object.entries(co.paths)) {
            if (Array.isArray(v)) paths[k] = v.filter((x): x is string => typeof x === 'string');
          }
        }
        opts.tsconfigs!.push({
          dir: dirOf(path),
          baseUrl: typeof co.baseUrl === 'string' ? co.baseUrl : undefined,
          paths,
        });
      }
    } catch {
      // unreadable or malformed config: fall back to heuristics
    }
  }
  return opts;
}

/**
 * Environment-agnostic entry point (no node:* imports): the caller supplies `readFile`,
 * e.g. fs-based in the CLI or fetch-based in the browser.
 */
export async function generateDiagram(
  type: DiagramType,
  files: FileNode[],
  readFile: (path: string) => Promise<string>,
  opts: RenderOptions = {},
): Promise<string> {
  if (type === 'tree') return renderTree(files, opts);
  if (type !== 'deps' && type !== 'arch') throw new Error(`Unknown diagram type: ${String(type)}`);

  const parsable = files
    .map((f) => ({ path: f.path, parser: getParser(f.path) }))
    .filter((x) => x.parser !== undefined);

  const refs: ImportRef[] = [];
  let next = 0;
  const worker = async (): Promise<void> => {
    while (next < parsable.length) {
      const { path, parser } = parsable[next++];
      try {
        const source = await readFile(path);
        for (const spec of parser!.extract(path, source)) refs.push({ from: path, spec });
      } catch {
        // unreadable / unparsable file: skip
      }
    }
  };
  await Promise.all(Array.from({ length: Math.min(CONCURRENCY, parsable.length) }, worker));

  const edges = resolveImports(files, refs, await loadResolveOptions(files, readFile));
  return type === 'deps' ? renderDeps(edges, files, opts) : renderArch(edges, files, opts);
}
