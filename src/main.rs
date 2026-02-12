mod args;
mod commit;
mod config;
mod git_repo;
mod report;

use anyhow::Result;
use args::Args;
use clap::Parser;
use config::Config;

fn main() -> Result<()> {
    let args = Args::parse();

    // Validate date arguments
    let from_empty = args.from.is_empty();
    let to_empty = args.to.is_empty();

    if (args.date.is_some()) && (!from_empty || !to_empty) {
        anyhow::bail!("--date cannot be used with --from/--to");
    }

    if !to_empty && from_empty {
        anyhow::bail!("--to must be used with --from");
    }

    if args.verbose {
        println!("📂 Scanning directory: {}", args.directory.display());
        if args.depth > 0 {
            println!("🔄 Max depth: {}", args.depth);
        }
        if !from_empty && !to_empty {
            println!("📅 Date range: {} to {}", args.from, args.to);
        } else if let Some(date) = args.date {
            println!("📅 Date filter: {}", date);
        }
        if let Some(ref config_path) = args.config {
            println!("📄 Config file: {}", config_path.display());
        }
        println!();
    }

    // Load config if provided
    let config = if let Some(config_path) = &args.config {
        Some(Config::load(config_path)?)
    } else {
        None
    };

    let repos = git_repo::find_git_repositories(&args.directory, args.depth)?;

    if args.verbose {
        println!("✅ Found {} git repository(ies)\n", repos.len());
    }

    let mut all_commits = Vec::new();

    for repo_path in repos {
        if args.verbose {
            println!("📦 Processing: {}", repo_path.display());
        }

        let (start_date, end_date) = if !from_empty && !to_empty {
            let today = chrono::Local::now().date_naive();
            let start = args.from.parse().ok().unwrap_or(today);
            let end = args.to.parse().ok().unwrap_or(today);
            (Some(start), Some(end))
        } else if !from_empty && to_empty {
            // Only --from specified: from to today
            let today = chrono::Local::now().date_naive();
            let start = args.from.parse().ok().unwrap_or(today);
            (Some(start), Some(today))
        } else if let Some(date) = args.date {
            (Some(date), Some(date))
        } else {
            let today = chrono::Local::now().date_naive();
            (Some(today), Some(today))
        };

        let commits = commit::get_commits(&repo_path, start_date, end_date, &args.author)?;

        if args.verbose && !commits.is_empty() {
            println!("   └── Found {} commit(s)", commits.len());
        }

        all_commits.extend(commits);
    }

    // Generate report with or without config
    let report = if let Some(ref cfg) = config {
        report::generate_report_with_config(&all_commits, cfg, args.verbose)?
    } else {
        report::generate_report(&all_commits)?
    };

    println!("{}", report.summary);

    Ok(())
}
