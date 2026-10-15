import type { Parser } from '../types.js';

const MARK = '\u0001';

/**
 * Replace comments with whitespace and string literals with `\u0001N\u0001`
 * placeholders so that later regexes cannot match inside strings/comments.
 * Returns the cleaned code and the string table (null = template with `${}`).
 */
function mask(src: string): { code: string; strings: (string | null)[] } {
  const strings: (string | null)[] = [];
  let out = '';
  let prev = ''; // last significant char emitted (for regex-literal heuristic)
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
    if (c === '"' || c === "'" || c === '`') {
      let j = i + 1;
      let val = '';
      let dynamic = false;
      while (j < n && src[j] !== c) {
        if (src[j] === '\\') {
          val += src[j + 1] ?? '';
          j += 2;
          continue;
        }
        if (c === '`' && src[j] === '$' && src[j + 1] === '{') dynamic = true;
        if (c !== '`' && src[j] === '\n') break; // unterminated
        val += src[j];
        j++;
      }
      strings.push(dynamic ? null : val);
      out += `${MARK}${strings.length - 1}${MARK}`;
      prev = '"';
      i = j + 1;
      continue;
    }
    if (c === '/' && (prev === '' || '(,=:[!&|?{};+-*%<>~^'.includes(prev))) {
      // regex literal
      let j = i + 1;
      let inClass = false;
      while (j < n && src[j] !== '\n') {
        if (src[j] === '\\') j += 2;
        else {
          if (src[j] === '[') inClass = true;
          else if (src[j] === ']') inClass = false;
          else if (src[j] === '/' && !inClass) break;
          j++;
        }
      }
      if (j < n && src[j] === '/') {
        out += ' ';
        prev = ')';
        i = j + 1;
        continue;
      }
    }
    out += c;
    if (!/\s/.test(c)) prev = c;
    i++;
  }
  return { code: out, strings };
}

const S = `${MARK}(\\d+)${MARK}`;
const BOUNDARY = '(?<![.\\w$])';
const PATTERNS: RegExp[] = [
  // import x, { y } from 'm' / import type {..} from 'm' / import * as ns from 'm'
  new RegExp(`${BOUNDARY}import\\s+(?:type\\s+|typeof\\s+)?[^;${MARK}(]*?\\bfrom\\s*${S}`, 'g'),
  // import 'side-effect'
  new RegExp(`${BOUNDARY}import\\s*${S}`, 'g'),
  // export * from 'm' / export { a } from 'm' / export type {..} from 'm'
  new RegExp(
    `${BOUNDARY}export\\s+(?:type\\s+)?(?:\\*(?:\\s*as\\s+[\\w$]+)?|\\{[^}]*\\})\\s*from\\s*${S}`,
    'g',
  ),
  // require('m') and import('m')
  new RegExp(`${BOUNDARY}require\\s*\\(\\s*${S}\\s*[,)]`, 'g'),
  new RegExp(`${BOUNDARY}import\\s*\\(\\s*${S}`, 'g'),
];

export const jsParser: Parser = {
  exts: ['js', 'jsx', 'ts', 'tsx', 'mjs', 'cjs'],
  extract(_path, source) {
    const { code, strings } = mask(source);
    const found: { idx: number; spec: string }[] = [];
    for (const re of PATTERNS) {
      re.lastIndex = 0;
      for (const m of code.matchAll(re)) {
        const spec = strings[Number(m[1])];
        if (spec) found.push({ idx: m.index ?? 0, spec });
      }
    }
    found.sort((a, b) => a.idx - b.idx);
    return [...new Set(found.map((f) => f.spec))];
  },
};
