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

    if args.verbose {
        println!("📂 Scanning directory: {}", args.directory.display());
        if args.recursive {
            println!("🔄 Recursive mode enabled");
        }
        if let Some(date) = args.date {
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

    let repos = git_repo::find_git_repositories(&args.directory, args.recursive)?;

    if args.verbose {
        println!("✅ Found {} git repository(ies)\n", repos.len());
    }

    let mut all_commits = Vec::new();

    for repo_path in repos {
        if args.verbose {
            println!("📦 Processing: {}", repo_path.display());
        }

        let target_date = args
            .date
            .unwrap_or_else(|| chrono::Local::now().date_naive());

        let commits = commit::get_commits(&repo_path, Some(target_date), args.author.as_deref())?;

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
