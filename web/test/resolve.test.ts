import { describe, it, expect } from 'vitest';
import { resolveImports } from '../src/core/resolve.js';
import type { ImportRef } from '../src/core/types.js';

const fl = (...paths: string[]) => paths.map((path) => ({ path }));
const edges = (files: string[], refs: [string, string][]) =>
  resolveImports(
    fl(...files),
    refs.map(([from, spec]): ImportRef => ({ from, spec })),
  ).map((e) => `${e.from} -> ${e.to} (${e.weight})`);

describe('resolveImports: JS/TS', () => {
  const files = ['src/a.ts', 'src/b.tsx', 'src/lib/index.ts', 'src/lib/c.js', 'src/esm.ts', 'top.mjs'];
  it('resolves relative specs with extension and index probing', () => {
    expect(
      edges(files, [
        ['src/a.ts', './b'],
        ['src/a.ts', './lib'],
        ['src/a.ts', './lib/c.js'],
        ['src/lib/c.js', '../../top.mjs'],
        ['src/lib/c.js', '..'], // no src/index.* -> dropped
      ]),
    ).toEqual([
      'src/a.ts -> src/b.tsx (1)',
      'src/a.ts -> src/lib/c.js (1)',
      'src/a.ts -> src/lib/index.ts (1)',
      'src/lib/c.js -> top.mjs (1)',
    ]);
  });
  it('maps TS ESM .js specifiers to .ts sources', () => {
    expect(edges(files, [['src/a.ts', './esm.js']])).toEqual(['src/a.ts -> src/esm.ts (1)']);
  });
  it('drops externals, escaping paths and self-edges; aggregates weight', () => {
    expect(
      edges(files, [
        ['src/a.ts', 'react'],
        ['src/a.ts', '@scope/pkg'],
        ['src/a.ts', '../../../x'],
        ['src/a.ts', './a'],
        ['src/a.ts', './b'],
        ['src/a.ts', './b.tsx'],
        ['src/a.ts', './b'], // duplicate spec counted once
      ]),
    ).toEqual(['src/a.ts -> src/b.tsx (2)']);
  });
});

describe('resolveImports: Python', () => {
  const files = [
    'pkg/__init__.py',
    'pkg/a.py',
    'pkg/sub/__init__.py',
    'pkg/sub/b.py',
    'pkg/sub/deep/c.py',
    'other/a.py',
  ];
  it('resolves dotted relative specs', () => {
    expect(
      edges(files, [
        ['pkg/sub/b.py', '.'],
        ['pkg/sub/b.py', '..a'],
        ['pkg/sub/b.py', '.deep.c'],
        ['pkg/sub/deep/c.py', '...a.Symbol'], // symbol import falls back to module
        ['pkg/a.py', '.sub.b'],
        ['pkg/a.py', '....x'], // escapes root
      ]),
    ).toEqual([
      'pkg/a.py -> pkg/sub/b.py (1)',
      'pkg/sub/b.py -> pkg/a.py (1)',
      'pkg/sub/b.py -> pkg/sub/__init__.py (1)',
      'pkg/sub/b.py -> pkg/sub/deep/c.py (1)',
      'pkg/sub/deep/c.py -> pkg/a.py (1)',
    ]);
  });
  it('resolves absolute dotted specs by unique suffix', () => {
    expect(
      edges(files, [
        ['other/a.py', 'pkg.sub.b'],
        ['other/a.py', 'pkg.sub'],
        ['other/a.py', 'deep.c'],
        ['pkg/sub/b.py', 'a'], // ambiguous (pkg/a.py, other/a.py) -> dropped
        ['pkg/sub/b.py', 'os.path'],
      ]),
    ).toEqual([
      'other/a.py -> pkg/sub/__init__.py (1)',
      'other/a.py -> pkg/sub/b.py (1)',
      'other/a.py -> pkg/sub/deep/c.py (1)',
    ]);
  });
});

