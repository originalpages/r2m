package core

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func has(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("expected output to contain %q:\n%s", sub, s)
	}
}

func hasNot(t *testing.T, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Fatalf("expected output NOT to contain %q:\n%s", sub, s)
	}
}

func count(s string, re string) int { return len(regexp.MustCompile(re).FindAllString(s, -1)) }

func TestRenderTree(t *testing.T) {
	files := fl("a/b/c.ts", "a/d.ts", `x"<y>.md`)
	out := RenderTree(files, Options{})
	if !strings.HasPrefix(out, "flowchart LR") {
		t.Fatal("missing header")
	}
	has(t, out, `n0["."]`)
	has(t, out, `"x#quot;#lt;y#gt;.md"`)

	d1 := RenderTree(files, Options{Depth: 1})
	has(t, d1, "…(2 files)")
	hasNot(t, d1, "c.ts")

	var many []FileNode
	for i := 0; i < 50; i++ {
		many = append(many, FileNode{Path: fmt.Sprintf("d/s/f%d.ts", i)})
	}
	c := RenderTree(many, Options{MaxNodes: 10})
	has(t, c, "…(50 files)")
	if n := count(c, `(?m)^.*\["`); n > 10 {
		t.Fatalf("too many nodes: %d", n)
	}
}

func TestRenderDeps(t *testing.T) {
	files := fl("src/a.ts", "src/lib/b.ts", `src/lib/c"<x>.ts`, "README.md", "lonely.ts")
	edges := []Edge{
		{"src/a.ts", "src/lib/b.ts", 1},
		{"src/lib/b.ts", `src/lib/c"<x>.ts`, 3},
	}
	_ = files
	out := RenderDeps(edges, Options{})
	if !strings.HasPrefix(out, "flowchart LR\n") {
		t.Fatal("missing header")
	}
	has(t, out, `["src/"]`)
	has(t, out, `["lib/"]`)
	has(t, out, `"c#quot;#lt;x#gt;.ts"`)
	hasNot(t, out, "lonely")
	if count(out, `(?m)^\s*n\d+\["`) != 3 {
		t.Fatalf("expected 3 nodes:\n%s", out)
	}
	if !regexp.MustCompile(`n\d+ -->\|3\| n\d+`).MatchString(out) {
		t.Fatal("missing weighted edge")
	}
	if count(out, `subgraph`) != count(out, `(?m)^\s*end$`) {
		t.Fatal("unbalanced subgraphs")
	}
}

func TestRenderDepsCollapsesChains(t *testing.T) {
	out := RenderDeps([]Edge{{"a/b/c/x.ts", "a/b/c/y.ts", 1}}, Options{})
	has(t, out, `["a/b/c/"]`)
	if count(out, `subgraph`) != 1 {
		t.Fatal("expected one subgraph")
	}
}

func TestRenderDepsCapsNodes(t *testing.T) {
	var many []Edge
	for i := 0; i < 20; i++ {
		many = append(many, Edge{fmt.Sprintf("m/f%d.ts", i), "hub.ts", 1})
	}
	many = append(many, Edge{"m/f0.ts", "m/f1.ts", 5})
	out := RenderDeps(many, Options{MaxNodes: 3})
	if count(out, `(?m)^\s*n\d+\["`) != 3 {
		t.Fatalf("expected 3 nodes:\n%s", out)
	}
	has(t, out, `"hub.ts"`)
	has(t, out, `"f0.ts"`)
	has(t, out, `"f1.ts"`)
	has(t, out, "%% 18 less-connected files omitted")
}

func TestRenderDepsEmpty(t *testing.T) {
	has(t, RenderDeps(nil, Options{}), "%% no internal dependencies found")
}

