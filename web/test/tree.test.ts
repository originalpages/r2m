import { describe, it, expect } from 'vitest';
import { renderTree } from '../src/core/render/tree.js';

describe('renderTree', () => {
  const files = [{ path: 'a/b/c.ts' }, { path: 'a/d.ts' }, { path: 'x"<y>.md' }];
  it('renders flowchart with safe ids and escaped labels', () => {
    const out = renderTree(files, {});
    expect(out.startsWith('flowchart LR')).toBe(true);
    expect(out).toContain('n0["."]');
    expect(out).toContain('"x#quot;#lt;y#gt;.md"');
  });
  it('respects depth', () => {
    const out = renderTree(files, { depth: 1 });
    expect(out).toContain('…(2 files)');
    expect(out).not.toContain('c.ts');
  });
  it('auto-collapses over maxNodes', () => {
    const many = Array.from({ length: 50 }, (_, i) => ({ path: `d/s/f${i}.ts` }));
    const out = renderTree(many, { maxNodes: 10 });
    expect(out).toContain('…(50 files)');
  });
});
