export interface FileNode {
  path: string; // posix, relative to root
  size?: number;
}

/** Raw import specifier found in file `from`. */
export interface ImportRef {
  from: string;
  spec: string;
}

export interface Edge {
  from: string;
  to: string;
  weight: number;
}

/** Project config that changes how import specifiers map to files. */
export interface ResolveOptions {
  /** `go.mod` files: directory (posix, '' = root) and declared module path. */
  goModules?: { dir: string; module: string }[];
  /** `tsconfig.json` / `jsconfig.json` files with their `baseUrl` and `paths`. */
  tsconfigs?: { dir: string; baseUrl?: string; paths?: Record<string, string[]> }[];
}

export interface Parser {
  exts: string[];
  /** Returns raw import specifiers. */
  extract(path: string, source: string): string[];
}

export interface RenderOptions {
  depth?: number;
  maxNodes?: number;
}

export type DiagramType = 'tree' | 'deps' | 'arch';
