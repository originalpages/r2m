import type { Edge, FileNode, ImportRef, ResolveOptions } from './types.js';

// Environment-agnostic posix path helpers (no node:path so the web build can use this).

function dirname(p: string): string {
  const i = p.lastIndexOf('/');
  return i < 0 ? '' : p.slice(0, i);
}

function extname(p: string): string {
  const base = p.slice(p.lastIndexOf('/') + 1);
  const i = base.lastIndexOf('.');
  return i <= 0 ? '' : base.slice(i).toLowerCase();
}

/** Join + normalize; returns undefined if the path escapes the root. */
function normalize(p: string): string | undefined {
  const out: string[] = [];
  for (const seg of p.split('/')) {
    if (!seg || seg === '.') continue;
    if (seg === '..') {
      if (!out.length) return undefined;
      out.pop();
    } else out.push(seg);
  }
  return out.join('/');
}

const JS_EXTS = ['.ts', '.tsx', '.js', '.jsx', '.mjs', '.cjs', '.mts', '.cts'];
const PROBE_EXTS = ['.ts', '.tsx', '.js', '.jsx', '.mjs', '.cjs', '.py'];
const JVM_EXTS = ['.java', '.kt', '.kts', '.scala', '.groovy'];

/** Python standard library top-level modules (3.12): never resolved to local files. */
const PY_STDLIB = new Set([
  'abc', 'aifc', 'antigravity', 'argparse', 'array', 'ast', 'asyncio', 'atexit', 'audioop',
  'base64', 'bdb', 'binascii', 'bisect', 'builtins', 'bz2', 'cProfile', 'calendar', 'cgi',
  'cgitb', 'chunk', 'cmath', 'cmd', 'code', 'codecs', 'codeop', 'collections', 'colorsys',
  'compileall', 'concurrent', 'configparser', 'contextlib', 'contextvars', 'copy', 'copyreg',
  'crypt', 'csv', 'ctypes', 'curses', 'dataclasses', 'datetime', 'dbm', 'decimal', 'difflib',
  'dis', 'doctest', 'email', 'encodings', 'ensurepip', 'enum', 'errno', 'faulthandler',
  'fcntl', 'filecmp', 'fileinput', 'fnmatch', 'fractions', 'ftplib', 'functools', 'gc',
  'genericpath', 'getopt', 'getpass', 'gettext', 'glob', 'graphlib', 'grp', 'gzip', 'hashlib',
  'heapq', 'hmac', 'html', 'http', 'idlelib', 'imaplib', 'imghdr', 'importlib', 'inspect',
  'io', 'ipaddress', 'itertools', 'json', 'keyword', 'lib2to3', 'linecache', 'locale',
  'logging', 'lzma', 'mailbox', 'mailcap', 'marshal', 'math', 'mimetypes', 'mmap',
  'modulefinder', 'msilib', 'msvcrt', 'multiprocessing', 'netrc', 'nis', 'nntplib', 'nt',
  'ntpath', 'nturl2path', 'numbers', 'opcode', 'operator', 'optparse', 'os', 'ossaudiodev',
  'pathlib', 'pdb', 'pickle', 'pickletools', 'pipes', 'pkgutil', 'platform', 'plistlib',
  'poplib', 'posix', 'posixpath', 'pprint', 'profile', 'pstats', 'pty', 'pwd', 'py_compile',
  'pyclbr', 'pydoc', 'pydoc_data', 'pyexpat', 'queue', 'quopri', 'random', 're', 'readline',
  'reprlib', 'resource', 'rlcompleter', 'runpy', 'sched', 'secrets', 'select', 'selectors',
  'shelve', 'shlex', 'shutil', 'signal', 'site', 'smtplib', 'sndhdr', 'socket', 'socketserver',
  'spwd', 'sqlite3', 'sre_compile', 'sre_constants', 'sre_parse', 'ssl', 'stat', 'statistics',
  'string', 'stringprep', 'struct', 'subprocess', 'sunau', 'symtable', 'sys', 'sysconfig',
  'syslog', 'tabnanny', 'tarfile', 'telnetlib', 'tempfile', 'termios', 'textwrap', 'this',
  'threading', 'time', 'timeit', 'tkinter', 'token', 'tokenize', 'tomllib', 'trace',
  'traceback', 'tracemalloc', 'tty', 'turtle', 'turtledemo', 'types', 'typing', 'unicodedata',
  'unittest', 'urllib', 'uu', 'uuid', 'venv', 'warnings', 'wave', 'weakref', 'webbrowser',
  'winreg', 'winsound', 'wsgiref', 'xdrlib', 'xml', 'xmlrpc', 'zipapp', 'zipfile', 'zipimport',
  'zlib', 'zoneinfo',
]);

