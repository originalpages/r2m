// Package cli implements the r2m command line.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/originalpages/r2m/internal/core"
)

// Version is the release version; overridden at build time with -ldflags.
var Version = "0.1.0"

const help = `r2m - show how a project is built, as Mermaid diagrams

Easy mode (plain words, no dashes):
  r2m                       asks what you want to see
  r2m <folder>              everything, in your browser
  r2m <folder> overview     main parts and how they connect
  r2m <folder> deps         which files use which
  r2m <folder> folders      the folder structure
  r2m <folder> deps save    save as a Markdown file instead
  r2m owner/repo            a public GitHub project

Leave <folder> out to use the folder you are in. (A folder that has the same name
as a word, for example "deps", is treated as the folder.)

Options (for scripts; they print Mermaid text):
  -t, --type <tree|deps|arch>  Diagram type (default: tree)
  -d, --depth <N>              Max folder depth
  -n, --max-nodes <N>          Max nodes before collapsing
  -o, --out <file>             Write to file instead of stdout
      --md                     Wrap output in a ` + "```mermaid" + ` fence
      --update <file>          Update the r2m marker block in a Markdown file
      --focus <folder>         With -t deps: show one folder in detail (e.g. src/components)
      --view                   Open the diagrams in your browser
      --no-view                Print text instead of opening the browser
  -h, --help                   Show help
  -v, --version                Show version

Exit codes: 0 ok, 1 runtime error, 2 usage error`

// IO abstracts the process environment so the CLI is testable.
type IO struct {
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	Interactive bool              // attached to a terminal
	Open        func(string) bool // opens a file in the browser; nil = OpenFile
	ViewDir     string            // where to write the HTML page; "" = platform default
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

type options struct {
	typ, out, update    string
	focus               string
	depth, maxNodes     int
	md, view, noView    bool
	help, version       bool
	save                bool
	target              string
	wordKind            string // type chosen with a plain word, "" if none
	allKinds            bool   // the word "all"
	explicitTextOptions bool
}

var intRe = regexp.MustCompile(`^\d+$`)

func posInt(name, v string) (int, error) {
	if !intRe.MatchString(v) {
		return 0, usageError{fmt.Sprintf("--%s must be a positive integer", name)}
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, usageError{fmt.Sprintf("--%s must be a positive integer", name)}
	}
	return n, nil
}

var words = map[string]string{
	"overview": "arch", "arch": "arch",
	"deps": "deps", "dependencies": "deps",
	"folders": "tree", "tree": "tree",
}

func parseArgs(args []string) (*options, error) {
	o := &options{}
	var positionals []string
	needValue := func(flag string, i *int, inline *string) (string, error) {
		if inline != nil {
			return *inline, nil
		}
		if *i+1 >= len(args) {
			return "", usageError{fmt.Sprintf("option %s requires a value", flag)}
		}
		*i++
		return args[*i], nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positionals = append(positionals, a)
			continue
		}
		name := a
		var inline *string
		if strings.HasPrefix(a, "--") {
			if eq := strings.IndexByte(a, '='); eq >= 0 {
				v := a[eq+1:]
				inline, name = &v, a[:eq]
			}
		}
		switch name {
		case "-h", "--help":
			o.help = true
		case "-v", "--version":
			o.version = true
		case "--md":
			o.md, o.explicitTextOptions = true, true
		case "--view":
			o.view = true
		case "--no-view":
			o.noView, o.explicitTextOptions = true, true
		case "-t", "--type":
			v, err := needValue(name, &i, inline)
			if err != nil {
				return nil, err
			}
			o.typ, o.explicitTextOptions = v, true
		case "-d", "--depth":
			v, err := needValue(name, &i, inline)
			if err != nil {
				return nil, err
			}
			if o.depth, err = posInt("depth", v); err != nil {
				return nil, err
			}
		case "-n", "--max-nodes":
			v, err := needValue(name, &i, inline)
			if err != nil {
				return nil, err
			}
			if o.maxNodes, err = posInt("max-nodes", v); err != nil {
				return nil, err
			}
		case "-o", "--out":
			v, err := needValue(name, &i, inline)
			if err != nil {
				return nil, err
			}
			o.out, o.explicitTextOptions = v, true
		case "--focus":
			v, err := needValue(name, &i, inline)
			if err != nil {
				return nil, err
			}
			o.focus, o.explicitTextOptions = v, true
		case "--update":
			v, err := needValue(name, &i, inline)
			if err != nil {
				return nil, err
			}
			o.update, o.explicitTextOptions = v, true
		default:
			return nil, usageError{fmt.Sprintf("unknown option %s", a)}
		}
	}

	// Positionals are plain words (overview, deps, save, ...) or one folder / GitHub reference.
	// A word only counts as a folder when nothing else could be the folder, so a stray
	// folder named "deps" never breaks `r2m my-project deps`.
	isWord := func(p string) bool {
		l := strings.ToLower(p)
		return l == "help" || l == "all" || l == "save" || l == "file" || words[l] != ""
	}
	var targets []string
	for _, p := range positionals {
		if !isWord(p) {
			targets = append(targets, p)
		}
	}
	if len(targets) == 0 {
		for _, p := range positionals {
			if _, err := os.Stat(p); err == nil && isWord(p) {
				targets = append(targets, p) // a folder that happens to be named like a word
				break
			}
		}
	}
	if len(targets) > 1 {
		return nil, usageError{fmt.Sprintf("expected at most one folder or repository, got %q and %q", targets[0], targets[1])}
	}
	for _, p := range positionals {
		if len(targets) == 1 && p == targets[0] && o.target == "" {
			o.target = p
			continue
		}
		switch l := strings.ToLower(p); {
		case l == "help":
			o.help = true
		case l == "all":
			o.allKinds = true
		case l == "save" || l == "file":
			o.save = true
		case words[l] != "":
			o.wordKind = words[l]
		}
	}
	return o, nil
}

