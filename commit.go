package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CommitInfo carries the subset of commit data used by the report.
type CommitInfo struct {
	ID          string
	AuthorName  string
	AuthorEmail string
	Message     string
	Timestamp   time.Time
	Repository  string
	Path        string
}

// getCommits collects non-merge commits from a repository whose local commit
// day falls within [startDay, endDay], optionally filtered by author
// (case-insensitive substring match, any-of). Results are ordered oldest
// first, matching the original's reverse-time revwalk.
func getCommits(repoPath string, startDay, endDay time.Time, authors []string) ([]CommitInfo, error) {
	fullPath, err := canonicalize(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get full path %s: %w", repoPath, err)
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository %s: %w", repoPath, err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve HEAD of %s: %w", repoPath, err)
	}

	iter, err := repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, err
	}

	repoName := filepath.Base(repoPath)

	lowerAuthors := make([]string, len(authors))
	for i, a := range authors {
		lowerAuthors[i] = strings.ToLower(a)
	}

	var commits []CommitInfo
	err = iter.ForEach(func(c *object.Commit) error {
		if c.NumParents() > 1 {
			return nil // skip merge commits
		}

		day := localDay(c.Committer.When)
		if day.Before(startDay) || day.After(endDay) {
			return nil
		}

		if len(lowerAuthors) > 0 {
			author := strings.ToLower(c.Author.Name)
			matched := false
			for _, a := range lowerAuthors {
				if strings.Contains(author, a) {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		commits = append(commits, CommitInfo{
			ID:          c.Hash.String(),
			AuthorName:  c.Author.Name,
			AuthorEmail: c.Author.Email,
			Message:     c.Message,
			Timestamp:   c.Committer.When,
			Repository:  repoName,
			Path:        fullPath,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(commits, func(i, j int) bool {
		return commits[i].Timestamp.Before(commits[j].Timestamp)
	})

	return commits, nil
}

func canonicalize(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil // keep the absolute path if symlinks can't be resolved
	}
	return resolved, nil
}
