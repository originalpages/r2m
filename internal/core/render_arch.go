package core

import (
	"fmt"
	"sort"
	"strings"
)

// RootGroup is the group name for files in the repository root.
const RootGroup = "(root)"

// groupOf returns the first `depth` directory segments; root-level files -> RootGroup.
func groupOf(path string, depth int) string {
	parts := splitNonEmpty(path, "/")
	dirs := parts[:max(0, len(parts)-1)]
	if len(dirs) == 0 {
		return RootGroup
	}
	n := max(1, depth)
	if n > len(dirs) {
		n = len(dirs)
	}
	return strings.Join(dirs[:n], "/")
}

// RenderArch renders files collapsed to top-level directories with aggregated edges.
func RenderArch(edges []Edge, files []FileNode, opts Options) string {
	depth := opts.Depth
	if depth == 0 {
		depth = 1
	}
	maxNodes := opts.MaxNodes
	if maxNodes == 0 {
		maxNodes = defaultGraphMaxNodes
	}

	counts := map[string]int{}
	for _, f := range files {
		counts[groupOf(f.Path, depth)]++
	}

	agg := map[string]*Edge{}
	var order []string
	for _, e := range edges {
		from, to := groupOf(e.From, depth), groupOf(e.To, depth)
		if from == to {
			continue
		}
		for _, g := range []string{from, to} {
			if _, ok := counts[g]; !ok {
				counts[g] = 0
			}
		}
		key := from + "\x00" + to
		if cur, ok := agg[key]; ok {
			cur.Weight += e.Weight
		} else {
			agg[key] = &Edge{From: from, To: to, Weight: e.Weight}
			order = append(order, key)
		}
	}
	groupEdges := make([]Edge, 0, len(order))
	for _, k := range order {
		groupEdges = append(groupEdges, *agg[k])
	}
	sort.SliceStable(groupEdges, func(a, b int) bool {
		if c := LocaleCompare(groupEdges[a].From, groupEdges[b].From); c != 0 {
			return c < 0
		}
		return LocaleCompare(groupEdges[a].To, groupEdges[b].To) < 0
	})

	groups := make([]string, 0, len(counts))
	for g := range counts {
		groups = append(groups, g)
	}
	sort.SliceStable(groups, func(a, b int) bool {
		ga, gb := groups[a], groups[b]
		switch {
		case ga == gb:
			return false
		case ga == RootGroup:
			return true
		case gb == RootGroup:
			return false
		}
		return LocaleCompare(ga, gb) < 0
	})
	kept, keptEdges, omitted := capNodes(groups, groupEdges, maxNodes)
	shown, edgesOmitted := capEdges(keptEdges, MaxEdges)

	lines := []string{"flowchart LR"}
	if omitted > 0 {
		lines = append(lines, fmt.Sprintf("  %%%% %d less-connected groups omitted (maxNodes=%d)", omitted, maxNodes))
	}
	if edgesOmitted > 0 {
		lines = append(lines, fmt.Sprintf("  %%%% %d lightest edges omitted (Mermaid renders at most 500)", edgesOmitted))
	}
	ids := map[string]string{}
	n := 0
	for _, g := range groups {
		if !kept[g] {
			continue
		}
		id := fmt.Sprintf("g%d", n)
		n++
		ids[g] = id
		name := g
		if g != RootGroup {
			name = g + "/"
		}
		lines = append(lines, fmt.Sprintf("  %s[\"%s<br/>%s\"]", id, Esc(name), plural(counts[g], "file")))
	}
	for _, e := range shown {
		lines = append(lines, edgeLine(ids[e.From], ids[e.To], e.Weight, true))
	}
	return strings.Join(lines, "\n") + "\n"
}