// Run executes the CLI and returns the process exit code.
func Run(args []string, io_ IO) int {
	code, err := run(args, io_)
	if err != nil {
		if _, ok := err.(usageError); ok {
			fmt.Fprintf(io_.Stderr, "r2m: %s\nRun with --help for usage.\n", err)
			return 2
		}
		fmt.Fprintf(io_.Stderr, "r2m: %s\n", err)
		return 1
	}
	return code
}

func prompt(r *bufio.Reader, w io.Writer, q string) string {
	fmt.Fprint(w, q)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}

func run(args []string, e IO) (int, error) {
	o, err := parseArgs(args)
	if err != nil {
		return 0, err
	}
	if o.help {
		fmt.Fprintln(e.Stdout, help)
		return 0, nil
	}
	if o.version {
		fmt.Fprintln(e.Stdout, Version)
		return 0, nil
	}

	// No arguments at all in a terminal: ask.
	if len(args) == 0 && e.Interactive && e.Stdin != nil {
		r := bufio.NewReader(e.Stdin)
		o.target = prompt(r, e.Stdout, "Which folder or GitHub project? [press Enter for the current folder] ")
		if k, ok := words[strings.ToLower(o.target)]; ok { // typed "overview" instead of a folder
			o.wordKind, o.target = k, ""
		}
		fmt.Fprintln(e.Stdout, "What do you want to see?")
		fmt.Fprintln(e.Stdout, "  1) Everything (browser)")
		fmt.Fprintln(e.Stdout, "  2) Overview")
		fmt.Fprintln(e.Stdout, "  3) Which files use which")
		fmt.Fprintln(e.Stdout, "  4) Folder structure")
		if o.wordKind == "" {
			switch prompt(r, e.Stdout, "Choose 1-4 [1]: ") {
			case "2":
				o.wordKind = "arch"
			case "3":
				o.wordKind = "deps"
			case "4":
				o.wordKind = "tree"
			}
		}
		switch strings.ToLower(prompt(r, e.Stdout, "Save as a file instead of opening the browser? [y/N] ")) {
		case "y", "yes":
			o.save = true
		}
	}

	typ := o.typ
	if typ == "" {
		typ = o.wordKind
	}
	if typ != "" && typ != "tree" && typ != "deps" && typ != "arch" {
		return 0, usageError{fmt.Sprintf("--type must be one of tree, deps, arch (got %q)", typ)}
	}
	target := o.target
	if target == "" {
		target = "."
	}
	opts := core.Options{Depth: o.depth, MaxNodes: o.maxNodes}

	wordMode := o.wordKind != "" || o.allKinds || o.save
	// Friendly default: in a terminal with no output options, show everything in the browser.
	view := o.view || ((e.Interactive || wordMode) && !o.explicitTextOptions && !o.save)
	if o.noView {
		view = false
	}

	resolved, err := ResolveInput(target)
	if err != nil {
		return 0, err
	}
	resolved.Arg = target
	defer resolved.Cleanup()
	root := resolved.Root
	files, err := core.ScanDir(root, nil)
	if err != nil {
		return 0, err
	}
	read := func(p string) (string, error) {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		return string(b), err
	}

	kinds := []struct {
		typ         core.DiagramType
		title, desc string
	}{
		{core.Arch, "Overview", "Main parts of the project and how they depend on each other. Numbers are import counts."},
		{core.Deps, "Dependencies", "Which files import which, grouped by folder."},
		{core.Tree, "Folders", "The folder and file structure."},
	}
	// "all" (or nothing chosen) means every diagram; a specific type narrows it.
	selected := kinds[:0:0]
	for _, k := range kinds {
		if typ == "" || o.allKinds || string(k.typ) == typ {
			selected = append(selected, k)
		}
	}
	if o.allKinds {
		typ = ""
	}

	switch {
	case o.save:
		return saveMarkdown(e, resolved, selected, typ, files, read, opts)
	case view:
		return viewHTML(e, resolved, selected, files, read, opts)
	}

	// Plain text output (scripts, pipes, explicit options).
	t := core.DiagramType(typ)
	if t == "" && o.focus != "" {
		t = core.Deps
	}
	if t == "" {
		t = core.Tree
	}
	if o.focus != "" && t != core.Deps {
		return 0, usageError{"--focus works with --type deps"}
	}
	var mermaid string
	if o.focus != "" {
		mermaid = core.RenderDepsFocus(core.ResolveEdges(files, read), o.focus, opts)
	} else if mermaid, err = core.Generate(t, files, read, opts); err != nil {
		return 0, err
	}
	if !strings.HasSuffix(mermaid, "\n") {
		mermaid += "\n"
	}
	if o.update != "" {
		existing, err := os.ReadFile(o.update)
		if err != nil && !os.IsNotExist(err) {
			return 0, err
		}
		if err := os.WriteFile(o.update, []byte(UpdateMarkerBlock(string(existing), mermaid)), 0o644); err != nil {
			return 0, err
		}
	}
	rendered := mermaid
	if o.md {
		rendered = "```mermaid\n" + mermaid + "```\n"
	}
	switch {
	case o.out != "":
		if err := os.MkdirAll(filepath.Dir(o.out), 0o755); err != nil {
			return 0, err
		}
		if err := os.WriteFile(o.out, []byte(rendered), 0o644); err != nil {
			return 0, err
		}
	case o.update == "":
		fmt.Fprint(e.Stdout, rendered)
	}
	return 0, nil
}

