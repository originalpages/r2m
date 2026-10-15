import type { Parser } from '../types.js';
import { jsParser } from './js.js';
import { pythonParser } from './python.js';
import { goParser } from './go.js';
import { javaParser } from './java.js';

/** Registry of parsers by language. */
export const parsers: Parser[] = [jsParser, pythonParser, goParser, javaParser];

/** Returns the parser matching the file extension, or undefined. Never throws. */
export function getParser(path: string): Parser | undefined {
  const base = String(path ?? '').split(/[\\/]/).pop() ?? '';
  const dot = base.lastIndexOf('.');
  if (dot < 0) return undefined;
  const ext = base.slice(dot + 1).toLowerCase();
  return parsers.find((p) => p.exts.includes(ext));
}
