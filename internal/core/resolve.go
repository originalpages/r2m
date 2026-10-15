package core

import (
	"sort"
	"strings"
)

// Environment-agnostic posix path helpers (no path/filepath: specs always use '/').

func dirname(p string) string {
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return ""
	}
	return p[:i]
}

func extname(p string) string {
	base := p[strings.LastIndexByte(p, '/')+1:]
	i := strings.LastIndexByte(base, '.')
	if i <= 0 {
		return ""
	}
	return strings.ToLower(base[i:])
}

// normalize joins and cleans a path; ok is false if it escapes the root.
func normalize(p string) (string, bool) {
	var out []string
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "", ".":
		case "..":
			if len(out) == 0 {
				return "", false
			}
			out = out[:len(out)-1]
		default:
			out = append(out, seg)
		}
	}
	return strings.Join(out, "/"), true
}

var (
	jsExts    = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"}
	probeExts = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".py"}
	jvmExts   = []string{".java", ".kt", ".kts", ".scala", ".groovy"}
)

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

type family int

const (
	famOther family = iota
	famJS
	famPy
	famJVM
	famGo
)

func familyOf(path string) family {
	e := extname(path)
	switch {
	case contains(jsExts, e):
		return famJS
	case e == ".py":
		return famPy
	case contains(jvmExts, e):
		return famJVM
	case e == ".go":
		return famGo
	}
	return famOther
}

// probe tries a base path (extension not guaranteed) against the file set.
func probe(base string, set map[string]bool) (string, bool) {
	if set[base] {
		return base, true
	}
	for _, e := range probeExts {
		if set[base+e] {
			return base + e, true
		}
	}
	for _, e := range probeExts {
		if p := base + "/index" + e; set[p] {
			return p, true
		}
	}
	if p := base + "/__init__.py"; set[p] {
		return p, true
	}
	// TS ESM convention: `./foo.js` refers to `foo.ts`
	if e := extname(base); e != "" && contains([]string{".js", ".jsx", ".mjs", ".cjs"}, e) {
		stripped := base[:len(base)-len(e)]
		for _, x := range jsExts {
			if set[stripped+x] {
				return stripped + x, true
			}
		}
	}
	return "", false
}

func prefixed(dir, rest string) (string, bool) {
	if dir != "" {
		return normalize(dir + "/" + rest)
	}
	return normalize(rest)
}

// suffixIndex maps a path-like key (without extension) and every segment-suffix of it to files.
type suffixIndex struct {
	m map[string]map[string]bool
}

func newSuffixIndex() *suffixIndex { return &suffixIndex{m: map[string]map[string]bool{}} }

func (s *suffixIndex) add(key, file string) {
	var parts []string
	for _, p := range strings.Split(key, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	for i := range parts {
		k := strings.Join(parts[i:], "/")
		set := s.m[k]
		if set == nil {
			set = map[string]bool{}
			s.m[k] = set
		}
		set[file] = true
	}
}

// get returns the unique match, if any.
func (s *suffixIndex) get(key string) (string, bool) {
	set := s.m[key]
	if len(set) != 1 {
		return "", false
	}
	for f := range set {
		return f, true
	}
	return "", false
}

type indexes struct {
	set   map[string]bool
	py    *suffixIndex
	jvm   *suffixIndex
	goIdx *suffixIndex
	goRep map[string]string
	opts  ResolveOptions
}

func buildIndexes(files []FileNode, opts ResolveOptions) *indexes {
	ix := &indexes{
		set: map[string]bool{}, py: newSuffixIndex(), jvm: newSuffixIndex(),
		goIdx: newSuffixIndex(), goRep: map[string]string{}, opts: opts,
	}
	goDirs := map[string][]string{}
	var goDirOrder []string
	for _, f := range files {
		ix.set[f.Path] = true
	}
	for _, f := range files {
		path := f.Path
		e := extname(path)
		switch familyOf(path) {
		case famPy:
			noExt := path[:len(path)-len(e)]
			if strings.HasSuffix(noExt, "/__init__") {
				ix.py.add(dirname(noExt), path)
			} else {
				ix.py.add(noExt, path)
			}
		case famJVM:
			ix.jvm.add(path[:len(path)-len(e)], path)
		case famGo:
			if !strings.HasSuffix(path, "_test.go") {
				d := dirname(path)
				if _, ok := goDirs[d]; !ok {
					goDirOrder = append(goDirOrder, d)
				}
				goDirs[d] = append(goDirs[d], path)
			}
		}
	}
	for _, d := range goDirOrder {
		list := goDirs[d]
		sort.Strings(list)
		name := d[strings.LastIndexByte(d, '/')+1:]
		rep := list[0]
		switch {
		case contains(list, d+"/"+name+".go"):
			rep = d + "/" + name + ".go"
		case contains(list, d+"/doc.go"):
			rep = d + "/doc.go"
		}
		ix.goRep[d] = rep
		if d != "" {
			ix.goIdx.add(d, rep)
		}
	}
	return ix
}

// matchDotted tries the dotted key, then progressively drops trailing segments (symbol imports).
func matchDotted(spec string, idx *suffixIndex) (string, bool) {
	spec = strings.TrimSuffix(spec, ".*")
	var parts []string
	for _, p := range strings.Split(spec, ".") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	for n := len(parts); n >= 1; n-- {
		if hit, ok := idx.get(strings.Join(parts[:n], "/")); ok {
			return hit, true
		}
	}
	return "", false
}

func resolveOne(from, spec string, ix *indexes) (string, bool) {
	dir := dirname(from)

	// Path-relative: ./x, ../x, ., ..
	if spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") {
		base, ok := prefixed(dir, spec)
		if !ok {
			return "", false
		}
		return probe(base, ix.set)
	}

	fam := familyOf(from)

	// Python dotted relative: .x, ..x.y, .
	if strings.HasPrefix(spec, ".") {
		dots := 0
		for dots < len(spec) && spec[dots] == '.' {
			dots++
		}
		base := dir
		for i := 1; i < dots; i++ {
			if base == "" {
				return "", false
			}
			base = dirname(base)
		}
		var rest []string
		for _, p := range strings.Split(spec[dots:], ".") {
			if p != "" {
				rest = append(rest, p)
			}
		}
		for n := len(rest); n >= 0; n-- {
			p, ok := prefixed(base, strings.Join(rest[:n], "/"))
			if !ok {
				continue
			}
			if ix.set[p+".py"] {
				return p + ".py", true
			}
			init := "__init__.py"
			if p != "" {
				init = p + "/__init__.py"
			}
			if ix.set[init] {
				return init, true
			}
		}
		return "", false
	}

	switch fam {
	case famPy:
		if pyStdlib[strings.SplitN(spec, ".", 2)[0]] {
			return "", false
		}
		return matchDotted(spec, ix.py)
	case famJVM:
		return matchDotted(spec, ix.jvm)
	case famGo:
		if mods := ix.opts.GoModules; len(mods) > 0 {
			// go.mod known: only imports under a module path are internal, resolved exactly.
			var best *GoModule
			for i := range mods {
				m := &mods[i]
				if (spec == m.Module || strings.HasPrefix(spec, m.Module+"/")) &&
					(best == nil || len(m.Module) > len(best.Module)) {
					best = m
				}
			}
			if best == nil {
				return "", false
			}
			rest := ""
			if len(spec) > len(best.Module) {
				rest = spec[len(best.Module)+1:]
			}
			d, ok := prefixed(best.Dir, rest)
			if !ok {
				return "", false
			}
			hit, ok := ix.goRep[d]
			return hit, ok
		}
		parts := splitNonEmpty(spec, "/")
		if len(parts) < 2 {
			return "", false // single segment = stdlib (fmt, os, ...)
		}
		// Match the longest unique suffix of the import path (module prefix differs from repo layout).
		for i := range parts {
			if hit, ok := ix.goIdx.get(strings.Join(parts[i:], "/")); ok {
				return hit, true
			}
		}
		return "", false
	case famJS:
		return resolveAlias(from, spec, ix)
	}
	return "", false
}