type Family = 'js' | 'py' | 'jvm' | 'go' | 'other';

function family(path: string): Family {
  const e = extname(path);
  if (JS_EXTS.includes(e)) return 'js';
  if (e === '.py') return 'py';
  if (JVM_EXTS.includes(e)) return 'jvm';
  if (e === '.go') return 'go';
  return 'other';
}

/** Probe a base path (no extension guaranteed) against the file set. */
function probe(base: string, set: Set<string>): string | undefined {
  if (set.has(base)) return base;
  for (const e of PROBE_EXTS) if (set.has(base + e)) return base + e;
  for (const e of PROBE_EXTS) if (set.has(`${base}/index${e}`)) return `${base}/index${e}`;
  if (set.has(`${base}/__init__.py`)) return `${base}/__init__.py`;
  // TS ESM convention: `./foo.js` refers to `foo.ts`
  const e = extname(base);
  if (e && ['.js', '.jsx', '.mjs', '.cjs'].includes(e)) {
    const stripped = base.slice(0, -e.length);
    for (const x of JS_EXTS) if (set.has(stripped + x)) return stripped + x;
  }
  return undefined;
}

function prefixed(dir: string, rest: string): string | undefined {
  return normalize(dir ? `${dir}/${rest}` : rest);
}

/**
 * Suffix index: module key (path-like, without extension) -> files.
 * Every segment-suffix of a key is indexed so `b/c` matches `src/a/b/c.py`.
 */
class SuffixIndex {
  private map = new Map<string, Set<string>>();

  add(key: string, file: string): void {
    const parts = key.split('/').filter(Boolean);
    for (let i = 0; i < parts.length; i++) {
      const k = parts.slice(i).join('/');
      let s = this.map.get(k);
      if (!s) this.map.set(k, (s = new Set()));
      s.add(file);
    }
  }

  /** Unique match or undefined. */
  get(key: string): string | undefined {
    const s = this.map.get(key);
    return s && s.size === 1 ? s.values().next().value : undefined;
  }
}

interface Indexes {
  set: Set<string>;
  py: SuffixIndex;
  jvm: SuffixIndex;
  go: SuffixIndex;
  /** Go package directory -> representative file (exact lookup, used with go.mod modules). */
  goRep: Map<string, string>;
  opts: ResolveOptions;
}

function buildIndexes(files: FileNode[], opts: ResolveOptions): Indexes {
  const set = new Set(files.map((f) => f.path));
  const py = new SuffixIndex();
  const jvm = new SuffixIndex();
  const go = new SuffixIndex();
  const goRep = new Map<string, string>();

  // Go: one representative file per package directory (prefer <dir>/<dir>.go, doc.go, then first).
  const goDirs = new Map<string, string[]>();
  for (const { path } of files) {
    const fam = family(path);
    const e = extname(path);
    if (fam === 'py') {
      const noExt = path.slice(0, -e.length);
      py.add(noExt.endsWith('/__init__') ? dirname(noExt) : noExt, path);
    } else if (fam === 'jvm') {
      jvm.add(path.slice(0, -e.length), path);
    } else if (fam === 'go' && !path.endsWith('_test.go')) {
      const d = dirname(path);
      const list = goDirs.get(d) ?? [];
      list.push(path);
      goDirs.set(d, list);
    }
  }
  for (const [d, list] of goDirs) {
    list.sort();
    const name = d.slice(d.lastIndexOf('/') + 1);
    const rep =
      list.find((p) => p === `${d}/${name}.go`) ?? list.find((p) => p === `${d}/doc.go`) ?? list[0];
    goRep.set(d, rep);
    if (d) go.add(d, rep);
  }
  return { set, py, jvm, go, goRep, opts };
}

/** Try the dotted key, then progressively drop trailing segments (symbol imports). */
function matchDotted(spec: string, idx: SuffixIndex): string | undefined {
  const parts = spec.replace(/\.\*$/, '').split('.').filter(Boolean);
  for (let n = parts.length; n >= 1; n--) {
    const hit = idx.get(parts.slice(0, n).join('/'));
    if (hit) return hit;
  }
  return undefined;
}

