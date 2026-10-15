import { generateDiagram, CONFIG_FILES } from './core/generate';
import type { DiagramType, FileNode } from './core/types';

// Keep in sync with DefaultIgnores in internal/core/scan.go.
const DEFAULT_IGNORES = [
  'node_modules',
  '.git',
  'dist',
  'build',
  '.next',
  '__pycache__',
  'venv',
  'coverage',
];
const SUPPORTED_EXTS = ['js', 'jsx', 'ts', 'tsx', 'mjs', 'cjs', 'py', 'go', 'java'];
const MAX_FETCH_FILES = 400;
const CONCURRENCY = 8;

const $ = <T extends HTMLElement>(id: string): T => document.getElementById(id) as T;
const form = $<HTMLFormElement>('form');
const goBtn = $<HTMLButtonElement>('go');
const statusEl = $<HTMLElement>('status');
const resultEl = $<HTMLElement>('result');
const diagramEl = $<HTMLElement>('diagram');
const sourceEl = $<HTMLElement>('source');

let currentSource = '';
let currentSvg = '';
let currentName = 'diagram';
let currentTitle = '';
let currentSourceUrl = '';
const PROJECT_URL = 'https://github.com/originalpages/r2m';

class UserError extends Error {}

function setStatus(msg: string, isError = false): void {
  statusEl.textContent = msg;
  statusEl.className = isError ? 'error' : '';
}

