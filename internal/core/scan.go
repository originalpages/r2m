package core

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultIgnores are skipped everywhere in the tree.
var DefaultIgnores = []string{
	"node_modules", ".git", "dist", "build", ".next", "__pycache__", "venv", "coverage",
}

// ScanDir walks root and returns files sorted by path. It honours DefaultIgnores
// (or ignore, when non-nil) and the root .gitignore. Symlinks and unreadable
// subdirectories are skipped.
func ScanDir(root string, ignore []string) ([]FileNode, error) {
	if ignore == nil {
		ignore = DefaultIgnores
	}
	ig := &Ignorer{}
	ig.Add(ignore...)
	if data, err := os.ReadFile(filepath.Join(root, ".gitignore")); err == nil {
		ig.Add(strings.Split(string(data), "\n")...)
	}

	var out []FileNode
	var walk func(rel string) error
	walk = func(rel string) error {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			if rel == "" {
				return err // unreadable root is an error
			}
			return nil
		}
		for _, e := range entries { // ReadDir returns entries sorted by name
			p := e.Name()
			if rel != "" {
				p = rel + "/" + e.Name()
			}
			switch {
			case e.IsDir():
				if ig.Ignores(p, true) {
					continue
				}
				if err := walk(p); err != nil {
					return err
				}
			case e.Type().IsRegular():
				if ig.Ignores(p, false) {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				out = append(out, FileNode{Path: p, Size: info.Size()})
			}
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	return out, nil
}
