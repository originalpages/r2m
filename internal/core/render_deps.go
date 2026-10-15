package core

import (
	"fmt"
	"sort"
	"strings"
)

const defaultGraphMaxNodes = 150

type depDir struct {
	name  string
	dirs  map[string]*depDir
	files []string // full paths
}

func buildDepTree(paths []string) *depDir {
	root := &depDir{dirs: map[string]*depDir{}}
	for _, p := range paths {
		parts := splitNonEmpty(p, "/")
		cur := root
		for _, part := range parts[:max(0, len(parts)-1)] {
			next := cur.dirs[part]
			if next == nil {
				next = &depDir{name: part, dirs: map[string]*depDir{}}
				cur.dirs[part] = next
			}
			cur = next
		}
		cur.files = append(cur.files, p)
	}
	return root
}

func uniqueNodes(edges []Edge) []string {
	set := map[string]bool{}
	for _, e := range edges {
		set[e.From] = true
		set[e.To] = true
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// outsidePrefix marks pseudo nodes (folders outside the focus) in focused graphs.
const outsidePrefix = "\x00"

// renderGraph draws file nodes clustered by directory plus optional "outside" folder nodes.
func renderGraph(edges []Edge, opts Options) string {
	maxNodes := opts.MaxNodes
	if maxNodes == 0 {
		maxNodes = defaultGraphMaxNodes
	}
	nodes := uniqueNodes(edges)
	kept, keptEdges, omitted := capNodes(nodes, edges, maxNodes)
	shown, edgesOmitted := capEdges(keptEdges, MaxEdges)

	lines := []string{"flowchart LR"}
	if len(nodes) == 0 {
		lines = append(lines, "  %% no internal dependencies found")
		return strings.Join(lines, "\n") + "\n"
	}
	if omitted > 0 {
		lines = append(lines, fmt.Sprintf("  %%%% %d less-connected files omitted (maxNodes=%d)", omitted, maxNodes))
	}
	if edgesOmitted > 0 {
		lines = append(lines, fmt.Sprintf("  %%%% %d lightest edges omitted (Mermaid renders at most 500)", edgesOmitted))
	}

	ids := map[string]string{}
	n, s := 0, 0

	var emit func(d *depDir, indent string)
	emit = func(d *depDir, indent string) {
		subs := make([]*depDir, 0, len(d.dirs))
		for _, c := range d.dirs {
			subs = append(subs, c)
		}
		sort.SliceStable(subs, func(a, b int) bool { return LocaleCompare(subs[a].name, subs[b].name) < 0 })
		for _, c := range subs {
			// Collapse single-child chains (a/b/c with no files in a, b) into one subgraph.
			label := c.name
			cur := c
			for len(cur.files) == 0 && len(cur.dirs) == 1 {
				for _, only := range cur.dirs {
					cur = only
				}
				label += "/" + cur.name
			}
			lines = append(lines, fmt.Sprintf("%ssubgraph s%d[\"%s/\"]", indent, s, Esc(label)))
			s++
			emit(cur, indent+"  ")
			lines = append(lines, indent+"end")
		}
		files := append([]string(nil), d.files...)
		sort.Strings(files)
		for _, f := range files {
			id := fmt.Sprintf("n%d", n)
			n++
			ids[f] = id
			lines = append(lines, fmt.Sprintf("%s%s[\"%s\"]", indent, id, Esc(f[strings.LastIndexByte(f, '/')+1:])))
		}
	}

	var fileNodes, outside []string
	for k := range kept {
		if strings.HasPrefix(k, outsidePrefix) {
			outside = append(outside, k)
		} else {
			fileNodes = append(fileNodes, k)
		}
	}
	sort.Strings(fileNodes)
	sort.Strings(outside)
	emit(buildDepTree(fileNodes), "  ")

	var outIDs []string
	for i, k := range outside {
		id := fmt.Sprintf("x%d", i)
		ids[k] = id
		outIDs = append(outIDs, id)
		lines = append(lines, fmt.Sprintf("  %s[\"%s\"]", id, Esc(strings.TrimPrefix(k, outsidePrefix))))
	}
	for _, e := range shown {
		lines = append(lines, edgeLine(ids[e.From], ids[e.To], e.Weight, false))
	}
	if len(outIDs) > 0 {
		lines = append(lines, "  classDef outside fill:#f6f8fa,stroke:#8c959f,stroke-dasharray:4 4,color:#57606a")
		lines = append(lines, "  class "+strings.Join(outIDs, ",")+" outside")
	}
	return strings.Join(lines, "\n") + "\n"
}

// RenderDeps renders the file->file dependency graph, clustered by directory.
// Only files with edges are shown.
func RenderDeps(edges []Edge, opts Options) string { return renderGraph(edges, opts) }

// inFocus reports whether path belongs to the focus folder. "." means files in the root.
func inFocus(path, focus string) bool {
	if focus == "." {
		return !strings.Contains(path, "/")
	}
	return path == focus || strings.HasPrefix(path, focus+"/")
}

// RenderDepsFocus renders one folder in detail: its files and their links, with the folders
// it talks to outside the focus shown as dashed boxes. An empty focus renders everything.
func RenderDepsFocus(edges []Edge, focus string, opts Options) string {
	focus = strings.Trim(focus, "/")
	if focus == "" {
		return renderGraph(edges, opts)
	}
	outsideKey := func(p string) string {
		d := dirname(p)
		if d == "" {
			d = "(root)"
		}
		return outsidePrefix + d + "/"
	}
	agg := map[string]*Edge{}
	var order []string
	for _, e := range edges {
		fi, ti := inFocus(e.From, focus), inFocus(e.To, focus)
		if !fi && !ti {
			continue
		}
		from, to := e.From, e.To
		if !fi {
			from = outsideKey(e.From)
		}
		if !ti {
			to = outsideKey(e.To)
		}
		key := from + "\x01" + to
		if cur, ok := agg[key]; ok {
			cur.Weight += e.Weight
		} else {
			agg[key] = &Edge{From: from, To: to, Weight: e.Weight}
			order = append(order, key)
		}
	}
	out := make([]Edge, 0, len(order))
	for _, k := range order {
		out = append(out, *agg[k])
	}
	return renderGraph(out, opts)
}

// CountNodes returns the number of distinct files that take part in the edges.
func CountNodes(edges []Edge) int { return len(uniqueNodes(edges)) }

// Focus is one folder offered for a detailed view of a large dependency graph.
type Focus struct {
	Label  string // what the user picks, e.g. "src/components/ (34 files)"
	Prefix string // folder path, "." for root files
	Files  int    // files with dependencies inside the folder
	Edges  int    // arrows in the focused graph (what makes a graph hard to read)
	Source string // Mermaid source for the focused graph
}

const (
	focusSplitFiles = 40 // top-level folders with more files or arrows are also offered per sub-folder
	focusSplitEdges = 60
	maxFoci         = 30
)

// DepsFoci lists folders worth zooming into, biggest first.
func DepsFoci(edges []Edge, opts Options) []Focus {
	key := func(p string, depth int) string {
		parts := splitNonEmpty(p, "/")
		if len(parts) <= 1 {
			return "."
		}
		dirs := parts[:len(parts)-1]
		if depth > len(dirs) {
			depth = len(dirs)
		}
		return strings.Join(dirs[:depth], "/")
	}
	files := uniqueNodes(edges)
	count := func(depth int) map[string]int {
		m := map[string]int{}
		for _, f := range files {
			m[key(f, depth)]++
		}
		return m
	}
	make1 := func(prefix string, n int) Focus {
		label := prefix + "/"
		if prefix == "." {
			label = "(root files)"
		}
		src := RenderDepsFocus(edges, prefix, opts)
		return Focus{
			Label:  fmt.Sprintf("%s (%d files)", label, n),
			Prefix: prefix,
			Files:  n,
			Edges:  strings.Count(src, "-->"),
			Source: src,
		}
	}

	var foci []Focus
	bigTop := map[string]bool{}
	for prefix, n := range count(1) {
		if n < 2 {
			continue
		}
		f := make1(prefix, n)
		foci = append(foci, f)
		if prefix != "." && (n > focusSplitFiles || f.Edges > focusSplitEdges) {
			bigTop[prefix] = true
		}
	}
	for prefix, n := range count(2) {
		top := prefix
		if i := strings.IndexByte(prefix, '/'); i >= 0 {
			top = prefix[:i]
		}
		if n < 2 || !bigTop[top] || prefix == top {
			continue
		}
		foci = append(foci, make1(prefix, n))
	}
	sort.SliceStable(foci, func(a, b int) bool {
		if foci[a].Files != foci[b].Files {
			return foci[a].Files > foci[b].Files
		}
		return foci[a].Prefix < foci[b].Prefix
	})
	if len(foci) > maxFoci {
		foci = foci[:maxFoci]
	}
	return foci
}
