import type { Parser } from '../types.js';

/** Blank out comments and string literals (incl. triple-quoted docstrings). */
function strip(src: string): string {
  let out = '';
  const n = src.length;
  let i = 0;
  while (i < n) {
    const c = src[i];
    if (c === '#') {
      while (i < n && src[i] !== '\n') i++;
      continue;
    }
    if (c === '"' || c === "'") {
      const triple = src.startsWith(c.repeat(3), i);
      const q = triple ? c.repeat(3) : c;
      let j = i + q.length;
      while (j < n && !src.startsWith(q, j)) {
        if (src[j] === '\\') j++;
        else if (!triple && src[j] === '\n') break;
        j++;
      }
      const stop = Math.min(n, j + q.length);
      out += src.slice(i, stop).replace(/[^\n]/g, ' ');
      i = stop;
      continue;
    }
    out += c;
    i++;
  }
  return out;
}

const NAME = '[A-Za-z_]\\w*';
const DOTTED = `${NAME}(?:\\s*\\.\\s*${NAME})*`;

export const pythonParser: Parser = {
  exts: ['py'],
  extract(_path, source) {
    // join backslash continuations, split `;` statements
    const code = strip(source).replace(/\\\r?\n/g, ' ').replace(/;/g, '\n');
    const out = new Set<string>();

    for (const m of code.matchAll(/^[ \t]*import[ \t]+([^\n]+)/gm)) {
      for (const part of m[1].split(',')) {
        const name = part.trim().split(/\s+as\s+/)[0].replace(/\s+/g, '');
        if (new RegExp(`^${DOTTED}$`).test(name)) out.add(name);
      }
    }

    const fromRe = new RegExp(
      `^[ \\t]*from[ \\t]+(\\.*(?:${DOTTED})?)[ \\t]*import\\b[ \\t]*(\\([^)]*\\)|[^\\n]*)`,
      'gm',
    );
    for (const m of code.matchAll(fromRe)) {
      const mod = m[1].replace(/\s+/g, '');
      if (!mod) continue;
      out.add(mod);
      if (/^\.+$/.test(mod)) {
        // `from . import x`: x may be a submodule; emit `.x` candidates too
        const names = m[2].replace(/[()]/g, ' ').split(',');
        for (const part of names) {
          const nm = part.trim().split(/\s+as\s+/)[0];
          if (new RegExp(`^${NAME}$`).test(nm)) out.add(mod + nm);
        }
      }
    }
    return [...out];
  },
};