function resolveOne(from: string, spec: string, ix: Indexes): string | undefined {
  const dir = dirname(from);

  // Path-relative: ./x, ../x, ., ..
  if (spec === '.' || spec === '..' || spec.startsWith('./') || spec.startsWith('../')) {
    const base = prefixed(dir, spec);
    return base === undefined ? undefined : probe(base, ix.set);
  }

  const fam = family(from);

  // Python dotted relative: .x, ..x.y, .
  if (spec.startsWith('.')) {
    const m = /^(\.+)(.*)$/.exec(spec)!;
    let base: string | undefined = dir;
    for (let i = 1; i < m[1].length && base !== undefined; i++) {
      base = base ? dirname(base) : undefined;
    }
    if (base === undefined) return undefined;
    const rest = m[2].split('.').filter(Boolean);
    // Try full path, then drop trailing segments (from .mod import Symbol)
    for (let n = rest.length; n >= 0; n--) {
      const p = prefixed(base, rest.slice(0, n).join('/'));
      if (p === undefined) continue;
      if (ix.set.has(`${p}.py`)) return `${p}.py`;
      const init = p ? `${p}/__init__.py` : '__init__.py';
      if (ix.set.has(init)) return init;
    }
    return undefined;
  }

  // Absolute specs: language-specific suffix matching.
  if (fam === 'py') {
    if (PY_STDLIB.has(spec.split('.')[0])) return undefined;
    return matchDotted(spec, ix.py);
  }
  if (fam === 'jvm') return matchDotted(spec, ix.jvm);
  if (fam === 'go') {
    const mods = ix.opts.goModules;
    if (mods?.length) {
      // go.mod known: only imports under a module path are internal, resolved exactly.
      let best: { dir: string; module: string } | undefined;
      for (const m of mods) {
        if ((spec === m.module || spec.startsWith(m.module + '/')) && (!best || m.module.length > best.module.length)) {
          best = m;
        }
      }
      if (!best) return undefined;
      const rest = spec.slice(best.module.length + 1);
      const d = prefixed(best.dir, rest);
      return d === undefined ? undefined : ix.goRep.get(d);
    }
    const parts = spec.split('/').filter(Boolean);
    if (parts.length < 2) return undefined; // single-segment = stdlib (fmt, os, ...)
    // Match the longest unique suffix of the import path (module prefix differs from repo layout).
    for (let i = 0; i < parts.length; i++) {
      const hit = ix.go.get(parts.slice(i).join('/'));
      if (hit) return hit;
    }
    return undefined;
  }
  if (fam === 'js') return resolveAlias(from, spec, ix);
  return undefined; // others: external
}

/** tsconfig/jsconfig `paths` and `baseUrl` (nearest config wins; `extends` is not followed). */
function resolveAlias(from: string, spec: string, ix: Indexes): string | undefined {
  let cfg: NonNullable<ResolveOptions['tsconfigs']>[number] | undefined;
  for (const c of ix.opts.tsconfigs ?? []) {
    const inside = c.dir === '' || from.startsWith(c.dir + '/');
    if (inside && (!cfg || c.dir.length > cfg.dir.length)) cfg = c;
  }
  if (!cfg) return undefined;
  const base = prefixed(cfg.dir, cfg.baseUrl ?? '.');
  if (base === undefined) return undefined;
  for (const [pattern, targets] of Object.entries(cfg.paths ?? {})) {
    const star = pattern.indexOf('*');
    let sub: string | undefined;
    if (star < 0) sub = spec === pattern ? '' : undefined;
    else {
      const pre = pattern.slice(0, star);
      const post = pattern.slice(star + 1);
      if (spec.length >= pre.length + post.length && spec.startsWith(pre) && spec.endsWith(post)) {
        sub = spec.slice(pre.length, spec.length - post.length);
      }
    }
    if (sub === undefined) continue;
    for (const t of targets) {
      const p = prefixed(base, t.replace('*', sub));
      const hit = p === undefined ? undefined : probe(p, ix.set);
      if (hit) return hit;
    }
  }
  if (cfg.baseUrl !== undefined) {
    const p = prefixed(base, spec);
    return p === undefined ? undefined : probe(p, ix.set);
  }
  return undefined;
}

export function resolveImports(
  files: FileNode[],
  refs: ImportRef[],
  opts: ResolveOptions = {},
): Edge[] {
  const ix = buildIndexes(files, opts);
  const weights = new Map<string, Edge>();
  const seen = new Set<string>();
  for (const { from, spec } of refs) {
    if (!spec) continue;
    const to = resolveOne(from, spec.trim(), ix);
    if (!to || to === from) continue;
    // weight = number of distinct specifiers in `from` that resolve to `to`
    // (repeated identical specs, e.g. import + re-export of the same module, count once).
    const key = `${from}\0${to}`;
    const specKey = `${key}\0${spec}`;
    if (seen.has(specKey)) continue;
    seen.add(specKey);
    const e = weights.get(key);
    if (e) e.weight++;
    else weights.set(key, { from, to, weight: 1 });
  }
  return [...weights.values()].sort(
    (a, b) => a.from.localeCompare(b.from) || a.to.localeCompare(b.to),
  );
}
