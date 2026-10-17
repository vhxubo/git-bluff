package main

import (
	"os"
	"path/filepath"
	"testing"
)

// mkRepo creates a directory that looks like a git repository (.git/HEAD).
func mkRepo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The original Rust binary visits N+1 levels below the start directory for
// --depth N; these expectations were verified against it.
func TestFindGitRepositoriesDepth(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"a", "a/nested", "a/nested/deep", "b", "b/x/y", ".hidden", "ignored"} {
		mkRepo(t, filepath.Join(root, d))
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		depth int
		want  []string
	}{
		{0, nil}, // root itself is not a repo
		{1, []string{"a", "a/nested", "b"}},
		{2, []string{"a", "a/nested", "a/nested/deep", "b", "b/x/y"}},
	}
	for _, tt := range tests {
		repos, err := findGitRepositories(root, tt.depth)
		if err != nil {
			t.Fatalf("depth %d: %v", tt.depth, err)
		}
		got := make([]string, len(repos))
		for i, r := range repos {
			rel, err := filepath.Rel(root, r)
			if err != nil {
				t.Fatal(err)
			}
			got[i] = rel
		}
		if len(got) != len(tt.want) {
			t.Errorf("depth %d: got %v, want %v", tt.depth, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("depth %d: got %v, want %v", tt.depth, got, tt.want)
				break
			}
		}
	}
}

func TestFindGitRepositoriesRootIsRepo(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, root)

	repos, err := findGitRepositories(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || filepath.Clean(repos[0]) != filepath.Clean(root) {
		t.Errorf("got %v", repos)
	}
}

func TestFindGitRepositoriesNestedGitignore(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, filepath.Join(root, "proj"))
	mkRepo(t, filepath.Join(root, "proj", "vendor")) // ignored by proj/.gitignore
	if err := os.WriteFile(filepath.Join(root, "proj", ".gitignore"), []byte("vendor/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repos, err := findGitRepositories(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || filepath.Base(repos[0]) != "proj" {
		t.Errorf("got %v, want only proj", repos)
	}
}

func TestParseIgnoreLine(t *testing.T) {
	tests := []struct {
		line     string
		want     bool
		negated  bool
		dirOnly  bool
		anchored bool
	}{
		{"", false, false, false, false},
		{"# comment", false, false, false, false},
		{"target/", true, false, true, false},
		{"!keep", true, true, false, false},
		{"/build", true, false, false, true},
		{"a/b/c", true, false, false, true},
		{"node_modules", true, false, false, false},
	}
	for _, tt := range tests {
		p, ok := parseIgnoreLine(tt.line)
		if ok != tt.want {
			t.Errorf("parseIgnoreLine(%q) ok = %v, want %v", tt.line, ok, tt.want)
			continue
		}
		if !ok {
			continue
		}
		if p.negated != tt.negated || p.dirOnly != tt.dirOnly || p.anchored != tt.anchored {
			t.Errorf("parseIgnoreLine(%q) = %+v", tt.line, p)
		}
	}
}

func TestIgnorePatternMatches(t *testing.T) {
	tests := []struct {
		pattern string
		comps   []string
		isDir   bool
		want    bool
	}{
		{"target", []string{"target"}, true, true},
		{"target", []string{"a", "target"}, true, true},   // unanchored matches any depth
		{"/target", []string{"a", "target"}, true, false}, // anchored
		{"target/", []string{"target"}, false, false},     // dir-only vs file
		{"*.log", []string{"debug.log"}, false, true},
		{"*.log", []string{"dir", "debug.log"}, false, true},
		{"logs/**/x", []string{"logs", "a", "b", "x"}, true, true},
		{"logs/*", []string{"logs", "a", "b"}, true, false}, // * does not cross separators
		{"d?fault", []string{"default"}, false, true},
		{"[abc]at", []string{"bat"}, false, true},
		{"[!a]at", []string{"bat"}, false, true},
		{"[!a]at", []string{"aat"}, false, false},
	}
	for _, tt := range tests {
		p, ok := parseIgnoreLine(tt.pattern)
		if !ok {
			t.Fatalf("parseIgnoreLine(%q) not ok", tt.pattern)
		}
		if got := p.matches(tt.comps, tt.isDir); got != tt.want {
			t.Errorf("pattern %q vs %v (isDir=%v) = %v, want %v", tt.pattern, tt.comps, tt.isDir, got, tt.want)
		}
	}
}

func TestGitIgnoreFileLastMatchWins(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n!keep.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := loadGitIgnoreFile(filepath.Join(dir, ".gitignore"))
	if f == nil {
		t.Fatal("gitignore not loaded")
	}
	if _, ignored := f.match([]string{"x.log"}, false); !ignored {
		t.Error("x.log should be ignored")
	}
	if _, ignored := f.match([]string{"keep.log"}, false); ignored {
		t.Error("keep.log should be un-ignored by !keep.log")
	}
}

func TestIgnoreStackNearestFileWins(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Root ignores all logs; sub's own .gitignore un-ignores debug.log only.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", ".gitignore"), []byte("!debug.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newIgnoreStack(root)
	if !s.ignored(filepath.Join(root, "app.log"), false) {
		t.Error("app.log should be ignored by root .gitignore")
	}
	if s.ignored(filepath.Join(root, "sub", "debug.log"), false) {
		t.Error("sub/debug.log should be un-ignored by nearest .gitignore")
	}
	if !s.ignored(filepath.Join(root, "sub", "other.log"), false) {
		t.Error("sub/other.log should stay ignored: !debug.log does not match it")
	}
}
