mod args;
mod commit;
mod git_repo;
mod report;

use anyhow::Result;
use args::Args;
use clap::Parser;

fn main() -> Result<()> {
    let args = Args::parse();

    println!("🔍 Git Bluff - Daily Report Generator");
    println!("========================================");

    if args.verbose {
        println!("📂 Scanning directory: {}", args.directory.display());
        if args.recursive {
            println!("🔄 Recursive mode enabled");
        }
        if let Some(date) = args.date {
            println!("📅 Date filter: {}", date);
        }
        println!();
    }

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

    let report = report::generate_report(&all_commits)?;

    println!("\n📊 Daily Report Generated!");
    println!("=========================");
    println!("{}", report.summary);

    Ok(())
}