function parseRepo(input: string): { owner: string; repo: string; branch?: string } {
  let s = input.trim();
  s = s.replace(/^(?:https?:\/\/)?(?:www\.)?github\.com\//i, '');
  s = s.replace(/[?#].*$/, '').replace(/\.git$/i, '').replace(/\/+$/, '');
  const parts = s.split('/').filter(Boolean);
  if (parts.length < 2 || !/^[\w.-]+$/.test(parts[0]) || !/^[\w.-]+$/.test(parts[1])) {
    throw new UserError('Enter a repository as "owner/repo" or a github.com URL.');
  }
  const out: { owner: string; repo: string; branch?: string } = { owner: parts[0], repo: parts[1] };
  if (parts[2] === 'tree' && parts.length > 3) out.branch = parts.slice(3).join('/');
  return out;
}

function ignored(path: string): boolean {
  return path.split('/').some((seg) => DEFAULT_IGNORES.includes(seg));
}

function ext(path: string): string {
  const i = path.lastIndexOf('.');
  return i < 0 ? '' : path.slice(i + 1).toLowerCase();
}

async function fetchTree(
  owner: string,
  repo: string,
  ref: string,
): Promise<{ files: FileNode[]; truncated: boolean }> {
  const url = `https://api.github.com/repos/${owner}/${repo}/git/trees/${encodeURIComponent(ref)}?recursive=1`;
  let res: Response;
  try {
    res = await fetch(url, { headers: { Accept: 'application/vnd.github+json' } });
  } catch {
    throw new UserError('Network error while contacting the GitHub API.');
  }
  if (res.status === 404) {
    throw new UserError(
      `Not found: ${owner}/${repo}@${ref}. Check the name and branch; private repositories are not supported.`,
    );
  }
  if (res.status === 403 || res.status === 429) {
    const reset = res.headers.get('x-ratelimit-reset');
    const when = reset ? ` Resets around ${new Date(Number(reset) * 1000).toLocaleTimeString()}.` : '';
    throw new UserError(
      `GitHub API rate limit exceeded or access forbidden (unauthenticated limit is 60 requests/hour).${when}`,
    );
  }
  if (res.status === 409) throw new UserError('The repository is empty.');
  if (!res.ok) throw new UserError(`GitHub API error: HTTP ${res.status}.`);
  const data = (await res.json()) as {
    tree: { path: string; type: string; size?: number }[];
    truncated?: boolean;
  };
  const files = data.tree
    .filter((e) => e.type === 'blob' && !ignored(e.path))
    .map((e) => ({ path: e.path, size: e.size }));
  return { files, truncated: !!data.truncated };
}

function makeLimiter(max: number): <T>(fn: () => Promise<T>) => Promise<T> {
  let active = 0;
  const queue: (() => void)[] = [];
  return async <T>(fn: () => Promise<T>): Promise<T> => {
    // A finishing task hands its slot straight to the next waiter, so `active` never exceeds `max`.
    if (active >= max) await new Promise<void>((r) => queue.push(r));
    else active++;
    try {
      return await fn();
    } finally {
      const nextWaiter = queue.shift();
      if (nextWaiter) nextWaiter();
      else active--;
    }
  };
}

async function loadMermaid() {
  const m = (await import('mermaid')).default;
  // htmlLabels off: labels are plain SVG text, so PNG export is not blocked by the browser.
  m.initialize({
    startOnLoad: false,
    securityLevel: 'strict',
    theme: 'default',
    maxTextSize: 5_000_000,
    htmlLabels: false,
    flowchart: { htmlLabels: false },
  });
  return m;
}

function download(name: string, text: string, mime: string): void {
  const url = URL.createObjectURL(new Blob([text], { type: mime }));
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

async function generate(): Promise<void> {
  const { owner, repo, branch: urlBranch } = parseRepo($<HTMLInputElement>('repo').value);
  const branch = $<HTMLInputElement>('branch').value.trim() || urlBranch || 'HEAD';
  const type = $<HTMLSelectElement>('type').value as DiagramType;
  const num = (id: string): number | undefined => {
    const v = parseInt($<HTMLInputElement>(id).value, 10);
    return Number.isFinite(v) && v > 0 ? v : undefined;
  };
  const opts = { depth: num('depth'), maxNodes: num('maxNodes') };

  setStatus('Fetching file list...');
  const { files, truncated } = await fetchTree(owner, repo, branch);
  if (!files.length) throw new UserError('No files found after applying default ignores.');

  const notices: string[] = [];
  if (truncated) notices.push('GitHub truncated the file list (repository too large); diagram is partial.');

  const supported = files.filter((f) => SUPPORTED_EXTS.includes(ext(f.path)));
  const allowed = new Set(supported.slice(0, MAX_FETCH_FILES).map((f) => f.path));
  // Project config (go.mod, tsconfig.json) is tiny and improves import resolution.
  if (type !== 'tree') {
    for (const f of files) {
      if (CONFIG_FILES.includes(f.path.slice(f.path.lastIndexOf('/') + 1))) allowed.add(f.path);
    }
  }
  if (type !== 'tree' && supported.length > MAX_FETCH_FILES) {
    notices.push(
      `Only the first ${MAX_FETCH_FILES} of ${supported.length} source files were analysed.`,
    );
  }

  const limit = makeLimiter(CONCURRENCY);
  const rawBase = `https://raw.githubusercontent.com/${owner}/${repo}/${branch
    .split('/')
    .map(encodeURIComponent)
    .join('/')}/`;
  let fetched = 0;
  let failed = 0;
  const readFile = async (path: string): Promise<string> => {
    if (!allowed.has(path)) return '';
    return limit(async () => {
      try {
        const r = await fetch(rawBase + path.split('/').map(encodeURIComponent).join('/'));
        if (!r.ok) throw new Error(String(r.status));
        const t = await r.text();
        fetched++;
        if (type !== 'tree') setStatus(`Fetching sources... ${fetched}/${allowed.size}`);
        return t;
      } catch {
        failed++;
        return '';
      }
    });
  };

  setStatus('Generating diagram...');
  const source = await generateDiagram(type, files, readFile, opts);
  if (failed) notices.push(`${failed} file(s) could not be fetched and were skipped.`);

  setStatus('Rendering...');
  const mermaid = await loadMermaid();
  const { svg } = await mermaid.render(`m${Date.now()}`, source);

  currentSource = source;
  currentSvg = svg;
  currentName = `${repo}-${type}`;
  currentTitle = `${owner}/${repo} \u2013 ${{ arch: 'Overview', deps: 'Dependencies', tree: 'Folders' }[type]}`;
  currentSourceUrl = `https://github.com/${owner}/${repo}`;
  sourceEl.textContent = source;
  diagramEl.innerHTML = svg; // sanitized by mermaid (securityLevel 'strict')
  resultEl.hidden = false;
  setStatus(`${owner}/${repo} (${branch}): ${files.length} files.${notices.length ? '\n' + notices.join('\n') : ''}`);
}

form.addEventListener('submit', async (ev) => {
  ev.preventDefault();
  goBtn.disabled = true;
  try {
    await generate();
  } catch (e) {
    resultEl.hidden = true;
    const msg = e instanceof UserError ? e.message : `Failed: ${e instanceof Error ? e.message : String(e)}`;
    setStatus(msg, true);
  } finally {
    goBtn.disabled = false;
  }
});

$('copy').addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(currentSource);
    setStatus('Copied Mermaid source to clipboard.');
  } catch {
    setStatus('Copy failed; select the source text manually.', true);
  }
});
$('dl-mmd').addEventListener('click', () => download(`${currentName}.mmd`, currentSource, 'text/plain'));
/** Title + diagram + footer as one standalone SVG, so SVG and PNG exports match. */
function composeSvg(): { xml: string; w: number; h: number } | null {
  const svg = diagramEl.querySelector('svg');
  if (!svg) return null;
  const vb = svg.viewBox.baseVal;
  const box = svg.getBoundingClientRect();
  const w = Math.ceil(vb.width || box.width);
  const h = Math.ceil(vb.height || box.height);
  const pad = 24;
  const W = Math.max(w + pad * 2, 520);
  const H = h + 64 + 44;
  const clone = svg.cloneNode(true) as SVGSVGElement;
  clone.removeAttribute('style');
  clone.setAttribute('width', String(w));
  clone.setAttribute('height', String(h));
  clone.setAttribute('x', String(pad));
  clone.setAttribute('y', '64');
  const esc = (t: string) => t.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  const font = 'system-ui, Segoe UI, Arial, sans-serif';
  const footer = `Generated by r2m · ${PROJECT_URL} · Source: ${currentSourceUrl}`;
  const xml =
    `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">` +
    `<rect width="100%" height="100%" fill="#ffffff"/>` +
    `<text x="${pad}" y="38" font-family="${font}" font-size="22" font-weight="700" fill="#1f2328">${esc(currentTitle)}</text>` +
    new XMLSerializer().serializeToString(clone) +
    `<text x="${pad}" y="${H - 16}" font-family="${font}" font-size="13" fill="#656d76">${esc(footer)}</text>` +
    `</svg>`;
  return { xml, w: W, h: H };
}

$('dl-svg').addEventListener('click', () => {
  const c = composeSvg();
  if (c) download(`${currentName}.svg`, c.xml, 'image/svg+xml');
});

$('dl-png').addEventListener('click', () => {
  const c = composeSvg();
  if (!c) return;
  const img = new Image();
  img.onload = () => {
    const scale = Math.min(2, 16000 / Math.max(c.w, c.h));
    const canvas = document.createElement('canvas');
    canvas.width = Math.round(c.w * scale);
    canvas.height = Math.round(c.h * scale);
    const ctx = canvas.getContext('2d')!;
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
    canvas.toBlob((b) => {
      if (!b) return setStatus('Could not create the picture.', true);
      const url = URL.createObjectURL(b);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${currentName}.png`;
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    }, 'image/png');
  };
  img.onerror = () => setStatus('Could not create the picture.', true);
  img.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(c.xml);
});
