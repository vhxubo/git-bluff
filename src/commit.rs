use anyhow::{Context, Result};
use chrono::{DateTime, NaiveDate, TimeZone, Utc};
use git2::{Commit, Repository, Sort};
use std::path::Path;

#[allow(dead_code)]
pub struct CommitInfo {
    pub id: String,
    pub author_name: String,
    pub author_email: String,
    pub message: String,
    pub timestamp: DateTime<Utc>,
    pub repository: String,
    pub path: String,
}

pub fn get_commits(
    repo_path: &Path,
    start_date: Option<NaiveDate>,
    end_date: Option<NaiveDate>,
    authors: &[String],
) -> Result<Vec<CommitInfo>> {
    let full_path = std::fs::canonicalize(repo_path)
        .with_context(|| format!("Failed to get full path: {}", repo_path.display()))?;
    let repo = Repository::open(repo_path)
        .with_context(|| format!("Failed to open repository: {}", repo_path.display()))?;

    let mut revwalk = repo
        .revwalk()
        .with_context(|| "Failed to create revision walker")?;

    revwalk.set_sorting(Sort::TIME | Sort::REVERSE)?;

    revwalk
        .push_head()
        .with_context(|| format!("Failed to push HEAD for repo at: {}", repo_path.display()))?;

    let mut commits = Vec::new();
    let repo_name = repo_path
        .file_name()
        .unwrap_or_default()
        .to_string_lossy()
        .into_owned();

    for oid_result in revwalk {
        let oid = oid_result.with_context(|| "Failed to get commit OID")?;

        let commit = repo
            .find_commit(oid)
            .with_context(|| format!("Failed to find commit: {}", oid))?;

        let parents = commit.parent_count();
        if parents > 1 {
            continue;
        }

        if start_date.is_some() || end_date.is_some() {
            let commit_date = commit.time().seconds();
            let naive_date = DateTime::<Utc>::from_timestamp(commit_date, 0)
                .map(|dt| dt.date_naive())
                .unwrap_or_else(|| {
                    let dt = Utc.timestamp_opt(commit_date, 0).single();
                    dt.map(|d| d.date_naive()).unwrap_or_default()
                });

            if let Some(start) = start_date {
                if naive_date < start {
                    continue;
                }
            }
            if let Some(end) = end_date {
                if naive_date > end {
                    continue;
                }
            }
        }

        // Filter by authors if any provided
        if !authors.is_empty() {
            let commit_author = commit.author().name().unwrap_or("").to_lowercase();
            let matches = authors
                .iter()
                .any(|filter_author| commit_author.contains(&filter_author.to_lowercase()));
            if !matches {
                continue;
            }
        }

        let commit_info = parse_commit(&commit, &repo_name, &full_path)?;
        commits.push(commit_info);
    }

    Ok(commits)
}

fn parse_commit<'a>(commit: &Commit<'a>, repo_name: &str, path: &Path) -> Result<CommitInfo> {
    let message = commit.message().unwrap_or("").to_string();

    Ok(CommitInfo {
        id: commit.id().to_string(),
        path: path.display().to_string(),
        author_name: commit.author().name().unwrap_or("Unknown").to_string(),
        author_email: commit.author().email().unwrap_or("Unknown").to_string(),
        message,
        timestamp: DateTime::<Utc>::from_timestamp(commit.time().seconds(), 0).unwrap_or_else(
            || {
                Utc.timestamp_opt(commit.time().seconds(), 0)
                    .single()
                    .unwrap_or_default()
            },
        ),
        repository: repo_name.to_string(),
    })
}
