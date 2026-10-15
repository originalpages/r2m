import type { Parser } from '../types.js';

/** Blank out `//` and `/* *​/` comments while leaving string literals intact. */
export function stripCComments(src: string): string {
  let out = '';
  const n = src.length;
  let i = 0;
  while (i < n) {
    const c = src[i];
    const d = src[i + 1];
    if (c === '/' && d === '/') {
      while (i < n && src[i] !== '\n') i++;
      continue;
    }
    if (c === '/' && d === '*') {
      const end = src.indexOf('*/', i + 2);
      const stop = end === -1 ? n : end + 2;
      out += src.slice(i, stop).replace(/[^\n]/g, ' ');
      i = stop;
      continue;
    }
    if (c === '"' || c === '`' || c === "'") {
      let j = i + 1;
      while (j < n && src[j] !== c) {
        if (src[j] === '\\' && c !== '`') j++;
        else if (src[j] === '\n' && c !== '`') break;
        j++;
      }
      out += src.slice(i, j + 1);
      i = j + 1;
      continue;
    }
    out += c;
    i++;
  }
  return out;
}

const SPEC = String.raw`(?:[\w.]+\s+)?(?:"([^"\n]+)"|\`([^\`\n]+)\`)`;

export const goParser: Parser = {
  exts: ['go'],
  extract(_path, source) {
    const code = stripCComments(source);
    const out = new Set<string>();
    const add = (m: RegExpMatchArray) => {
      const s = m[1] ?? m[2];
      if (s) out.add(s);
    };
    // grouped: import ( ... )
    for (const blk of code.matchAll(/^[ \t]*import\s*\(([^)]*)\)/gm)) {
      for (const line of blk[1].split(/[\n;]/)) {
        const m = line.trim().match(new RegExp(`^${SPEC}`));
        if (m) add(m);
      }
    }
    // single: import [alias] "path"
    for (const m of code.matchAll(new RegExp(`^[ \\t]*import\\s+${SPEC}`, 'gm'))) add(m);
    return [...out];
  },
};
