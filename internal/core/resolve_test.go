package core

import (
	"fmt"
	"reflect"
	"testing"
)

func fl(paths ...string) []FileNode {
	out := make([]FileNode, len(paths))
	for i, p := range paths {
		out[i] = FileNode{Path: p}
	}
	return out
}

func edgesOf(files []string, refs [][2]string, opts ...ResolveOptions) []string {
	var r []ImportRef
	for _, x := range refs {
		r = append(r, ImportRef{From: x[0], Spec: x[1]})
	}
	var o ResolveOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	var out []string
	for _, e := range ResolveImports(fl(files...), r, o) {
		out = append(out, fmt.Sprintf("%s -> %s (%d)", e.From, e.To, e.Weight))
	}
	return out
}

func check(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

var jsFiles = []string{"src/a.ts", "src/b.tsx", "src/lib/index.ts", "src/lib/c.js", "src/esm.ts", "top.mjs"}

func TestResolveJSRelative(t *testing.T) {
	check(t, edgesOf(jsFiles, [][2]string{
		{"src/a.ts", "./b"}, {"src/a.ts", "./lib"}, {"src/a.ts", "./lib/c.js"},
		{"src/lib/c.js", "../../top.mjs"}, {"src/lib/c.js", ".."},
	}), []string{
		"src/a.ts -> src/b.tsx (1)", "src/a.ts -> src/lib/c.js (1)", "src/a.ts -> src/lib/index.ts (1)",
		"src/lib/c.js -> top.mjs (1)",
	})
}

func TestResolveTSEsmJs(t *testing.T) {
	check(t, edgesOf(jsFiles, [][2]string{{"src/a.ts", "./esm.js"}}), []string{"src/a.ts -> src/esm.ts (1)"})
}

func TestResolveDropsExternalsAndAggregates(t *testing.T) {
	check(t, edgesOf(jsFiles, [][2]string{
		{"src/a.ts", "react"}, {"src/a.ts", "@scope/pkg"}, {"src/a.ts", "../../../x"}, {"src/a.ts", "./a"},
		{"src/a.ts", "./b"}, {"src/a.ts", "./b.tsx"}, {"src/a.ts", "./b"},
	}), []string{"src/a.ts -> src/b.tsx (2)"})
}

var pyFiles = []string{"pkg/__init__.py", "pkg/a.py", "pkg/sub/__init__.py", "pkg/sub/b.py", "pkg/sub/deep/c.py", "other/a.py"}

func TestResolvePythonRelative(t *testing.T) {
	check(t, edgesOf(pyFiles, [][2]string{
		{"pkg/sub/b.py", "."}, {"pkg/sub/b.py", "..a"}, {"pkg/sub/b.py", ".deep.c"},
		{"pkg/sub/deep/c.py", "...a.Symbol"}, {"pkg/a.py", ".sub.b"}, {"pkg/a.py", "....x"},
	}), []string{
		"pkg/a.py -> pkg/sub/b.py (1)", "pkg/sub/b.py -> pkg/a.py (1)",
		"pkg/sub/b.py -> pkg/sub/__init__.py (1)", "pkg/sub/b.py -> pkg/sub/deep/c.py (1)",
		"pkg/sub/deep/c.py -> pkg/a.py (1)",
	})
}

func TestResolvePythonAbsolute(t *testing.T) {
	check(t, edgesOf(pyFiles, [][2]string{
		{"other/a.py", "pkg.sub.b"}, {"other/a.py", "pkg.sub"}, {"other/a.py", "deep.c"},
		{"pkg/sub/b.py", "a"}, {"pkg/sub/b.py", "os.path"},
	}), []string{
		"other/a.py -> pkg/sub/__init__.py (1)", "other/a.py -> pkg/sub/b.py (1)", "other/a.py -> pkg/sub/deep/c.py (1)",
	})
}

func TestResolveJava(t *testing.T) {
	files := []string{"src/main/java/com/x/A.java", "src/main/java/com/x/util/B.java", "src/main/java/com/x/C.kt"}
	check(t, edgesOf(files, [][2]string{
		{"src/main/java/com/x/A.java", "com.x.util.B"}, {"src/main/java/com/x/A.java", "com.x.C.helper"},
		{"src/main/java/com/x/util/B.java", "com.x.A.*"}, {"src/main/java/com/x/util/B.java", "java.util.List"},
	}), []string{
		"src/main/java/com/x/A.java -> src/main/java/com/x/C.kt (1)",
		"src/main/java/com/x/A.java -> src/main/java/com/x/util/B.java (1)",
		"src/main/java/com/x/util/B.java -> src/main/java/com/x/A.java (1)",
	})
}

func TestResolveGoHeuristic(t *testing.T) {
	files := []string{"main.go", "internal/store/store.go", "internal/store/db.go", "internal/store/store_test.go", "pkg/api/handler.go"}
	check(t, edgesOf(files, [][2]string{
		{"main.go", "github.com/me/app/internal/store"}, {"main.go", "github.com/me/app/pkg/api"},
		{"main.go", "fmt"}, {"main.go", "net/http"},
	}), []string{"main.go -> internal/store/store.go (1)", "main.go -> pkg/api/handler.go (1)"})
}

func TestResolveGoModule(t *testing.T) {
	files := []string{"cmd/main.go", "internal/http/h.go", "util/u.go"}
	got := edgesOf(files, [][2]string{
		{"cmd/main.go", "net/http"}, {"cmd/main.go", "example.com/m/internal/http"},
		{"cmd/main.go", "example.com/m/util"}, {"cmd/main.go", "github.com/other/util"},
	}, ResolveOptions{GoModules: []GoModule{{Dir: "", Module: "example.com/m"}}})
	check(t, got, []string{"cmd/main.go -> internal/http/h.go (1)", "cmd/main.go -> util/u.go (1)"})
}

func TestResolvePythonStdlib(t *testing.T) {
	check(t, edgesOf([]string{"app.py", "utils/json.py", "utils/helpers.py"}, [][2]string{
		{"app.py", "json"}, {"app.py", "os.path"}, {"app.py", "helpers"},
	}), []string{"app.py -> utils/helpers.py (1)"})
}

func TestResolveTSConfigPaths(t *testing.T) {
	files := []string{"src/main.ts", "src/lib/a.ts", "src/lib/b/index.ts", "other/x.ts"}
	base := "."
	cfg := TSConfig{Dir: "", BaseURL: &base, Paths: map[string][]string{"@/*": {"src/*"}}, PathKeys: []string{"@/*"}}
	check(t, edgesOf(files, [][2]string{
		{"src/main.ts", "@/lib/a"}, {"src/main.ts", "@/lib/b"}, {"src/main.ts", "react"}, {"other/x.ts", "@/lib/a"},
	}, ResolveOptions{TSConfigs: []TSConfig{cfg}}), []string{
		"other/x.ts -> src/lib/a.ts (1)", "src/main.ts -> src/lib/a.ts (1)", "src/main.ts -> src/lib/b/index.ts (1)",
	})
	check(t, edgesOf(files, [][2]string{{"src/main.ts", "src/lib/a"}},
		ResolveOptions{TSConfigs: []TSConfig{{Dir: "", BaseURL: &base}}}), []string{"src/main.ts -> src/lib/a.ts (1)"})
}
