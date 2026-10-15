package core

import (
	"regexp"
	"strings"
)

type ignoreRule struct {
	re      *regexp.Regexp
	negate  bool
	dirOnly bool
}

// Ignorer implements the commonly used subset of .gitignore semantics.
type Ignorer struct {
	rules []ignoreRule
}

// Add parses gitignore-style pattern lines.
func (ig *Ignorer) Add(lines ...string) {
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		// Trailing spaces are ignored unless escaped with a backslash.
		for strings.HasSuffix(line, " ") && !strings.HasSuffix(line, "\\ ") {
			line = line[:len(line)-1]
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r := ignoreRule{}
		if strings.HasPrefix(line, "!") {
			r.negate = true
			line = line[1:]
		} else if strings.HasPrefix(line, "\\!") || strings.HasPrefix(line, "\\#") {
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			r.dirOnly = true
			line = strings.TrimRight(line, "/")
		}
		if line == "" {
			continue
		}
		anchored := strings.Contains(line, "/")
		line = strings.TrimPrefix(line, "/")
		prefix := "(?:^|/)"
		if anchored {
			prefix = "^"
		}
		re, err := regexp.Compile(prefix + globToRegex(line) + "$")
		if err != nil {
			continue // invalid pattern: ignore it, like git does with unusable lines
		}
		r.re = re
		ig.rules = append(ig.rules, r)
	}
}

// Ignores reports whether the posix path (relative to the root) is ignored.
// The last matching rule wins; callers prune ignored directories while walking.
func (ig *Ignorer) Ignores(path string, isDir bool) bool {
	ignored := false
	for _, r := range ig.rules {
		if r.dirOnly && !isDir {
			continue
		}
		if r.re.MatchString(path) {
			ignored = !r.negate
		}
	}
	return ignored
}

func globToRegex(g string) string {
	var b strings.Builder
	for i := 0; i < len(g); i++ {
		c := g[i]
		switch c {
		case '*':
			if i+1 < len(g) && g[i+1] == '*' {
				atStart := i == 0 || g[i-1] == '/'
				j := i + 2
				if atStart && j < len(g) && g[j] == '/' {
					b.WriteString("(?:.*/)?")
					i = j
				} else if atStart && j == len(g) {
					b.WriteString(".*")
					i = j - 1
				} else {
					b.WriteString(".*")
					i = j - 1
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '[':
			end := strings.IndexByte(g[i+1:], ']')
			if end < 0 {
				b.WriteString(`\[`)
				continue
			}
			class := g[i+1 : i+1+end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteString("[" + class + "]")
			i += end + 1
		case '\\':
			if i+1 < len(g) {
				i++
				b.WriteString(regexp.QuoteMeta(string(g[i])))
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return b.String()
}
