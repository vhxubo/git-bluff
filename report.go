package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// conventionalRE matches conventional commit subjects `type(scope)!: desc`.
// Group 2 captures the description.
var conventionalRE = regexp.MustCompile(`(?i)^[a-z]+(\([^)]*\))?!?:\s*(.+)$`)

// Report is the generated output.
type Report struct {
	Summary string
}

func generateReport(commits []CommitInfo) *Report {
	return &Report{Summary: formatTextSummary(commits)}
}

func generateReportWithConfig(commits []CommitInfo, cfg *Config, verbose bool) *Report {
	return &Report{Summary: formatTextSummaryWithConfig(commits, cfg, verbose)}
}

// commitSubject returns the first non-empty line of a commit message.
func commitSubject(message string) string {
	for _, line := range strings.Split(message, "\n") {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

// extractSquashLines pulls additional lines from a commit body that look like
// squashed commit subjects: non-empty lines that are not bullets (-, +, *)
// or numbered list items, and not git-svn-id noise.
func extractSquashLines(message string) []string {
	var lines []string
	foundSubject := false

	for _, line := range strings.Split(message, "\n") {
		trimmed := strings.TrimSpace(line)
		if !foundSubject {
			if trimmed != "" {
				foundSubject = true
			}
			continue
		}

		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "git-svn-id:") {
			continue
		}

		isListItem := strings.HasPrefix(trimmed, "-") ||
			strings.HasPrefix(trimmed, "+") ||
			strings.HasPrefix(trimmed, "*") ||
			(trimmed[0] >= '0' && trimmed[0] <= '9')

		if !isListItem {
			lines = append(lines, trimmed)
		}
	}

	return lines
}

// cleanCommitLine strips git-svn-id noise and conventional commit prefixes:
//
//	feat: description
//	feat(scope): description
//	feat!: description            (breaking)
//	feat(scope)!: description     (breaking with scope)
//
// Returns ok=false for lines that end up empty.
func cleanCommitLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}

	// 1. Remove git-svn-id and everything after it.
	base := line
	if i := strings.Index(line, "git-svn-id:"); i >= 0 {
		base = strings.TrimSpace(line[:i])
	}
	if base == "" {
		return "", false
	}

	// 2. Strip conventional commit prefix.
	content := base
	if m := conventionalRE.FindStringSubmatch(base); m != nil {
		content = m[2]
	}

	if content == "" {
		return "", false
	}
	return content, true
}

func formatTextSummary(commits []CommitInfo) string {
	var b strings.Builder
	lastPath := ""

	for i := range commits {
		c := &commits[i]
		if c.Path != lastPath {
			fmt.Fprintf(&b, "\nRepository Path: %s\n", c.Path)
			lastPath = c.Path
		}

		if cleaned, ok := cleanCommitLine(commitSubject(c.Message)); ok {
			b.WriteString(cleaned)
			b.WriteByte('\n')
		}
	}

	return b.String()
}

// groupKey orders report sections the way the Rust BTreeMap did:
// by (project_code, project_name, alias).
type groupKey struct {
	projectCode string
	projectName string
	alias       string
}

func formatTextSummaryWithConfig(commits []CommitInfo, cfg *Config, verbose bool) string {
	var out strings.Builder

	grouped := map[groupKey][]*CommitInfo{}
	var keys []groupKey
	for i := range commits {
		c := &commits[i]
		var key groupKey
		if code, name, alias, ok := cfg.findMatchingRepo(c.Path); ok {
			key = groupKey{code, name, alias}
		} else {
			key = groupKey{"UNKNOWN", "Unknown", c.Path}
		}
		if _, seen := grouped[key]; !seen {
			keys = append(keys, key)
		}
		grouped[key] = append(grouped[key], c)
	}

	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.projectCode != b.projectCode {
			return a.projectCode < b.projectCode
		}
		if a.projectName != b.projectName {
			return a.projectName < b.projectName
		}
		return a.alias < b.alias
	})

	currentProject := ""
	totalLines := 0
	for _, key := range keys {
		commitList := grouped[key]
		if key.projectCode != currentProject {
			if currentProject != "" {
				out.WriteString("=======================================================================\n")
			}
			fmt.Fprintf(&out, "\n%s %s\n\n", key.projectName, key.projectCode)
			currentProject = key.projectCode
		} else {
			out.WriteString("\n")
		}

		if verbose {
			pathStr := ""
			if len(commitList) > 0 {
				pathStr = fmt.Sprintf(" (%s)", commitList[0].Path)
			}
			fmt.Fprintf(&out, "Repository: %s%s\n", key.alias, pathStr)
		} else {
			fmt.Fprintf(&out, "%s\n", key.alias)
		}

		lineNum := 0
		for _, c := range commitList {
			if cleaned, ok := cleanCommitLine(commitSubject(c.Message)); ok {
				lineNum++
				fmt.Fprintf(&out, "%d. %s\n", lineNum, cleaned)
			}
			for _, sq := range extractSquashLines(c.Message) {
				if cleaned, ok := cleanCommitLine(sq); ok {
					lineNum++
					fmt.Fprintf(&out, "%d. %s\n", lineNum, cleaned)
				}
			}
		}
		totalLines += lineNum
	}

	fmt.Fprintf(&out, "\n=======================================================================\nTotal commits: %d\n", totalLines)
	return out.String()
}
