// Package core builds Mermaid diagrams (tree, deps, arch) from a set of files.
package core

// FileNode is a scanned file; Path is posix-style and relative to the root.
type FileNode struct {
	Path string
	Size int64
}

// ImportRef is a raw import specifier found in file From.
type ImportRef struct {
	From string
	Spec string
}

// Edge is a resolved dependency From -> To; Weight counts distinct specifiers.
type Edge struct {
	From   string
	To     string
	Weight int
}

// Options controls rendering.
type Options struct {
	Depth    int // 0 = unset
	MaxNodes int // 0 = unset
}

// DiagramType is one of "tree", "deps", "arch".
type DiagramType string

const (
	Tree DiagramType = "tree"
	Deps DiagramType = "deps"
	Arch DiagramType = "arch"
)

// GoModule is a go.mod module path and the directory containing it ("" = root).
type GoModule struct {
	Dir    string
	Module string
}

// TSConfig is the part of a tsconfig/jsconfig that affects import resolution.
type TSConfig struct {
	Dir     string
	BaseURL *string
	Paths   map[string][]string
	// PathKeys keeps the key order of Paths as written in the file.
	PathKeys []string
}

// ResolveOptions carries project config that changes import resolution.
type ResolveOptions struct {
	GoModules []GoModule
	TSConfigs []TSConfig
}
