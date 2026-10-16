package data

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// defaultIgnoreDirs are common build/dependency directories hidden by default.
var defaultIgnoreDirs = map[string]bool{
	"build": true, "dist": true, "out": true, "bin": true, "obj": true,
	"target": true, "vendor": true, "node_modules": true, "__pycache__": true,
	"Debug": true, "Release": true, "x64": true, "cmake-build-debug": true,
}

// TreeOptions contains filtering rules for tree generation.
type TreeOptions struct {
	IgnoreHidden      bool
	IgnoreDefaultDirs bool
	ExtraDirs         []string
	Keywords          []string
}

// TreeStat contains cumulative scan statistics.
type TreeStat struct {
	Dirs    int
	Files   int
	Ignored int
}

func (s TreeStat) Total() int { return s.Dirs + s.Files }

// TreeScanResult is one bounded batch from a TreeScanner.
type TreeScanResult struct {
	Text      string
	Stat      TreeStat
	Visited   int
	Done      bool
	Truncated bool
}

type treeFrame struct {
	dir     string
	prefix  string
	entries []fs.DirEntry
	index   int
}

// TreeScanner performs a resumable depth-first directory walk.
//
// Each Next call stops after maxEntries inspected entries while keeping the
// current directory stack and sibling cursor, so a later call can continue
// without restarting from the root.
type TreeScanner struct {
	root              string
	rootName          string
	ignoreHidden      bool
	ignoreDefaultDirs bool
	extraDirs         map[string]bool
	keywords          []string

	stack   []treeFrame
	stat    TreeStat
	visited int
	started bool
	done    bool
}

// NewTreeScanner validates root and constructs a resumable scanner.
func NewTreeScanner(root string, opts TreeOptions) (*TreeScanner, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &fs.PathError{Op: "tree", Path: root, Err: fs.ErrInvalid}
	}

	extra := extraDirSet(opts.ExtraDirs)
	keywords := make([]string, 0, len(opts.Keywords))
	for _, keyword := range opts.Keywords {
		if keyword = strings.TrimSpace(keyword); keyword != "" {
			keywords = append(keywords, keyword)
		}
	}

	rootName := filepath.Base(root)
	if rootName == "" || rootName == "." || rootName == string(filepath.Separator) {
		rootName = root
	}

	return &TreeScanner{
		root:              root,
		rootName:          rootName,
		ignoreHidden:      opts.IgnoreHidden,
		ignoreDefaultDirs: opts.IgnoreDefaultDirs,
		extraDirs:         extra,
		keywords:          keywords,
	}, nil
}

// Next returns the next batch and stops after inspecting maxEntries entries.
// A non-positive maxEntries value scans until the tree is complete.
func (s *TreeScanner) Next(maxEntries int) (TreeScanResult, error) {
	if s == nil || s.done {
		result := TreeScanResult{Done: true}
		if s != nil {
			result.Stat = s.stat
			result.Visited = s.visited
		}
		return result, nil
	}

	var out strings.Builder
	processed := 0
	if !s.started {
		s.started = true
		out.WriteString(s.rootName)
		out.WriteByte('\n')
		s.pushDir(s.root, "")
	}

	for len(s.stack) > 0 {
		if maxEntries > 0 && processed >= maxEntries {
			break
		}

		frame := &s.stack[len(s.stack)-1]
		if frame.index >= len(frame.entries) {
			s.stack = s.stack[:len(s.stack)-1]
			continue
		}

		entry := frame.entries[frame.index]
		frame.index++
		processed++
		s.visited++

		name := entry.Name()
		isDir := entry.IsDir()
		ignored := false
		switch {
		case s.ignoreHidden && strings.HasPrefix(name, "."):
			ignored = true
		case isDir && s.ignoreDefaultDirs && defaultIgnoreDirs[name]:
			ignored = true
		case isDir && s.extraDirs[name]:
			ignored = true
		case hasKeyword(name, s.keywords):
			ignored = true
		}
		if ignored {
			s.stat.Ignored++
			continue
		}

		if isDir {
			s.stat.Dirs++
		} else {
			s.stat.Files++
		}

		last := frame.index == len(frame.entries)
		connector := "├── "
		childPrefix := frame.prefix + "│   "
		if last {
			connector = "└── "
			childPrefix = frame.prefix + "    "
		}
		out.WriteString(frame.prefix)
		out.WriteString(connector)
		out.WriteString(name)
		out.WriteByte('\n')

		if isDir {
			s.pushDir(filepath.Join(frame.dir, name), childPrefix)
		}
	}

	s.done = len(s.stack) == 0
	return TreeScanResult{
		Text:      out.String(),
		Stat:      s.stat,
		Visited:   s.visited,
		Done:      s.done,
		Truncated: !s.done,
	}, nil
}

func (s *TreeScanner) pushDir(dir, prefix string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	s.stack = append(s.stack, treeFrame{
		dir:     dir,
		prefix:  prefix,
		entries: entries,
	})
}

// extraDirSet converts a slice into a case-sensitive lookup set.
func extraDirSet(dirs []string) map[string]bool {
	out := make(map[string]bool, len(dirs))
	for _, dir := range dirs {
		if dir = strings.TrimSpace(dir); dir != "" {
			out[dir] = true
		}
	}
	return out
}

func hasKeyword(name string, keywords []string) bool {
	for _, keyword := range keywords {
		if keyword != "" && strings.Contains(name, keyword) {
			return true
		}
	}
	return false
}

// BuildTree scans the complete tree using the same rules as TreeScanner.
func BuildTree(root string, opts TreeOptions) (string, *TreeStat, error) {
	scanner, err := NewTreeScanner(root, opts)
	if err != nil {
		return "", nil, err
	}

	var out strings.Builder
	for {
		result, err := scanner.Next(0)
		if err != nil {
			return "", nil, err
		}
		out.WriteString(result.Text)
		if result.Done {
			stat := result.Stat
			return out.String(), &stat, nil
		}
	}
}
