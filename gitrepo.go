package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// findGitRepositories walks a directory and collects git repositories.
//
// Mirrors the original Rust behavior: --depth N visits entries up to N+1
// levels below the starting directory, hidden entries are skipped, and
// .gitignore files found along the way are honored.
func findGitRepositories(directory string, depth int) ([]string, error) {
	var repos []string

	if depth == 0 {
		if isGitRepository(directory) {
			repos = append(repos, directory)
		}
		return repos, nil
	}

	root := filepath.Clean(directory)
	ignores := newIgnoreStack(root)
	maxLevel := depth + 1 // the walk root itself is level 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			// Unreadable entries are skipped, like the Rust `ignore` crate.
			return nil
		}

		if path != root && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		if pathLevel(root, path) > maxLevel {
			return filepath.SkipDir
		}

		if ignores.ignored(path, true) {
			return filepath.SkipDir
		}

		if isGitRepository(path) {
			repos = append(repos, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(repos)
	repos = dedupSorted(repos)

	return repos, nil
}

// pathLevel returns how many levels path sits below root (root = 0).
func pathLevel(root, path string) int {
	rel := strings.TrimPrefix(path, root)
	rel = strings.TrimPrefix(rel, string(filepath.Separator))
	if rel == "" {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator)) + 1
}

func isGitRepository(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = os.Stat(filepath.Join(gitDir, "HEAD"))
	return err == nil
}

func dedupSorted(paths []string) []string {
	out := paths[:0]
	for i, p := range paths {
		if i > 0 && p == paths[i-1] {
			continue
		}
		out = append(out, p)
	}
	return out
}

// --- minimal .gitignore support -------------------------------------------
//
// The Rust original leaned on the `ignore` crate for this. Here we implement
// just enough gitignore semantics for directory scanning: globs (*, ?, [...]),
// negation (!), dir-only patterns (trailing /), anchoring (leading / or
// embedded /), and ** segments. The nearest .gitignore with a matching
// pattern wins.

type ignoreSegment struct {
	doubleStar bool
	re         *regexp.Regexp // nil when doubleStar
}

type ignorePattern struct {
	negated  bool
	dirOnly  bool
	anchored bool
	segments []ignoreSegment
}

type gitIgnoreFile struct {
	patterns []ignorePattern
}

type ignoreStack struct {
	root  string
	files map[string]*gitIgnoreFile // dir -> parsed .gitignore (nil = none)
}

func newIgnoreStack(root string) *ignoreStack {
	return &ignoreStack{root: root, files: map[string]*gitIgnoreFile{}}
}

// ignored reports whether path should be skipped, consulting the .gitignore
// files of its ancestor directories from nearest to farthest.
func (s *ignoreStack) ignored(path string, isDir bool) bool {
	sep := string(filepath.Separator)
	for dir := filepath.Dir(path); dir == s.root || strings.HasPrefix(dir, s.root+sep); dir = filepath.Dir(dir) {
		if f := s.fileFor(dir); f != nil {
			rel, err := filepath.Rel(dir, path)
			if err == nil {
				if matched, ign := f.match(strings.Split(rel, sep), isDir); matched {
					return ign
				}
			}
		}
		if dir == s.root {
			break
		}
	}
	return false
}

func (s *ignoreStack) fileFor(dir string) *gitIgnoreFile {
	if f, ok := s.files[dir]; ok {
		return f
	}
	f := loadGitIgnoreFile(filepath.Join(dir, ".gitignore"))
	s.files[dir] = f
	return f
}

func loadGitIgnoreFile(path string) *gitIgnoreFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	f := &gitIgnoreFile{}
	for _, line := range strings.Split(string(data), "\n") {
		if p, ok := parseIgnoreLine(strings.TrimSuffix(line, "\r")); ok {
			f.patterns = append(f.patterns, p)
		}
	}
	if len(f.patterns) == 0 {
		return nil
	}
	return f
}

// match applies git's "last matching pattern wins" rule within one file.
func (f *gitIgnoreFile) match(comps []string, isDir bool) (matched, ignored bool) {
	for _, p := range f.patterns {
		if p.matches(comps, isDir) {
			matched = true
			ignored = !p.negated
		}
	}
	return matched, ignored
}

func parseIgnoreLine(line string) (ignorePattern, bool) {
	var p ignorePattern
	line = strings.TrimRight(line, " \t")
	if line == "" || strings.HasPrefix(line, "#") {
		return p, false
	}
	if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\!") {
		line = line[1:]
	}
	if strings.HasPrefix(line, "!") {
		p.negated = true
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		p.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if strings.HasPrefix(line, "/") {
		p.anchored = true
		line = strings.TrimPrefix(line, "/")
	} else if strings.Contains(line, "/") {
		p.anchored = true
	}
	if line == "" {
		return p, false
	}
	for _, seg := range strings.Split(line, "/") {
		switch seg {
		case "":
			continue
		case "**":
			p.segments = append(p.segments, ignoreSegment{doubleStar: true})
		default:
			p.segments = append(p.segments, ignoreSegment{re: compileGlob(seg)})
		}
	}
	return p, len(p.segments) > 0
}

func (p *ignorePattern) matches(comps []string, isDir bool) bool {
	if p.dirOnly && !isDir {
		return false
	}
	segs := p.segments
	if !p.anchored {
		// Unanchored patterns match at any depth.
		segs = append([]ignoreSegment{{doubleStar: true}}, segs...)
	}
	return matchSegments(segs, comps)
}

func matchSegments(segs []ignoreSegment, comps []string) bool {
	if len(segs) == 0 {
		return len(comps) == 0
	}
	if segs[0].doubleStar {
		for i := 0; i <= len(comps); i++ {
			if matchSegments(segs[1:], comps[i:]) {
				return true
			}
		}
		return false
	}
	if len(comps) == 0 || !segs[0].re.MatchString(comps[0]) {
		return false
	}
	return matchSegments(segs[1:], comps[1:])
}

// compileGlob compiles a single path segment glob to a regexp.
// *, ? and [...] are supported; unlike full gitignore semantics a segment
// never spans a path separator.
func compileGlob(seg string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	rs := []rune(seg)
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '*':
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '[':
			j := i + 1
			if j < len(rs) && (rs[j] == '!' || rs[j] == '^') {
				j++
			}
			if j < len(rs) && rs[j] == ']' {
				j++
			}
			for j < len(rs) && rs[j] != ']' {
				j++
			}
			if j >= len(rs) {
				b.WriteString(regexp.QuoteMeta(string(rs[i])))
				continue
			}
			inner := string(rs[i+1 : j])
			if strings.HasPrefix(inner, "!") {
				inner = "^" + inner[1:]
			}
			b.WriteString("[" + inner + "]")
			i = j
		default:
			b.WriteString(regexp.QuoteMeta(string(rs[i])))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}
