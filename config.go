package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlRepository is one explicitly listed repository in the config.
type yamlRepository struct {
	Alias    string `yaml:"alias"`
	RepoPath string `yaml:"repo_path"`
}

// yamlProject groups repositories under a named project.
type yamlProject struct {
	ProjectName  string           `yaml:"project_name"`
	ProjectCode  string           `yaml:"project_code"`
	Directory    string           `yaml:"directory"` // optional prefix match
	Repositories []yamlRepository `yaml:"repositories"`
}

// Config is the parsed YAML configuration file.
type Config struct {
	Projects []yamlProject `yaml:"projects"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	for i, p := range c.Projects {
		if p.ProjectName == "" {
			return fmt.Errorf("projects[%d]: missing project_name", i)
		}
		if p.ProjectCode == "" {
			return fmt.Errorf("projects[%d]: missing project_code", i)
		}
		for j, r := range p.Repositories {
			if r.Alias == "" {
				return fmt.Errorf("projects[%d].repositories[%d]: missing alias", i, j)
			}
			if r.RepoPath == "" {
				return fmt.Errorf("projects[%d].repositories[%d]: missing repo_path", i, j)
			}
		}
	}
	return nil
}

// findMatchingRepo maps a commit path to (projectCode, projectName, alias).
//
// Matching priority:
//  1. `repositories` entries (exact path match or glob pattern)
//  2. `directory` prefix match (alias = repo folder name)
func (c *Config) findMatchingRepo(commitPath string) (projectCode, projectName, alias string, ok bool) {
	for _, p := range c.Projects {
		for _, r := range p.Repositories {
			if pathMatches(r.RepoPath, commitPath) {
				return p.ProjectCode, p.ProjectName, r.Alias, true
			}
		}

		if p.Directory != "" {
			prefix := strings.TrimRight(p.Directory, "/") + "/"
			if strings.HasPrefix(commitPath, prefix) {
				return p.ProjectCode, p.ProjectName, filepath.Base(commitPath), true
			}
		}
	}
	return "", "", "", false
}

// pathMatches matches a configured path pattern against an actual commit path.
//
// Patterns containing glob wildcards (*, ?, [) use glob matching; otherwise
// path-component-aware matching is used — avoids /git/ec falsely matching
// /git/ecology while still matching /git/ec/frontend.
func pathMatches(pattern, path string) bool {
	if hasGlobChars(pattern) {
		re, err := globPatternToRegexp(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(path)
	}
	return pathComponentMatch(pattern, path)
}

// pathComponentMatch checks that one path contains the other at a `/` boundary:
//   - /git/ec/frontend matches /git/ec (pattern is a prefix of path)
//   - /git/ec does NOT match /git/ecology (no component boundary)
//   - /home/user/repo matches /home/user/repo/ (trailing slash ignored)
func pathComponentMatch(pattern, path string) bool {
	p := "/" + strings.Trim(pattern, "/") + "/"
	q := "/" + strings.Trim(path, "/") + "/"
	return strings.Contains(q, p) || strings.Contains(p, q)
}

func hasGlobChars(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// globPatternToRegexp compiles a glob where * and ? cross path separators,
// matching the Rust `glob` crate's default options.
func globPatternToRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	rs := []rune(pattern)
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
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
	return regexp.Compile(b.String())
}
