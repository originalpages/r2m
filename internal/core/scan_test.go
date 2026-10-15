package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanDirIgnores(t *testing.T) {
	root := t.TempDir()
	write := func(p, s string) {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "ignored.txt\n*.log\n/rootonly\nbuild-out/\n!keep.log\n")
	write("README.md", "# hi")
	write("ignored.txt", "x")
	write("a.log", "x")
	write("keep.log", "x")
	write("rootonly", "x")
	write("sub/rootonly", "x")
	write("build-out/f.ts", "x")
	write("node_modules/p/index.js", "x")
	write("src/lib/util.ts", "x")
	write("src/index.ts", "x")

	files, err := ScanDir(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range files {
		got = append(got, f.Path)
	}
	want := []string{".gitignore", "README.md", "keep.log", "src/index.ts", "src/lib/util.ts", "sub/rootonly"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	if files[0].Size == 0 {
		t.Fatal("size not recorded")
	}
}

func TestScanDirMissingRoot(t *testing.T) {
	if _, err := ScanDir(filepath.Join(t.TempDir(), "nope"), nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestIgnorerPatterns(t *testing.T) {
	ig := &Ignorer{}
	ig.Add("# comment", "", "*.tmp", "docs/**/gen", "/top", "dir/", `\#hash`)
	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"a.tmp", false, true}, {"x/y/a.tmp", false, true},
		{"docs/gen", false, true}, {"docs/a/b/gen", false, true}, {"other/gen", false, false},
		{"top", false, true}, {"sub/top", false, false},
		{"dir", true, true}, {"dir", false, false}, {"x/dir", true, true},
		{"#hash", false, true}, {"comment", false, false},
	}
	for _, c := range cases {
		if got := ig.Ignores(c.path, c.isDir); got != c.want {
			t.Errorf("Ignores(%q, dir=%v) = %v, want %v", c.path, c.isDir, got, c.want)
		}
	}
}
