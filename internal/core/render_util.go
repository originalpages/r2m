package core

import (
	"fmt"
	"sort"
	"strings"
)

// Esc escapes text for use inside a double-quoted Mermaid label. `#` must go first (it
// introduces entity codes); backticks would otherwise start a markdown string.
func Esc(s string) string {
	return strings.NewReplacer(
		"#", "#35;", "&", "#38;", `"`, "#quot;", "`", "#96;", "<", "#lt;", ">", "#gt;",
	).Replace(s)
}

// MaxEdges is Mermaid's default render limit (GitHub, mermaid.live, the web page).
const MaxEdges = 500

// capNodes keeps the `max` most-connected nodes (by summed edge weight, ties by name).
func capNodes(nodes []string, edges []Edge, max int) (kept map[string]bool, kedges []Edge, omitted int) {
	kept = map[string]bool{}
	if len(nodes) <= max {
		for _, n := range nodes {
			kept[n] = true
		}
		return kept, edges, 0
	}
	degree := map[string]int{}
	for _, e := range edges {
		degree[e.From] += e.Weight
		degree[e.To] += e.Weight
	}
	ranked := append([]string(nil), nodes...)
	sort.SliceStable(ranked, func(i, j int) bool {
		if degree[ranked[i]] != degree[ranked[j]] {
			return degree[ranked[i]] > degree[ranked[j]]
		}
		return LocaleCompare(ranked[i], ranked[j]) < 0
	})
	if max < 0 {
		max = 0
	}
	for _, n := range ranked[:max] {
		kept[n] = true
	}
	for _, e := range edges {
		if kept[e.From] && kept[e.To] {
			kedges = append(kedges, e)
		}
	}
	return kept, kedges, len(nodes) - len(kept)
}

// capEdges keeps the `max` heaviest edges (ties by name), preserving the original order.
func capEdges(edges []Edge, max int) ([]Edge, int) {
	if len(edges) <= max {
		return edges, 0
	}
	idx := make([]int, len(edges))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		ea, eb := edges[idx[a]], edges[idx[b]]
		if ea.Weight != eb.Weight {
			return ea.Weight > eb.Weight
		}
		if c := LocaleCompare(ea.From, eb.From); c != 0 {
			return c < 0
		}
		return LocaleCompare(ea.To, eb.To) < 0
	})
	keep := map[int]bool{}
	for _, i := range idx[:max] {
		keep[i] = true
	}
	out := make([]Edge, 0, max)
	for i, e := range edges {
		if keep[i] {
			out = append(out, e)
		}
	}
	return out, len(edges) - max
}

func edgeLine(from, to string, weight int, alwaysLabel bool) string {
	if weight > 1 || alwaysLabel {
		return fmt.Sprintf("  %s -->|%d| %s", from, weight, to)
	}
	return fmt.Sprintf("  %s --> %s", from, to)
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