func quoteIfNeeded(s string) string {
	if strings.ContainsAny(s, " \t") {
		return `"` + s + `"`
	}
	return s
}

type kindT = struct {
	typ         core.DiagramType
	title, desc string
}

func viewHTML(e IO, res *Resolved, kinds []kindT, files []core.FileNode, read core.ReadFunc, opts core.Options) (int, error) {
	name := res.Name
	var edges []core.Edge
	for _, k := range kinds {
		if k.typ != core.Tree {
			edges = core.ResolveEdges(files, read) // parse the project once for both graphs
			break
		}
	}
	var sections []Section
	for _, k := range kinds {
		var src string
		switch k.typ {
		case core.Deps:
			src = core.RenderDeps(edges, opts)
		case core.Arch:
			src = core.RenderArch(edges, files, opts)
		default:
			var err error
			if src, err = core.Generate(k.typ, files, read, opts); err != nil {
				return 0, err
			}
		}
		sec := Section{Title: k.title, Description: k.desc, Source: src}
		if k.typ == core.Deps {
			sec.Variants, sec.Selected = depVariants(edges, opts)
			if len(sec.Variants) > 0 {
				sec.Source = sec.Variants[sec.Selected].Source
			}
		}
		if k.typ == core.Deps && strings.Contains(src, "no internal dependencies found") {
			sec.Empty = "No imports between files were found. r2m reads JavaScript/TypeScript, Python, Go and " +
				"Java/Kotlin imports; for other languages only the Folders diagram is available."
		}
		sections = append(sections, sec)
	}
	dir := e.ViewDir
	if dir == "" {
		dir = viewDir()
	}
	removeStaleViews(dir)
	tmp, err := os.MkdirTemp(dir, "r2m-view-")
	if err != nil {
		return 0, err
	}
	file := filepath.Join(tmp, "diagram.html")
	if err := os.WriteFile(file, []byte(BuildHTML(Page{Name: name, SourceURL: res.SourceURL, Sections: sections})), 0o644); err != nil {
		return 0, err
	}
	open := e.Open
	if open == nil {
		open = OpenFile
	}
	if open(file) {
		fmt.Fprintf(e.Stdout, "Opened diagrams for %q in your browser.\nIf nothing appeared, open this file: %s\n", name, file)
	} else {
		fmt.Fprintf(e.Stdout, "Could not open a browser automatically. Open this file in your browser:\n%s\n", file)
		fmt.Fprintf(e.Stdout, "Tip: \"r2m %s deps save\" writes a Markdown file instead.\n", quoteIfNeeded(res.Arg))
	}
	return 0, nil
}

