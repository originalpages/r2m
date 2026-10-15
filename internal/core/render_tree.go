package core

import (
	"fmt"
	"sort"
	"strings"
)

type treeDir struct {
	name  string
	dirs  map[string]*treeDir
	files []string
}

const defaultTreeMaxNodes = 200

func newTreeDir(name string) *treeDir { return &treeDir{name: name, dirs: map[string]*treeDir{}} }

func (d *treeDir) countFiles() int {
	n := len(d.files)
	for _, c := range d.dirs {
		n += c.countFiles()
	}
	return n
}

func buildTree(files []FileNode) *treeDir {
	root := newTreeDir(".")
	for _, f := range files {
		parts := splitNonEmpty(f.Path, "/")
		if len(parts) == 0 {
			continue
		}
		cur := root
		for _, part := range parts[:len(parts)-1] {
			next := cur.dirs[part]
			if next == nil {
				next = newTreeDir(part)
				cur.dirs[part] = next
			}
			cur = next
		}
		cur.files = append(cur.files, parts[len(parts)-1])
	}
	return root
}

// nodeCount is the number of nodes emitted if directories deeper than `limit` are collapsed.
func nodeCount(d *treeDir, level, limit int) int {
	n := 1
	if level >= limit {
		if d.countFiles() > 0 {
			n++
		}
		return n
	}
	n += len(d.files)
	for _, c := range d.dirs {
		n += nodeCount(c, level+1, limit)
	}
	return n
}

func sortedDirs(d *treeDir) []*treeDir {
	out := make([]*treeDir, 0, len(d.dirs))
	for _, c := range d.dirs {
		out = append(out, c)
	}
	sort.SliceStable(out, func(a, b int) bool { return LocaleCompare(out[a].name, out[b].name) < 0 })
	return out
}

const infinity = int(^uint(0) >> 1)

// RenderTree renders the folder/file hierarchy as a `flowchart LR`.
func RenderTree(files []FileNode, opts Options) string {
	maxNodes := opts.MaxNodes
	if maxNodes == 0 {
		maxNodes = defaultTreeMaxNodes
	}
	root := buildTree(files)

	limit := opts.Depth
	if limit == 0 {
		limit = infinity
	}
	if nodeCount(root, 0, limit) > maxNodes {
		// auto-collapse: shrink depth until under the cap (min 1)
		deepest := 0
		var measure func(d *treeDir, l int)
		measure = func(d *treeDir, l int) {
			if l > deepest {
				deepest = l
			}
			for _, c := range d.dirs {
				measure(c, l+1)
			}
		}
		measure(root, 0)
		if deepest < limit {
			limit = deepest
		}
		for limit > 1 && nodeCount(root, 0, limit) > maxNodes {
			limit--
		}
	}

	var lines []string
	lines = append(lines, "flowchart LR")
	id := 0
	next := func() string { s := fmt.Sprintf("n%d", id); id++; return s }

	var emit func(d *treeDir, level int, myID string)
	emit = func(d *treeDir, level int, myID string) {
		if level >= limit {
			if n := d.countFiles(); n > 0 {
				c := next()
				suffix := "s"
				if n == 1 {
					suffix = ""
				}
				lines = append(lines, fmt.Sprintf("  %s[\"…(%d file%s)\"]", c, n, suffix))
				lines = append(lines, fmt.Sprintf("  %s --> %s", myID, c))
			}
			return
		}
		for _, c := range sortedDirs(d) {
			cid := next()
			lines = append(lines, fmt.Sprintf("  %s[\"%s/\"]", cid, Esc(c.name)))
			lines = append(lines, fmt.Sprintf("  %s --> %s", myID, cid))
			emit(c, level+1, cid)
		}
		files := append([]string(nil), d.files...)
		sort.Strings(files)
		for _, f := range files {
			fid := next()
			lines = append(lines, fmt.Sprintf("  %s[\"%s\"]", fid, Esc(f)))
			lines = append(lines, fmt.Sprintf("  %s --> %s", myID, fid))
		}
	}

	rootID := next()
	lines = append(lines, fmt.Sprintf("  %s[\"%s\"]", rootID, Esc(root.name)))
	emit(root, 0, rootID)
	return strings.Join(lines, "\n") + "\n"
}
