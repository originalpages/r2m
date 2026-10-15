package core

import (
	"reflect"
	"sort"
	"testing"
)

func extract(t *testing.T, path, src string) []string {
	t.Helper()
	p := GetParser(path)
	if p == nil {
		t.Fatalf("no parser for %s", path)
	}
	return p.Extract(path, src)
}

func sorted(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestGetParser(t *testing.T) {
	for _, f := range []string{"a.js", "a.jsx", "a.ts", "a.tsx", "a.mjs", "a.cjs", "x/y.PY", "a.go", "A.java"} {
		if GetParser(f) == nil {
			t.Errorf("expected parser for %s", f)
		}
	}
	for _, f := range []string{"README.md", "Makefile", ""} {
		if GetParser(f) != nil {
			t.Errorf("unexpected parser for %q", f)
		}
	}
}

func TestJSImportForms(t *testing.T) {
	src := "\nimport a from './a';\nimport * as b from \"../b.js\";\nimport { c, d as e } from 'pkg';\n" +
		"import type { T } from './types';\nimport './side-effect.css';\nexport * from './re';\n" +
		"export { x } from './re2';\nexport * as ns from './re3';\nconst f = require('./cjs');\n" +
		"const g = await import('./dyn');\nimport def, { z } from '@scope/pkg/sub';\nimport x = require('./eq');\n"
	want := []string{"./a", "../b.js", "pkg", "./types", "./side-effect.css", "./re", "./re2", "./re3", "./cjs", "./dyn", "@scope/pkg/sub", "./eq"}
	eq(t, sorted(extract(t, "a.ts", src)), sorted(want))
}

func TestJSMultiline(t *testing.T) {
	src := "import {\n  a,\n  b,\n} from './multi';\nexport {\n  q\n} from './multi2';"
	eq(t, extract(t, "a.js", src), []string{"./multi", "./multi2"})
}

func TestJSIgnoresCommentsAndStrings(t *testing.T) {
	src := "\n// import x from './line';\n/* import y from './block';\n   require('./block2') */\n" +
		"const s = \"import z from './instr'\";\nconst t = `require('./intpl')`;\nimport real from './real';\n" +
		"const r = /import 'x'/;\nobj.import('./method');\n"
	eq(t, extract(t, "a.js", src), []string{"./real"})
}

func TestJSDedupeAndDynamic(t *testing.T) {
	src := "import a from './a';\nimport('./a');\nrequire(name);\nimport(`./x/${n}`);"
	eq(t, extract(t, "a.js", src), []string{"./a"})
}

func TestJSXAndGenerics(t *testing.T) {
	src := "import React from 'react';\nexport const A = () => <div>import x from 'no'</div>;"
	found := false
	for _, s := range extract(t, "a.tsx", src) {
		if s == "react" {
			found = true
		}
	}
	if !found {
		t.Fatal("react import not found")
	}
}

func TestPythonImportForms(t *testing.T) {
	src := "\nimport os\nimport a.b.c\nimport x as y, z.w as q\nfrom pkg.mod import thing\nfrom .rel import f\n" +
		"from ..up.deep import g\nfrom . import sibling\nfrom .. import (\n    one,\n    two as t,\n)\n"
	got := map[string]bool{}
	for _, s := range extract(t, "a.py", src) {
		got[s] = true
	}
	for _, w := range []string{"os", "a.b.c", "x", "z.w", "pkg.mod", ".rel", "..up.deep", ".", ".sibling", "..", "..one", "..two"} {
		if !got[w] {
			t.Errorf("missing %q in %v", w, got)
		}
	}
}

func TestPythonIgnoresCommentsAndDocstrings(t *testing.T) {
	src := "\"\"\"\nimport fake\n\"\"\"\n# import nope\nx = \"import str\"\nimport real  # import trailing\n    \n"
	eq(t, extract(t, "a.py", src), []string{"real"})
}

func TestPythonIndentedDedupe(t *testing.T) {
	eq(t, extract(t, "a.py", "try:\n    import json\nexcept ImportError:\n    import json\n"), []string{"json"})
}

const goSrc = "package main\n\nimport \"fmt\"\nimport alias \"github.com/x/y\"\n\nimport (\n\t\"os\"\n\t// \"commented\"\n" +
	"\tstr \"strings\"\n\t_ \"github.com/lib/pq\"\n\t. \"testing\"\n\t\"example.com/proj/internal/util\" // trailing\n)\n\n" +
	"/* import \"nope\" */\nvar s = \"import \\\"fake\\\"\"\n"

var goWant = []string{"fmt", "github.com/x/y", "os", "strings", "github.com/lib/pq", "testing", "example.com/proj/internal/util"}

func TestGoImports(t *testing.T) {
	eq(t, sorted(extract(t, "a.go", goSrc)), sorted(goWant))
}

func TestGoRegexFallback(t *testing.T) {
	// the regex fallback must agree with go/parser on valid input
	eq(t, sorted(goImportsRegex(goSrc)), sorted(goWant))
	// and still work when the file is not valid Go
	broken := "package main\nimport (\n\t\"os\"\n)\nfunc ( {{{ \n"
	eq(t, extract(t, "a.go", broken), []string{"os"})
}

func TestJavaImports(t *testing.T) {
	src := "package a.b;\n\nimport java.util.List;\nimport static org.junit.Assert.assertEquals;\nimport com.foo.*;\n" +
		"// import ignored.Comment;\n/* import ignored.Block; */\nimport java.util.List;\n\npublic class A {}\n"
	eq(t, extract(t, "A.java", src), []string{"java.util.List", "org.junit.Assert.assertEquals", "com.foo.*"})
}

func TestParsersNeverPanicOnGarbage(t *testing.T) {
	garbage := "\x00\xff import 'unterminated\n`${ \"\"\" /* from . import ( \"\n"
	for _, p := range Parsers {
		_ = safeExtract(&p, "x."+p.Exts[0], garbage)
	}
}
