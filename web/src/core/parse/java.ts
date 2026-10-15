import type { Parser } from '../types.js';
import { stripCComments } from './go.js';

export const javaParser: Parser = {
  exts: ['java'],
  extract(_path, source) {
    const code = stripCComments(source);
    const out = new Set<string>();
    const re = /^[ \t]*import\s+(?:static\s+)?([\w$]+(?:\s*\.\s*(?:[\w$]+|\*))*)\s*;/gm;
    for (const m of code.matchAll(re)) out.add(m[1].replace(/\s+/g, ''));
    return [...out];
  },
};