func TestRenderArch(t *testing.T) {
	files := fl("index.ts", "src/a.ts", "src/core/b.ts", "src/core/c.ts", "lib/d.ts", "docs/x.md")
	edges := []Edge{
		{"index.ts", "src/a.ts", 1}, {"src/a.ts", "lib/d.ts", 1},
		{"src/core/b.ts", "lib/d.ts", 2}, {"src/core/b.ts", "src/core/c.ts", 4},
	}
	out := RenderArch(edges, files, Options{})
	has(t, out, `["(root)<br/>1 file"]`)
	has(t, out, `["src/<br/>3 files"]`)
	has(t, out, `["lib/<br/>1 file"]`)
	id := func(label string) string {
		m := regexp.MustCompile(`(g\d+)\["` + label).FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("no node %s", label)
		}
		return m[1]
	}
	has(t, out, id("src/")+" -->|3| "+id("lib/"))
	has(t, out, id(`\(root\)`)+" -->|1| "+id("src/"))
	if count(out, `-->`) != 2 {
		t.Fatal("expected 2 edges")
	}
	d2 := RenderArch(edges, files, Options{Depth: 2})
	has(t, d2, `["src/core/<br/>2 files"]`)
	has(t, d2, `["src/<br/>1 file"]`)
	has(t, d2, "-->|2|")
}

func memReader(src map[string]string) ReadFunc {
	return func(p string) (string, error) {
		s, ok := src[p]
		if !ok {
			return "", fmt.Errorf("ENOENT")
		}
		return s, nil
	}
}