func splitNonEmpty(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// resolveAlias handles tsconfig/jsconfig `paths` and `baseUrl` (nearest config wins; `extends` is not followed).
func resolveAlias(from, spec string, ix *indexes) (string, bool) {
	var cfg *TSConfig
	for i := range ix.opts.TSConfigs {
		c := &ix.opts.TSConfigs[i]
		inside := c.Dir == "" || strings.HasPrefix(from, c.Dir+"/")
		if inside && (cfg == nil || len(c.Dir) > len(cfg.Dir)) {
			cfg = c
		}
	}
	if cfg == nil {
		return "", false
	}
	baseURL := "."
	if cfg.BaseURL != nil {
		baseURL = *cfg.BaseURL
	}
	base, ok := prefixed(cfg.Dir, baseURL)
	if !ok {
		return "", false
	}
	for _, pattern := range cfg.PathKeys {
		targets := cfg.Paths[pattern]
		var sub string
		matched := false
		if star := strings.IndexByte(pattern, '*'); star < 0 {
			matched = spec == pattern
		} else {
			pre, post := pattern[:star], pattern[star+1:]
			if len(spec) >= len(pre)+len(post) && strings.HasPrefix(spec, pre) && strings.HasSuffix(spec, post) {
				sub = spec[len(pre) : len(spec)-len(post)]
				matched = true
			}
		}
		if !matched {
			continue
		}
		for _, t := range targets {
			p, ok := prefixed(base, strings.Replace(t, "*", sub, 1))
			if !ok {
				continue
			}
			if hit, ok := probe(p, ix.set); ok {
				return hit, true
			}
		}
	}
	if cfg.BaseURL != nil {
		if p, ok := prefixed(base, spec); ok {
			return probe(p, ix.set)
		}
	}
	return "", false
}

// ResolveImports turns raw specifiers into weighted file->file edges, sorted by (from, to).
// Weight is the number of distinct specifiers in `from` that resolve to `to`.
func ResolveImports(files []FileNode, refs []ImportRef, opts ResolveOptions) []Edge {
	ix := buildIndexes(files, opts)
	weights := map[string]*Edge{}
	var order []string
	seen := map[string]bool{}
	for _, r := range refs {
		if r.Spec == "" {
			continue
		}
		to, ok := resolveOne(r.From, strings.TrimSpace(r.Spec), ix)
		if !ok || to == r.From {
			continue
		}
		key := r.From + "\x00" + to
		specKey := key + "\x00" + r.Spec
		if seen[specKey] {
			continue
		}
		seen[specKey] = true
		if e, ok := weights[key]; ok {
			e.Weight++
		} else {
			weights[key] = &Edge{From: r.From, To: to, Weight: 1}
			order = append(order, key)
		}
	}
	edges := make([]Edge, 0, len(order))
	for _, k := range order {
		edges = append(edges, *weights[k])
	}
	sort.SliceStable(edges, func(a, b int) bool {
		if c := LocaleCompare(edges[a].From, edges[b].From); c != 0 {
			return c < 0
		}
		return LocaleCompare(edges[a].To, edges[b].To) < 0
	})
	return edges
}
