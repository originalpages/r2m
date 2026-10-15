package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

const concurrency = 16

// maxParseSize skips very large files (minified bundles, generated code): they are slow to
// scan and carry no useful structure. A Size of 0 means unknown and is still read.
const maxParseSize = 2 << 20

// ConfigFiles are basenames of project config files that affect import resolution.
var ConfigFiles = []string{"go.mod", "tsconfig.json", "jsconfig.json"}

// ReadFunc returns the text of a scanned file (path relative to the root).
type ReadFunc func(path string) (string, error)

func baseOf(p string) string { return p[strings.LastIndexByte(p, '/')+1:] }

func dirOf(p string) string {
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return ""
	}
	return p[:i]
}

var goModRe = regexp.MustCompile(`(?m)^[ \t]*module[ \t]+"?([^\s"]+)"?`)

var trailingCommaRe = regexp.MustCompile(`,(\s*[}\]])`)

// stripJSONC removes // and /* */ comments outside of strings (tsconfig flavour).
func stripJSONC(s string) string {
	var b strings.Builder
	n := len(s)
	for i := 0; i < n; {
		c := s[i]
		switch {
		case c == '"':
			j := i + 1
			for j < n && s[j] != '"' {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			if j >= n {
				j = n - 1
			}
			b.WriteString(s[i : j+1])
			i = j + 1
		case c == '/' && i+1 < n && s[i+1] == '/':
			for i < n && s[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && s[i+1] == '*':
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				i = n
			} else {
				i += 2 + end + 2
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return trailingCommaRe.ReplaceAllString(b.String(), "$1")
}

func loadResolveOptions(files []FileNode, read ReadFunc) ResolveOptions {
	var opts ResolveOptions
	for _, f := range files {
		name := baseOf(f.Path)
		if !contains(ConfigFiles, name) {
			continue
		}
		text, err := read(f.Path)
		if err != nil {
			continue
		}
		if name == "go.mod" {
			if m := goModRe.FindStringSubmatch(text); m != nil {
				opts.GoModules = append(opts.GoModules, GoModule{Dir: dirOf(f.Path), Module: m[1]})
			}
			continue
		}
		var raw struct {
			CompilerOptions struct {
				BaseURL *string                    `json:"baseUrl"`
				Paths   map[string]json.RawMessage `json:"paths"`
			} `json:"compilerOptions"`
		}
		clean := stripJSONC(text)
		if json.Unmarshal([]byte(clean), &raw) != nil {
			continue // malformed config: fall back to heuristics
		}
		cfg := TSConfig{Dir: dirOf(f.Path), BaseURL: raw.CompilerOptions.BaseURL, Paths: map[string][]string{}}
		cfg.PathKeys = orderedKeys(clean)
		for k, v := range raw.CompilerOptions.Paths {
			var targets []string
			if json.Unmarshal(v, &targets) == nil {
				cfg.Paths[k] = targets
			}
		}
		// keep only keys that parsed as string lists
		var keys []string
		for _, k := range cfg.PathKeys {
			if _, ok := cfg.Paths[k]; ok {
				keys = append(keys, k)
			}
		}
		cfg.PathKeys = keys
		opts.TSConfigs = append(opts.TSConfigs, cfg)
	}
	return opts
}

// orderedKeys returns the keys of compilerOptions.paths in file order.
func orderedKeys(jsonText string) []string {
	var doc struct {
		CompilerOptions struct {
			Paths json.RawMessage `json:"paths"`
		} `json:"compilerOptions"`
	}
	if json.Unmarshal([]byte(jsonText), &doc) != nil || len(doc.CompilerOptions.Paths) == 0 {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(string(doc.CompilerOptions.Paths)))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil
	}
	var keys []string
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			break
		}
		k, _ := t.(string)
		keys = append(keys, k)
		var skip json.RawMessage
		if dec.Decode(&skip) != nil {
			break
		}
	}
	return keys
}

// Generate builds a diagram of the given type. It is environment-agnostic: the caller
// supplies the file list and a reader.
func Generate(t DiagramType, files []FileNode, read ReadFunc, opts Options) (string, error) {
	switch t {
	case Tree:
		return RenderTree(files, opts), nil
	case Deps:
		return RenderDeps(ResolveEdges(files, read), opts), nil
	case Arch:
		return RenderArch(ResolveEdges(files, read), files, opts), nil
	}
	return "", fmt.Errorf("unknown diagram type: %s", t)
}

// ResolveEdges parses all supported files and returns the resolved file->file edges.
func ResolveEdges(files []FileNode, read ReadFunc) []Edge {
	type job struct {
		path   string
		parser *Parser
	}
	var jobs []job
	for _, f := range files {
		if f.Size > maxParseSize {
			continue
		}
		if p := GetParser(f.Path); p != nil {
			jobs = append(jobs, job{f.Path, p})
		}
	}

	var (
		mu   sync.Mutex
		refs []ImportRef
		wg   sync.WaitGroup
	)
	ch := make(chan job)
	workers := concurrency
	if len(jobs) < workers {
		workers = len(jobs)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				src, err := read(j.path)
				if err != nil {
					continue // unreadable file: skip
				}
				specs := safeExtract(j.parser, j.path, src)
				mu.Lock()
				for _, s := range specs {
					refs = append(refs, ImportRef{From: j.path, Spec: s})
				}
				mu.Unlock()
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()

	return ResolveImports(files, refs, loadResolveOptions(files, read))
}

// safeExtract guards against parser panics on pathological input.
func safeExtract(p *Parser, path, src string) (specs []string) {
	defer func() {
		if recover() != nil {
			specs = nil
		}
	}()
	return p.Extract(path, src)
}
