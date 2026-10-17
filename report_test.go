package main

import (
	"testing"
)

func TestCleanPlainMessage(t *testing.T) {
	got, ok := cleanCommitLine("fix the bug")
	if !ok || got != "fix the bug" {
		t.Errorf("got (%q, %v), want (%q, true)", got, ok, "fix the bug")
	}
}

func TestCleanConventional(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"feat: add login", "add login"},
		{"feat(parser): add support", "add support"},
		{"feat!: breaking change", "breaking change"},
		{"feat(api)!: breaking change", "breaking change"},
		{"Feat: add login", "add login"},
		{"FIX: bug", "bug"},
		{"fix bug\ngit-svn-id: svn://xxx", "fix bug"},
	}
	for _, tt := range tests {
		got, ok := cleanCommitLine(tt.in)
		if !ok || got != tt.want {
			t.Errorf("cleanCommitLine(%q) = (%q, %v), want (%q, true)", tt.in, got, ok, tt.want)
		}
	}
}

func TestCleanEmptyLine(t *testing.T) {
	for _, in := range []string{"", "   "} {
		if got, ok := cleanCommitLine(in); ok {
			t.Errorf("cleanCommitLine(%q) = %q, want not-ok", in, got)
		}
	}
}

func TestCommitSubject(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"feat: hello world", "feat: hello world"},
		{"feat: hello\n\nsome body text\n- bullet", "feat: hello"},
		{"\nfeat: hello\nbody", "feat: hello"},
	}
	for _, tt := range tests {
		if got := commitSubject(tt.in); got != tt.want {
			t.Errorf("commitSubject(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractSquashLines(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want []string
	}{
		{"basic", "feat: main feature\n\nfeat: add login\nfix: correct typo",
			[]string{"feat: add login", "fix: correct typo"}},
		{"skip_bullets", "feat: main\n\n- item one\n- item two\nnormal line",
			[]string{"normal line"}},
		{"skip_plus_star", "feat: main\n\n+ item one\n* item two\nfeat: real commit",
			[]string{"feat: real commit"}},
		{"skip_numbered", "feat: main\n\n1. first item\n2. second item\nfeat: actual commit",
			[]string{"feat: actual commit"}},
		{"skip_empty", "feat: main\n\n\nfeat: second\n\n\nfeat: third",
			[]string{"feat: second", "feat: third"}},
		{"no_body", "feat: simple commit", nil},
		{"all_bullets", "feat: main\n\n- bullet one\n- bullet two\n+ plus item", nil},
		{"skip_git_svn_id", "feat: main\n\nfeat: real commit\ngit-svn-id: svn://xxx\nanother: commit",
			[]string{"feat: real commit", "another: commit"}},
	}
	for _, tt := range tests {
		got := extractSquashLines(tt.msg)
		if len(got) != len(tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
				break
			}
		}
	}
}

func TestSquashChineseCommits(t *testing.T) {
	msg := "feat: 路检路查检测项弹窗展示\n\nfeat: 支持解析JSON格式检测数据\n\ngit-svn-id: https://192.168.4.15/svn/03_project/some-project/vehicle-ui@151857 1f296117-f2e1-c044-9fbd-e3fffb978f54"

	if got := commitSubject(msg); got != "feat: 路检路查检测项弹窗展示" {
		t.Errorf("subject = %q", got)
	}
	if got, ok := cleanCommitLine(commitSubject(msg)); !ok || got != "路检路查检测项弹窗展示" {
		t.Errorf("cleaned subject = (%q, %v)", got, ok)
	}

	lines := extractSquashLines(msg)
	if len(lines) != 1 || lines[0] != "feat: 支持解析JSON格式检测数据" {
		t.Fatalf("squash lines = %v", lines)
	}
	if got, ok := cleanCommitLine(lines[0]); !ok || got != "支持解析JSON格式检测数据" {
		t.Errorf("cleaned squash = (%q, %v)", got, ok)
	}
}

func TestFormatTextSummaryGroupsByPath(t *testing.T) {
	commits := []CommitInfo{
		{Path: "/a", Message: "feat: one"},
		{Path: "/a", Message: "fix: two"},
		{Path: "/b", Message: "chore: three"},
	}
	want := "\nRepository Path: /a\none\ntwo\n\nRepository Path: /b\nthree\n"
	if got := formatTextSummary(commits); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestFormatTextSummaryWithConfig(t *testing.T) {
	cfg := &Config{Projects: []yamlProject{{
		ProjectName:  "Alpha",
		ProjectCode:  "PRJ-001",
		Repositories: []yamlRepository{{Alias: "web_ui", RepoPath: "/a"}},
	}}}
	commits := []CommitInfo{
		{Path: "/a", Message: "feat: add login\n\nfix: css bug"},
		{Path: "/zzz", Message: "orphan commit"},
	}
	got := formatTextSummaryWithConfig(commits, cfg, false)
	want := "" +
		"\nAlpha PRJ-001\n\n" +
		"web_ui\n" +
		"1. add login\n" +
		"2. css bug\n" +
		"=======================================================================\n" +
		"\nUnknown UNKNOWN\n\n" +
		"/zzz\n" +
		"1. orphan commit\n" +
		"\n=======================================================================\n" +
		"Total commits: 3\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