describe('resolveImports: Java & Go', () => {
  it('resolves Java imports incl. static and wildcard', () => {
    const files = ['src/main/java/com/x/A.java', 'src/main/java/com/x/util/B.java', 'src/main/java/com/x/C.kt'];
    expect(
      edges(files, [
        ['src/main/java/com/x/A.java', 'com.x.util.B'],
        ['src/main/java/com/x/A.java', 'com.x.C.helper'],
        ['src/main/java/com/x/util/B.java', 'com.x.A.*'],
        ['src/main/java/com/x/util/B.java', 'java.util.List'],
      ]),
    ).toEqual([
      'src/main/java/com/x/A.java -> src/main/java/com/x/C.kt (1)',
      'src/main/java/com/x/A.java -> src/main/java/com/x/util/B.java (1)',
      'src/main/java/com/x/util/B.java -> src/main/java/com/x/A.java (1)',
    ]);
  });
  it('resolves Go package paths to a representative file', () => {
    const files = ['main.go', 'internal/store/store.go', 'internal/store/db.go', 'internal/store/store_test.go', 'pkg/api/handler.go'];
    expect(
      edges(files, [
        ['main.go', 'github.com/me/app/internal/store'],
        ['main.go', 'github.com/me/app/pkg/api'],
        ['main.go', 'fmt'],
        ['main.go', 'net/http'],
      ]),
    ).toEqual([
      'main.go -> internal/store/store.go (1)',
      'main.go -> pkg/api/handler.go (1)',
    ]);
  });
});

describe('resolveImports: project config', () => {
  const run = (
    files: string[],
    refs: [string, string][],
    opts: Parameters<typeof resolveImports>[2],
  ) =>
    resolveImports(
      fl(...files),
      refs.map(([from, spec]): ImportRef => ({ from, spec })),
      opts,
    ).map((e) => `${e.from} -> ${e.to}`);

  it('does not resolve Python stdlib modules to same-named local files', () => {
    expect(
      edges(['app.py', 'utils/json.py', 'utils/helpers.py'], [
        ['app.py', 'json'],
        ['app.py', 'os.path'],
        ['app.py', 'helpers'],
      ]),
    ).toEqual(['app.py -> utils/helpers.py (1)']);
  });

  it('uses go.mod module paths: stdlib net/http is not internal/http', () => {
    const files = ['cmd/main.go', 'internal/http/h.go', 'util/u.go'];
    const refs: [string, string][] = [
      ['cmd/main.go', 'net/http'],
      ['cmd/main.go', 'example.com/m/internal/http'],
      ['cmd/main.go', 'example.com/m/util'],
      ['cmd/main.go', 'github.com/other/util'],
    ];
    expect(run(files, refs, { goModules: [{ dir: '', module: 'example.com/m' }] })).toEqual([
      'cmd/main.go -> internal/http/h.go',
      'cmd/main.go -> util/u.go',
    ]);
  });

  it('resolves tsconfig paths and baseUrl', () => {
    const files = ['src/main.ts', 'src/lib/a.ts', 'src/lib/b/index.ts', 'other/x.ts'];
    const refs: [string, string][] = [
      ['src/main.ts', '@/lib/a'],
      ['src/main.ts', '@/lib/b'],
      ['src/main.ts', 'react'],
      ['other/x.ts', '@/lib/a'],
    ];
    const tsconfigs = [{ dir: '', baseUrl: '.', paths: { '@/*': ['src/*'] } }];
    expect(run(files, refs, { tsconfigs })).toEqual([
      'other/x.ts -> src/lib/a.ts',
      'src/main.ts -> src/lib/a.ts',
      'src/main.ts -> src/lib/b/index.ts',
    ]);
    expect(
      run(files, [['src/main.ts', 'src/lib/a']], { tsconfigs: [{ dir: '', baseUrl: '.' }] }),
    ).toEqual(['src/main.ts -> src/lib/a.ts']);
  });
});
