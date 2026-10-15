package core

import (
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Parser extracts raw import specifiers from source text. It never panics on bad input.
type Parser struct {
	Exts    []string
	Extract func(path, source string) []string
}

// Parsers lists the supported languages.
var Parsers = []Parser{jsParser, pythonParser, goParser, javaParser}

// GetParser returns the parser for a file path by extension (case-insensitive), or nil.
func GetParser(path string) *Parser {
	base := path
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		base = path[i+1:]
	}
	dot := strings.LastIndexByte(base, '.')
	if dot < 0 {
		return nil
	}
	ext := strings.ToLower(base[dot+1:])
	for i := range Parsers {
		for _, e := range Parsers[i].Exts {
			if e == ext {
				return &Parsers[i]
			}
		}
	}
	return nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ---- JS / TS ---------------------------------------------------------------

const mark = '\x01'

// maskJS replaces comments with whitespace and string literals with \x01N\x01 placeholders so
// later regexes cannot match inside strings/comments. A nil table entry means a template
// literal with `${}`.
func maskJS(src string) (string, []*string) {
	var strs []*string
	var out strings.Builder
	prev := byte(0)
	n := len(src)
	i := 0
	const regexPrev = "(,=:[!&|?{};+-*%<>~^"
	for i < n {
		c := src[i]
		var d byte
		if i+1 < n {
			d = src[i+1]
		}
		if c == '/' && d == '/' {
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && d == '*' {
			end := strings.Index(src[i+2:], "*/")
			stop := n
			if end >= 0 {
				stop = i + 2 + end + 2
			}
			for _, ch := range []byte(src[i:stop]) {
				if ch == '\n' {
					out.WriteByte('\n')
				} else {
					out.WriteByte(' ')
				}
			}
			i = stop
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			j := i + 1
			var val strings.Builder
			dynamic := false
			for j < n && src[j] != c {
				if src[j] == '\\' {
					if j+1 < n {
						val.WriteByte(src[j+1])
					}
					j += 2
					continue
				}
				if c == '`' && src[j] == '$' && j+1 < n && src[j+1] == '{' {
					dynamic = true
				}
				if c != '`' && src[j] == '\n' {
					break // unterminated
				}
				val.WriteByte(src[j])
				j++
			}
			if dynamic {
				strs = append(strs, nil)
			} else {
				s := val.String()
				strs = append(strs, &s)
			}
			out.WriteByte(mark)
			out.WriteString(strconv.Itoa(len(strs) - 1))
			out.WriteByte(mark)
			prev = '"'
			i = j + 1
			continue
		}
		if c == '/' && (prev == 0 || strings.IndexByte(regexPrev, prev) >= 0) {
			j := i + 1
			inClass := false
			for j < n && src[j] != '\n' {
				if src[j] == '\\' {
					j += 2
				} else {
					if src[j] == '[' {
						inClass = true
					} else if src[j] == ']' {
						inClass = false
					} else if src[j] == '/' && !inClass {
						break
					}
					j++
				}
			}
			if j < n && src[j] == '/' {
				out.WriteByte(' ')
				prev = ')'
				i = j + 1
				continue
			}
		}
		out.WriteByte(c)
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' && c != '\f' && c != '\v' {
			prev = c
		}
		i++
	}
	return out.String(), strs
}

var jsPatterns = []*regexp.Regexp{
	// import x, { y } from 'm' / import type {..} from 'm' / import * as ns from 'm'
	regexp.MustCompile(`import\s+(?:type\s+|typeof\s+)?[^;\x01(]*?\bfrom\s*\x01(\d+)\x01`),
	// import 'side-effect'
	regexp.MustCompile(`import\s*\x01(\d+)\x01`),
	// export * from 'm' / export { a } from 'm' / export type {..} from 'm'
	regexp.MustCompile(`export\s+(?:type\s+)?(?:\*(?:\s*as\s+[\w$]+)?|\{[^}]*\})\s*from\s*\x01(\d+)\x01`),
	// require('m') and import('m')
	regexp.MustCompile(`require\s*\(\s*\x01(\d+)\x01\s*[,)]`),
	regexp.MustCompile(`import\s*\(\s*\x01(\d+)\x01`),
}

func isWordByte(b byte) bool {
	return b == '_' || b == '$' || b == '.' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

var jsParser = Parser{
	Exts: []string{"js", "jsx", "ts", "tsx", "mjs", "cjs"},
	Extract: func(_, source string) []string {
		code, strs := maskJS(source)
		type hit struct {
			idx  int
			spec string
		}
		var found []hit
		for _, re := range jsPatterns {
			for _, m := range re.FindAllStringSubmatchIndex(code, -1) {
				// emulate the lookbehind (?<![.\w$]) at the start of the match
				if m[0] > 0 && isWordByte(code[m[0]-1]) {
					continue
				}
				k, err := strconv.Atoi(code[m[2]:m[3]])
				if err != nil || k >= len(strs) || strs[k] == nil || *strs[k] == "" {
					continue
				}
				found = append(found, hit{m[0], *strs[k]})
			}
		}
		sort.SliceStable(found, func(a, b int) bool { return found[a].idx < found[b].idx })
		specs := make([]string, len(found))
		for i, f := range found {
			specs[i] = f.spec
		}
		return dedupe(specs)
	},
}

// ---- Python ----------------------------------------------------------------

// stripPython blanks out comments and string literals (incl. triple-quoted docstrings).
func stripPython(src string) string {
	var out strings.Builder
	n := len(src)
	i := 0
	for i < n {
		c := src[i]
		if c == '#' {
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		}
		if c == '"' || c == '\'' {
			q := string(c)
			triple := strings.HasPrefix(src[i:], strings.Repeat(q, 3))
			if triple {
				q = strings.Repeat(q, 3)
			}
			j := i + len(q)
			for j < n && !strings.HasPrefix(src[j:], q) {
				if src[j] == '\\' {
					j++
				} else if !triple && src[j] == '\n' {
					break
				}
				j++
			}
			stop := j + len(q)
			if stop > n {
				stop = n
			}
			for _, ch := range []byte(src[i:stop]) {
				if ch == '\n' {
					out.WriteByte('\n')
				} else {
					out.WriteByte(' ')
				}
			}
			i = stop
			continue
		}
		out.WriteByte(c)
		i++
	}
	return out.String()
}

const pyName = `[A-Za-z_]\w*`
const pyDotted = pyName + `(?:\s*\.\s*` + pyName + `)*`

var (
	pyImportRe  = regexp.MustCompile(`(?m)^[ \t]*import[ \t]+([^\n]+)`)
	pyDottedRe  = regexp.MustCompile(`^` + pyDotted + `$`)
	pyNameRe    = regexp.MustCompile(`^` + pyName + `$`)
	pyFromRe    = regexp.MustCompile(`(?m)^[ \t]*from[ \t]+(\.*(?:` + pyDotted + `)?)[ \t]*import\b[ \t]*(\([^)]*\)|[^\n]*)`)
	pyContRe    = regexp.MustCompile(`\\\r?\n`)
	pyAsRe      = regexp.MustCompile(`\s+as\s+`)
	pyWSRe      = regexp.MustCompile(`\s+`)
	pyDotsOnlyR = regexp.MustCompile(`^\.+$`)
)

var pythonParser = Parser{
	Exts: []string{"py"},
	Extract: func(_, source string) []string {
		code := stripPython(source)
		code = pyContRe.ReplaceAllString(code, " ")
		code = strings.ReplaceAll(code, ";", "\n")
		var out []string

		for _, m := range pyImportRe.FindAllStringSubmatch(code, -1) {
			for _, part := range strings.Split(m[1], ",") {
				name := pyAsRe.Split(strings.TrimSpace(part), 2)[0]
				name = pyWSRe.ReplaceAllString(name, "")
				if pyDottedRe.MatchString(name) {
					out = append(out, name)
				}
			}
		}
		for _, m := range pyFromRe.FindAllStringSubmatch(code, -1) {
			mod := pyWSRe.ReplaceAllString(m[1], "")
			if mod == "" {
				continue
			}
			out = append(out, mod)
			if pyDotsOnlyR.MatchString(mod) {
				// `from . import x`: x may be a submodule; emit `.x` candidates too
				names := strings.NewReplacer("(", " ", ")", " ").Replace(m[2])
				for _, part := range strings.Split(names, ",") {
					nm := pyAsRe.Split(strings.TrimSpace(part), 2)[0]
					if pyNameRe.MatchString(nm) {
						out = append(out, mod+nm)
					}
				}
			}
		}
		return dedupe(out)
	},
}

// ---- Go --------------------------------------------------------------------

// stripCComments blanks out // and /* */ comments while leaving string literals intact.
func stripCComments(src string) string {
	var out strings.Builder
	n := len(src)
	i := 0
	for i < n {
		c := src[i]
		var d byte
		if i+1 < n {
			d = src[i+1]
		}
		if c == '/' && d == '/' {
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && d == '*' {
			end := strings.Index(src[i+2:], "*/")
			stop := n
			if end >= 0 {
				stop = i + 2 + end + 2
			}
			for _, ch := range []byte(src[i:stop]) {
				if ch == '\n' {
					out.WriteByte('\n')
				} else {
					out.WriteByte(' ')
				}
			}
			i = stop
			continue
		}
		if c == '"' || c == '`' || c == '\'' {
			j := i + 1
			for j < n && src[j] != c {
				if src[j] == '\\' && c != '`' {
					j++
				} else if src[j] == '\n' && c != '`' {
					break
				}
				j++
			}
			stop := j + 1
			if stop > n {
				stop = n
			}
			out.WriteString(src[i:stop])
			i = j + 1
			continue
		}
		out.WriteByte(c)
		i++
	}
	return out.String()
}

const goSpec = "(?:[\\w.]+\\s+)?(?:\"([^\"\\n]+)\"|`([^`\\n]+)`)"

var (
	goGroupRe  = regexp.MustCompile(`(?m)^[ \t]*import\s*\(([^)]*)\)`)
	goLineRe   = regexp.MustCompile(`^` + goSpec)
	goSingleRe = regexp.MustCompile(`(?m)^[ \t]*import\s+` + goSpec)
	goSplitRe  = regexp.MustCompile(`[\n;]`)
)

// goImportsRegex is the fallback for files go/parser cannot handle.
func goImportsRegex(source string) []string {
	code := stripCComments(source)
	var out []string
	add := func(m []string) {
		s := m[1]
		if s == "" {
			s = m[2]
		}
		if s != "" {
			out = append(out, s)
		}
	}
	for _, blk := range goGroupRe.FindAllStringSubmatch(code, -1) {
		for _, line := range goSplitRe.Split(blk[1], -1) {
			if m := goLineRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
				add(m)
			}
		}
	}
	for _, m := range goSingleRe.FindAllStringSubmatch(code, -1) {
		add(m)
	}
	return dedupe(out)
}

var goParser = Parser{
	Exts: []string{"go"},
	Extract: func(_, source string) []string {
		f, err := parser.ParseFile(token.NewFileSet(), "", source, parser.ImportsOnly)
		if err != nil || f == nil {
			return goImportsRegex(source)
		}
		var out []string
		for _, imp := range f.Imports {
			if s, err := strconv.Unquote(imp.Path.Value); err == nil && s != "" {
				out = append(out, s)
			}
		}
		return dedupe(out)
	},
}

// ---- Java ------------------------------------------------------------------

var javaRe = regexp.MustCompile(`(?m)^[ \t]*import\s+(?:static\s+)?([\w$]+(?:\s*\.\s*(?:[\w$]+|\*))*)\s*;`)

var javaParser = Parser{
	Exts: []string{"java"},
	Extract: func(_, source string) []string {
		code := stripCComments(source)
		var out []string
		for _, m := range javaRe.FindAllStringSubmatch(code, -1) {
			out = append(out, pyWSRe.ReplaceAllString(m[1], ""))
		}
		return dedupe(out)
	},
}
