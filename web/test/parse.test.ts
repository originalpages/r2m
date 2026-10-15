import { describe, it, expect } from 'vitest';
import { getParser, parsers } from '../src/core/parse/index.js';

function run(path: string, src: string): string[] {
  const p = getParser(path);
  if (!p) throw new Error('no parser for ' + path);
  return p.extract(path, src);
}

describe('getParser', () => {
  it('matches by extension, case-insensitive, never throws', () => {
    expect(parsers.length).toBe(4);
    for (const f of ['a.js', 'a.jsx', 'a.ts', 'a.tsx', 'a.mjs', 'a.cjs', 'x/y.PY', 'a.go', 'A.java']) {
      expect(getParser(f)).toBeDefined();
    }
    expect(getParser('README.md')).toBeUndefined();
    expect(getParser('Makefile')).toBeUndefined();
    expect(getParser('')).toBeUndefined();
  });
});

describe('js/ts', () => {
  it('handles all import forms', () => {
    const src = `
import a from './a';
import * as b from "../b.js";
import { c, d as e } from 'pkg';
import type { T } from './types';
import './side-effect.css';
export * from './re';
export { x } from './re2';
export * as ns from './re3';
const f = require('./cjs');
const g = await import('./dyn');
import def, { z } from '@scope/pkg/sub';
import x = require('./eq');
`;
    expect(run('a.ts', src).sort()).toEqual(
      ['./a', '../b.js', 'pkg', './types', './side-effect.css', './re', './re2', './re3', './cjs', './dyn', '@scope/pkg/sub', './eq'].sort(),
    );
  });

  it('handles multiline imports', () => {
    const src = `import {\n  a,\n  b,\n} from './multi';\nexport {\n  q\n} from './multi2';`;
    expect(run('a.js', src)).toEqual(['./multi', './multi2']);
  });

  it('ignores comments and strings', () => {
    const src = `
// import x from './line';
/* import y from './block';
   require('./block2') */
const s = "import z from './instr'";
const t = \`require('./intpl')\`;
import real from './real';
const r = /import 'x'/;
obj.import('./method');
`;
    expect(run('a.js', src)).toEqual(['./real']);
  });

  it('dedupes and skips dynamic expressions', () => {
    const src = `import a from './a';\nimport('./a');\nrequire(name);\nimport(\`./x/\${n}\`);`;
    expect(run('a.js', src)).toEqual(['./a']);
  });

  it('works with tsx generics/jsx', () => {
    const src = `import React from 'react';\nexport const A = () => <div>import x from 'no'</div>;`;
    expect(run('a.tsx', src)).toContain('react');
  });
});

describe('python', () => {
  it('handles import forms', () => {
    const src = `
import os
import a.b.c
import x as y, z.w as q
from pkg.mod import thing
from .rel import f
from ..up.deep import g
from . import sibling
from .. import (
    one,
    two as t,
)
`;
    const r = run('a.py', src);
    expect(r).toEqual(
      expect.arrayContaining(['os', 'a.b.c', 'x', 'z.w', 'pkg.mod', '.rel', '..up.deep', '.', '.sibling', '..', '..one', '..two']),
    );
  });

  it('ignores comments and docstrings', () => {
    const src = `"""\nimport fake\n"""\n# import nope\nx = "import str"\nimport real  # import trailing\n    \n`;
    expect(run('a.py', src)).toEqual(['real']);
  });

  it('handles indented imports and dedupes', () => {
    const src = `try:\n    import json\nexcept ImportError:\n    import json\n`;
    expect(run('a.py', src)).toEqual(['json']);
  });
});

describe('go', () => {
  it('handles single and grouped imports', () => {
    const src = `package main

import "fmt"
import alias "github.com/x/y"

import (
	"os"
	// "commented"
	str "strings"
	_ "github.com/lib/pq"
	. "testing"
	"example.com/proj/internal/util" // trailing
)

/* import "nope" */
var s = "import \\"fake\\""
`;
    expect(run('a.go', src).sort()).toEqual(
      ['fmt', 'github.com/x/y', 'os', 'strings', 'github.com/lib/pq', 'testing', 'example.com/proj/internal/util'].sort(),
    );
  });
});

describe('java', () => {
  it('returns full dotted names', () => {
    const src = `package a.b;

import java.util.List;
import static org.junit.Assert.assertEquals;
import com.foo.*;
// import ignored.Comment;
/* import ignored.Block; */
import java.util.List;

public class A {}
`;
    expect(run('A.java', src)).toEqual([
      'java.util.List',
      'org.junit.Assert.assertEquals',
      'com.foo.*',
    ]);
  });
});
