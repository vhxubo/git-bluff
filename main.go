// git-bluff generates formatted daily reports from git commits.
package main

import (
	"fmt"
	"os"
	"time"

	// Embed the timezone database so local-day computation works on systems
	// without zoneinfo (e.g. scratch containers, minimal Windows installs).
	_ "time/tzdata"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "0.3.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	args, err := parseArgs(argv)
	if err != nil {
		if err == errHelp {
			return nil
		}
		return err
	}
	if args.Version {
		fmt.Printf("git-bluff %s\n", version)
		return nil
	}

	if args.Date != "" && (args.From != "" || args.To != "") {
		return fmt.Errorf("--date cannot be used with --from/--to")
	}
	if args.To != "" && args.From == "" {
		return fmt.Errorf("--to must be used with --from")
	}

	// Resolve the date window, defaulting to today.
	today := localDay(time.Now())
	startDay, endDay := today, today
	switch {
	case args.Date != "":
		if startDay, err = parseDate(args.Date); err != nil {
			return err
		}
		endDay = startDay
	case args.From != "" && args.To != "":
		if startDay, err = parseDate(args.From); err != nil {
			return err
		}
		if endDay, err = parseDate(args.To); err != nil {
			return err
		}
	case args.From != "":
		if startDay, err = parseDate(args.From); err != nil {
			return err
		}
		endDay = today
	}

	if args.Verbose {
		fmt.Printf("📂 Scanning directory: %s\n", args.Directory)
		if args.Depth > 0 {
			fmt.Printf("🔄 Max depth: %d\n", args.Depth)
		}
		if args.From != "" && args.To != "" {
			fmt.Printf("📅 Date range: %s to %s\n", args.From, args.To)
		} else if args.Date != "" {
			fmt.Printf("📅 Date filter: %s\n", args.Date)
		}
		if args.Config != "" {
			fmt.Printf("📄 Config file: %s\n", args.Config)
		}
		fmt.Println()
	}

	var cfg *Config
	if args.Config != "" {
		if cfg, err = loadConfig(args.Config); err != nil {
			return fmt.Errorf("failed to load config %s: %w", args.Config, err)
		}
	}

	repos, err := findGitRepositories(args.Directory, args.Depth)
	if err != nil {
		return err
	}

	if args.Verbose {
		fmt.Printf("✅ Found %d git repository(ies)\n\n", len(repos))
	}

	var allCommits []CommitInfo
	for _, repoPath := range repos {
		if args.Verbose {
			fmt.Printf("📦 Processing: %s\n", repoPath)
		}

		commits, err := getCommits(repoPath, startDay, endDay, args.Authors)
		if err != nil {
			// A missing or empty repository should not kill the whole report.
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", repoPath, err)
			continue
		}

		if args.Verbose && len(commits) > 0 {
			fmt.Printf("   └── Found %d commit(s)\n", len(commits))
		}

		allCommits = append(allCommits, commits...)
	}

	var report *Report
	if cfg != nil {
		report = generateReportWithConfig(allCommits, cfg, args.Verbose)
	} else {
		report = generateReport(allCommits)
	}

	fmt.Println(report.Summary)
	return nil
}

// parseDate parses a YYYY-MM-DD date in the local timezone.
func parseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (expected YYYY-MM-DD)", s)
	}
	return t, nil
}

// localDay truncates a time to its calendar day in the local timezone.
func localDay(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}