func TestGenerate(t *testing.T) {
	src := map[string]string{
		"src/index.ts":    "import { u } from './lib/util.js';\nimport x from 'react';\n",
		"src/lib/util.ts": "export const u = 1;\n",
		"app/main.py":     "from .helpers import h\n",
		"app/helpers.py":  "h = 1\n",
		"README.md":       "# hi",
	}
	var paths []string
	for p := range src {
		paths = append(paths, p)
	}
	files := fl(append(paths, "broken.ts")...)

	deps, err := Generate(Deps, files, memReader(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`"index.ts"`, `"util.ts"`, `"main.py"`, `"helpers.py"`} {
		has(t, deps, s)
	}
	if count(deps, `-->`) != 2 {
		t.Fatalf("expected 2 edges:\n%s", deps)
	}
	arch, _ := Generate(Arch, files, memReader(src), Options{})
	has(t, arch, `["src/<br/>2 files"]`)
	hasNot(t, arch, "-->")
	if _, err := Generate("bogus", files, memReader(src), Options{}); err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestGenerateReadsGoModAndTSConfig(t *testing.T) {
	src := map[string]string{
		"go.mod":             "module example.com/m\n\ngo 1.22\n",
		"cmd/main.go":        "package main\nimport (\n\t\"net/http\"\n\t\"example.com/m/util\"\n)\n",
		"internal/http/h.go": "package http\n",
		"util/u.go":          "package util\n",
		"tsconfig.json":      "{\n // comment\n \"compilerOptions\": { \"baseUrl\": \".\", \"paths\": { \"@/*\": [\"web/*\"], }, },\n}",
		"web/main.ts":        "import a from '@/lib/a';\n",
		"web/lib/a.ts":       "export const a = 1;\n",
	}
	var paths []string
	for p := range src {
		paths = append(paths, p)
	}
	out, err := Generate(Deps, fl(paths...), memReader(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if count(out, `-->`) != 2 {
		t.Fatalf("want go.mod edge + alias edge:\n%s", out)
	}
	hasNot(t, out, `"h.go"`) // stdlib net/http must not match internal/http
}

func TestMermaidSafety(t *testing.T) {
	out := RenderTree(fl("`h`.ts", "a#1;.ts", "x&amp;y.ts"), Options{})
	hasNot(t, out, "`")
	has(t, out, "#96;h#96;.ts")
	has(t, out, "a#35;1;.ts")
	has(t, out, "x#38;amp;y.ts")
}

func TestNeverMoreThan500Edges(t *testing.T) {
	var edges []Edge
	for i := 0; i < 150; i++ {
		for j := 1; j <= 5; j++ {
			edges = append(edges, Edge{fmt.Sprintf("f%d.ts", i), fmt.Sprintf("f%d.ts", (i+j)%150), 1 + i%3})
		}
	}
	out := RenderDeps(edges, Options{MaxNodes: 150})
	if n := count(out, `-->`); n != 500 {
		t.Fatalf("edges = %d", n)
	}
	has(t, out, "250 lightest edges omitted")
}

func TestLocaleCompare(t *testing.T) {
	order := []string{"_next", "404", "a", "b", "B", "ünï"}
	for i := 0; i+1 < len(order); i++ {
		if LocaleCompare(order[i], order[i+1]) >= 0 {
			t.Errorf("%q should sort before %q", order[i], order[i+1])
		}
	}
}

func TestGenerateSkipsHugeFiles(t *testing.T) {
	src := map[string]string{
		"a.ts":   "import './b';\n",
		"b.ts":   "export {};\n",
		"big.ts": "import './b';\n",
	}
	files := []FileNode{{Path: "a.ts", Size: 20}, {Path: "b.ts", Size: 20}, {Path: "big.ts", Size: maxParseSize + 1}}
	out, err := Generate(Deps, files, memReader(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	has(t, out, `"a.ts"`)
	hasNot(t, out, "big.ts")
}

func TestRenderDepsFocus(t *testing.T) {
	edges := []Edge{
		{"src/ui/a.ts", "src/ui/b.ts", 1},
		{"src/ui/a.ts", "src/lib/util.ts", 2},
		{"src/lib/x.ts", "src/ui/b.ts", 1},
		{"src/lib/x.ts", "src/lib/util.ts", 1}, // unrelated to src/ui
		{"app/main.ts", "src/ui/a.ts", 3},
	}
	out := RenderDepsFocus(edges, "src/ui", Options{})
	has(t, out, `"a.ts"`)
	has(t, out, `"b.ts"`)
	hasNot(t, out, `"x.ts"`)    // outside files are folded into folder boxes
	hasNot(t, out, `"util.ts"`) // ...never shown individually
	has(t, out, `["src/lib/"]`)
	has(t, out, `["app/"]`)
	has(t, out, "classDef outside")
	has(t, out, "-->|2|")
	has(t, out, "-->|3|")
	// an empty focus is the full graph
	if RenderDepsFocus(edges, "", Options{}) != RenderDeps(edges, Options{}) {
		t.Fatal("empty focus should equal the full graph")
	}
	// root files
	root := RenderDepsFocus([]Edge{{"main.go", "pkg/a.go", 1}, {"pkg/a.go", "pkg/b.go", 1}}, ".", Options{})
	has(t, root, `"main.go"`)
	hasNot(t, root, `"b.go"`)
}

func TestDepsFociOffersBigFoldersAndSubfolders(t *testing.T) {
	var edges []Edge
	for i := 0; i < 45; i++ {
		edges = append(edges, Edge{fmt.Sprintf("src/ui/f%d.ts", i), fmt.Sprintf("src/lib/g%d.ts", i%5), 1})
	}
	edges = append(edges, Edge{"app/main.ts", "src/ui/f0.ts", 1}, Edge{"main.ts", "app/main.ts", 1})
	foci := DepsFoci(edges, Options{})
	var labels []string
	for _, f := range foci {
		labels = append(labels, f.Prefix)
		if !strings.HasPrefix(f.Source, "flowchart LR") {
			t.Errorf("%s: bad source", f.Prefix)
		}
	}
	want := map[string]bool{"src": true, "src/ui": true, "src/lib": true} // app has a single file: not worth a view
	got := map[string]bool{}
	for _, l := range labels {
		got[l] = true
	}
	for w := range want {
		if !got[w] {
			t.Errorf("missing focus %q in %v", w, labels)
		}
	}
	if foci[0].Prefix != "src" {
		t.Errorf("biggest folder first, got %q", foci[0].Prefix)
	}
}
