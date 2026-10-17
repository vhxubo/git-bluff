package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathMatchesExact(t *testing.T) {
	if !pathMatches("/git/ec/frontend", "/git/ec/frontend") {
		t.Error("exact match failed")
	}
}

func TestPathMatchesSubpath(t *testing.T) {
	if !pathMatches("/git/ec/frontend", "/home/user/git/ec/frontend") {
		t.Error("subpath match failed")
	}
}

func TestPathMatchesTrailingSlash(t *testing.T) {
	if !pathMatches("/home/user/git/ec/frontend", "/home/user/git/ec/frontend/") {
		t.Error("trailing slash match failed")
	}
}

func TestPathNoFalsePrefixMatch(t *testing.T) {
	if pathMatches("/git/ec", "/git/ecology/repo") || pathMatches("/git/ec", "/git/ecology") {
		t.Error("/git/ec should not match /git/ecology")
	}
}

func TestPathMatchesReverseContains(t *testing.T) {
	if !pathMatches("/home/user/git/ec/frontend", "/home/user/git/ec") {
		t.Error("reverse containment failed")
	}
}

func TestPathMatchesGlobStar(t *testing.T) {
	if !pathMatches("/git/ec/*/order-svc", "/git/ec/sub/order-svc") {
		t.Error("glob star should match")
	}
	if pathMatches("/git/ec/*/order-svc", "/git/ec/sub/frontend") {
		t.Error("glob star should not match frontend")
	}
}

func TestPathMatchesGlobQuestion(t *testing.T) {
	if !pathMatches("/git/ec/frontend-?", "/git/ec/frontend-1") {
		t.Error("glob ? should match single char")
	}
	if pathMatches("/git/ec/frontend-?", "/git/ec/frontend-12") {
		t.Error("glob ? should not match two chars")
	}
}

func testConfig() *Config {
	return &Config{Projects: []yamlProject{{
		ProjectName: "Test",
		ProjectCode: "T-1",
		Directory:   "/git/ec",
		Repositories: []yamlRepository{
			{Alias: "custom_alias", RepoPath: "/git/ec/frontend"},
		},
	}}}
}

func TestDirectoryPrefixMatch(t *testing.T) {
	code, name, alias, ok := testConfig().findMatchingRepo("/git/ec/order")
	if !ok || code != "T-1" || name != "Test" || alias != "order" {
		t.Errorf("got (%q,%q,%q,%v)", code, name, alias, ok)
	}
}

func TestDirectoryNoFalsePrefix(t *testing.T) {
	cfg := &Config{Projects: []yamlProject{{
		ProjectName: "Test",
		ProjectCode: "T-1",
		Directory:   "/git/ec",
	}}}
	if _, _, _, ok := cfg.findMatchingRepo("/git/ecology/repo"); ok {
		t.Error("directory prefix should not match /git/ecology")
	}
}

func TestReposHigherPriorityThanDirectory(t *testing.T) {
	_, _, alias, ok := testConfig().findMatchingRepo("/git/ec/frontend")
	if !ok || alias != "custom_alias" {
		t.Errorf("got alias %q, ok %v", alias, ok)
	}
}

func TestGlobRepoPath(t *testing.T) {
	cfg := &Config{Projects: []yamlProject{{
		ProjectName:  "Test",
		ProjectCode:  "T-1",
		Repositories: []yamlRepository{{Alias: "order", RepoPath: "/git/*/order-svc"}},
	}}}
	if _, _, alias, ok := cfg.findMatchingRepo("/git/ec/order-svc"); !ok || alias != "order" {
		t.Errorf("glob repo path should match, got (%q,%v)", alias, ok)
	}
	if _, _, _, ok := cfg.findMatchingRepo("/git/ec/frontend"); ok {
		t.Error("glob repo path should not match frontend")
	}
}

func TestLoadConfigValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")

	if err := os.WriteFile(path, []byte("projects:\n  - project_name: X\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(path); err == nil {
		t.Error("missing project_code should fail validation")
	}

	good := "projects:\n  - project_name: X\n    project_code: XC-1\n    repositories:\n      - alias: a\n        repo_path: /x/a\n"
	if err := os.WriteFile(path, []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(path); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
}