func saveMarkdown(e IO, res *Resolved, kinds []kindT, typ string, files []core.FileNode, read core.ReadFunc, opts core.Options) (int, error) {
	name := res.Name
	label := typ
	if label == "" {
		label = "all"
	}
	out := fmt.Sprintf("diagram-%s-%s.md", safeName(name), label)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", name)
	if res.SourceURL != "" {
		fmt.Fprintf(&b, "Source: <%s>\n\n", res.SourceURL)
	}
	for _, k := range kinds {
		src, err := core.Generate(k.typ, files, read, opts)
		if err != nil {
			return 0, err
		}
		if len(kinds) > 1 {
			fmt.Fprintf(&b, "## %s\n\n", k.title)
		}
		fmt.Fprintf(&b, "```mermaid\n%s```\n\n", strings.TrimRight(src, "\n")+"\n")
	}
	fmt.Fprintf(&b, "---\nGenerated by [r2m](%s) · %s\n", ProjectURL, ProjectURL)
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return 0, err
	}
	abs, _ := filepath.Abs(out)
	fmt.Fprintf(e.Stdout, "Saved: %s\n", abs)
	return 0, nil
}

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// safeName turns a project name into something safe in a file name on every OS.
func safeName(name string) string {
	n := strings.Trim(unsafeFileChars.ReplaceAllString(name, "-"), "-.")
	if n == "" {
		return "project"
	}
	if len(n) > 60 {
		n = n[:60]
	}
	return n
}

// removeStaleViews deletes pages from earlier runs (older than a day) so project file names
// don't linger in the temp folder.
func removeStaleViews(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "r2m-view-") {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

// A dependency graph is "big" (gets a folder picker) when it has many files or, more often,
// many arrows: arrows are what make the layout wide. readable* describe a view that fits on
// a screen without much scrolling.
const (
	bigGraphFiles     = 40
	bigGraphEdges     = 60
	readableFolderMax = 40 // arrows
)

// depVariants returns the picker entries for a large dependency graph (nil when not needed)
// and the index to show first: the biggest folder that is still readable on its own.
func depVariants(edges []core.Edge, opts core.Options) ([]Variant, int) {
	total := core.CountNodes(edges)
	if total <= bigGraphFiles && len(edges) <= bigGraphEdges {
		return nil, 0
	}
	foci := core.DepsFoci(edges, opts)
	if len(foci) == 0 {
		return nil, 0
	}
	variants := []Variant{{Label: fmt.Sprintf("Everything (%d files, %d links: very wide)", total, len(edges)), Source: core.RenderDeps(edges, opts)}}
	best, bestFiles := -1, -1
	smallest, smallestEdges := -1, 1<<30
	for i, f := range foci {
		variants = append(variants, Variant{Label: f.Label, Source: f.Source})
		if f.Edges <= readableFolderMax && f.Files > bestFiles {
			best, bestFiles = i+1, f.Files
		}
		if f.Edges < smallestEdges {
			smallest, smallestEdges = i+1, f.Edges
		}
	}
	if best < 0 {
		best = smallest
	}
	return variants, best
}
