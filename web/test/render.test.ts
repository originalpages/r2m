import { describe, it, expect } from 'vitest';
import { renderDeps } from '../src/core/render/deps.js';
import { renderArch } from '../src/core/render/arch.js';
import { generateDiagram } from '../src/core/generate.js';
import type { Edge } from '../src/core/types.js';

const fl = (...paths: string[]) => paths.map((path) => ({ path }));
const nodeLines = (out: string) => out.split('\n').filter((l) => /^\s*n\d+\["/.test(l));

describe('renderDeps', () => {
  const files = fl('src/a.ts', 'src/lib/b.ts', 'src/lib/c"<x>.ts', 'README.md', 'lonely.ts');
  const edges: Edge[] = [
    { from: 'src/a.ts', to: 'src/lib/b.ts', weight: 1 },
    { from: 'src/lib/b.ts', to: 'src/lib/c"<x>.ts', weight: 3 },
  ];
  it('renders nested subgraphs, escaped labels and only connected files', () => {
    const out = renderDeps(edges, files, {});
    expect(out.startsWith('flowchart LR\n')).toBe(true);
    expect(out).toMatch(/subgraph s\d+\["src\/"\]/);
    expect(out).toMatch(/subgraph s\d+\["lib\/"\]/);
    expect(out).toContain('"c#quot;#lt;x#gt;.ts"');
    expect(out).not.toContain('lonely');
    expect(out).not.toContain('README');
    expect(nodeLines(out)).toHaveLength(3);
    expect(out).toMatch(/n\d+ --> n\d+/);
    expect(out).toMatch(/n\d+ -->\|3\| n\d+/);
    expect((out.match(/subgraph/g) ?? []).length).toBe((out.match(/^\s*end$/gm) ?? []).length);
  });
  it('collapses single-child directory chains', () => {
    const out = renderDeps([{ from: 'a/b/c/x.ts', to: 'a/b/c/y.ts', weight: 1 }], [], {});
    expect(out).toContain('["a/b/c/"]');
    expect((out.match(/subgraph/g) ?? []).length).toBe(1);
  });
  it('caps to the most-connected files', () => {
    const hub = 'hub.ts';
    const many: Edge[] = Array.from({ length: 20 }, (_, i) => ({ from: `m/f${i}.ts`, to: hub, weight: 1 }));
    many.push({ from: 'm/f0.ts', to: 'm/f1.ts', weight: 5 });
    const out = renderDeps(many, [], { maxNodes: 3 });
    expect(nodeLines(out)).toHaveLength(3);
    expect(out).toContain('"hub.ts"');
    expect(out).toContain('"f0.ts"');
    expect(out).toContain('"f1.ts"');
    expect(out).toContain('%% 18 less-connected files omitted');
  });
  it('handles no edges', () => {
    expect(renderDeps([], files, {})).toContain('%% no internal dependencies found');
  });
});

describe('renderArch', () => {
  const files = fl('index.ts', 'src/a.ts', 'src/core/b.ts', 'src/core/c.ts', 'lib/d.ts', 'docs/x.md');
  const edges: Edge[] = [
    { from: 'index.ts', to: 'src/a.ts', weight: 1 },
    { from: 'src/a.ts', to: 'lib/d.ts', weight: 1 },
    { from: 'src/core/b.ts', to: 'lib/d.ts', weight: 2 },
    { from: 'src/core/b.ts', to: 'src/core/c.ts', weight: 4 }, // intra-group
  ];
  it('collapses to top-level dirs with counts and weighted edges', () => {
    const out = renderArch(edges, files, {});
    expect(out).toContain('["(root)<br/>1 file"]');
    expect(out).toContain('["src/<br/>3 files"]');
    expect(out).toContain('["lib/<br/>1 file"]');
    const id = (label: string) => new RegExp(`(g\\d+)\\["${label}`).exec(out)![1];
    expect(out).toContain(`${id('src/')} -->|3| ${id('lib/')}`);
    expect(out).toContain(`${id('\\(root\\)')} -->|1| ${id('src/')}`);
    expect(out.match(/-->/g)).toHaveLength(2);
  });
  it('honors depth', () => {
    const out = renderArch(edges, files, { depth: 2 });
    expect(out).toContain('["src/core/<br/>2 files"]');
    expect(out).toContain('["src/<br/>1 file"]');
    expect(out).toContain('-->|2|'); // src/core -> lib
  });
});

describe('generateDiagram', () => {
  const sources: Record<string, string> = {
    'src/index.ts': "import { u } from './lib/util.js';\nimport x from 'react';\n",
    'src/lib/util.ts': "export const u = 1;\n",
    'app/main.py': 'from .helpers import h\n',
    'app/helpers.py': 'h = 1\n',
    'README.md': '# hi',
  };
  const files = fl(...Object.keys(sources), 'broken.ts');
  const read = async (p: string) => {
    if (!(p in sources)) throw new Error('ENOENT');
    return sources[p];
  };
  it('tree', async () => {
    expect(await generateDiagram('tree', files, read)).toContain('flowchart LR');
  });
  it('deps (skips unreadable files)', async () => {
    const out = await generateDiagram('deps', files, read);
    expect(out).toContain('"index.ts"');
    expect(out).toContain('"util.ts"');
    expect(out).toContain('"main.py"');
    expect(out).toContain('"helpers.py"');
    expect(out.match(/-->/g)).toHaveLength(2);
  });
  it('arch', async () => {
    const out = await generateDiagram('arch', files, read);
    expect(out).toContain('["src/<br/>2 files"]');
    expect(out).not.toMatch(/-->/); // all edges are intra-group
  });
});

describe('Mermaid safety', () => {
  const edgeCount = (out: string) => out.split('\n').filter((l) => /-->/.test(l)).length;

  it('escapes #, &, quotes and backticks in labels', async () => {
    const out = await generateDiagram('tree', fl('`h`.ts', 'a#1;.ts', 'x&amp;y.ts'), async () => '');
    expect(out).not.toMatch(/`/);
    expect(out).toContain('#96;h#96;.ts');
    expect(out).toContain('a#35;1;.ts');
    expect(out).toContain('x#38;amp;y.ts');
  });

  it('never emits more than 500 edges', () => {
    const files = fl(...Array.from({ length: 150 }, (_, i) => `f${i}.ts`));
    const edges: Edge[] = [];
    for (let i = 0; i < 150; i++) {
      for (let j = 1; j <= 5; j++) {
        edges.push({ from: `f${i}.ts`, to: `f${(i + j) % 150}.ts`, weight: 1 + (i % 3) });
      }
    }
    const deps = renderDeps(edges, files, { maxNodes: 150 });
    expect(edgeCount(deps)).toBe(500);
    expect(deps).toContain('250 lightest edges omitted');
  });
});
