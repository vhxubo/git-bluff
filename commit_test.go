package main

import (
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// shanghai overrides time.Local for the test and restores the original after.
func shanghai(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load Asia/Shanghai: %v", err)
	}
	orig := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = orig })
	return loc
}

type testCommit struct {
	when    time.Time
	message string
	author  string
	parents int // 2 = merge commit taking the two previous tips as parents
}

// buildRepo creates an on-disk repository containing the given commits.
func buildRepo(t *testing.T, dir string, commits ...testCommit) {
	t.Helper()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	storer := repo.Storer

	// A shared empty tree keeps every commit valid.
	treeObj := storer.NewEncodedObject()
	if err := (&object.Tree{}).Encode(treeObj); err != nil {
		t.Fatal(err)
	}
	treeHash, err := storer.SetEncodedObject(treeObj)
	if err != nil {
		t.Fatal(err)
	}

	var tips []plumbing.Hash
	for _, c := range commits {
		var parents []plumbing.Hash
		switch {
		case c.parents >= 2:
			parents = append(parents, tips[len(tips)-1], tips[len(tips)-2])
		case len(tips) > 0:
			parents = append(parents, tips[len(tips)-1])
		}

		obj := storer.NewEncodedObject()
		commit := &object.Commit{
			Author:       object.Signature{Name: c.author, Email: "x@y.z", When: c.when},
			Committer:    object.Signature{Name: c.author, Email: "x@y.z", When: c.when},
			Message:      c.message,
			TreeHash:     treeHash,
			ParentHashes: parents,
		}
		if err := commit.Encode(obj); err != nil {
			t.Fatal(err)
		}
		h, err := storer.SetEncodedObject(obj)
		if err != nil {
			t.Fatal(err)
		}
		tips = append(tips, h)
		if err := storer.SetReference(plumbing.NewHashReference("refs/heads/main", h)); err != nil {
			t.Fatal(err)
		}
	}
	if err := storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main")); err != nil {
		t.Fatal(err)
	}
}

// The original Rust build bucketed commits by UTC day, so commits made
// between midnight and +08:00 landed on the previous day. This test pins the
// fixed behavior: days are computed in the local timezone.
func TestGetCommitsLocalTimezoneBuckets(t *testing.T) {
	shanghai(t)
	dir := t.TempDir()

	at := func(day, hh, mm string) time.Time {
		tt, err := time.Parse("2006-01-02 15:04 -0700", day+" "+hh+":"+mm+" +0800")
		if err != nil {
			t.Fatal(err)
		}
		return tt
	}

	// 01:30 on Jan 17 CST == 17:30 UTC on Jan 16: the old UTC-day logic lost it.
	buildRepo(t, dir,
		testCommit{at("2026-01-16", "20", "00"), "feat: evening before", "alice", 1},
		testCommit{at("2026-01-17", "01", "30"), "fix: after midnight", "alice", 1},
		testCommit{at("2026-01-17", "10", "00"), "feat: mid morning", "bob", 1},
	)

	day, err := parseDate("2026-01-17")
	if err != nil {
		t.Fatal(err)
	}
	commits, err := getCommits(dir, day, day, nil)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]string, len(commits))
	for i, c := range commits {
		got[i] = c.Message
	}
	want := []string{"fix: after midnight", "feat: mid morning"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGetCommitsAuthorFilter(t *testing.T) {
	loc := shanghai(t)
	dir := t.TempDir()
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, loc)

	buildRepo(t, dir,
		testCommit{base, "feat: by Alice", "Alice Smith", 1},
		testCommit{base.Add(time.Hour), "feat: by Bob", "bob jones", 1},
	)

	day := localDay(base)
	commits, err := getCommits(dir, day, day, []string{"alice", "Carol"})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].AuthorName != "Alice Smith" {
		t.Errorf("author filter got %v", commits)
	}
}

func TestGetCommitsSkipsMergeCommits(t *testing.T) {
	loc := shanghai(t)
	dir := t.TempDir()
	base := time.Date(2026, 3, 2, 12, 0, 0, 0, loc)

	buildRepo(t, dir,
		testCommit{base, "feat: first", "alice", 1},
		testCommit{base.Add(time.Minute), "feat: second", "alice", 1},
		testCommit{base.Add(2 * time.Minute), "merge: whatever", "alice", 2},
	)

	day := localDay(base)
	commits, err := getCommits(dir, day, day, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		for _, c := range commits {
			t.Logf("kept: %q", c.Message)
		}
		t.Errorf("got %d commits, want 2 (merge skipped)", len(commits))
	}
}

func TestGetCommitsEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := git.PlainInit(dir, false); err != nil {
		t.Fatal(err)
	}
	day := localDay(time.Now())
	if _, err := getCommits(dir, day, day, nil); err == nil {
		t.Error("empty repo should return an error (caller warns and skips)")
	}
}
