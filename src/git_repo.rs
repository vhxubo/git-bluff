use anyhow::Result;
use ignore::WalkBuilder;
use std::path::{Path, PathBuf};

pub fn find_git_repositories(directory: &Path, depth: usize) -> Result<Vec<PathBuf>> {
    let mut repos = Vec::new();

    if depth == 0 {
        if is_git_repository(directory) {
            repos.push(directory.to_path_buf());
        }
        return Ok(repos);
    }

    let mut walker = WalkBuilder::new(directory)
        .max_depth(Some(depth + 1)) // +1 because we start at the directory itself
        .follow_links(false)
        .require_git(false) // Don't require .git to walk
        .build();

    for result in walker {
        let entry = result?;
        let path = entry.path();

        // Check if it's a directory containing .git
        if entry.file_type().map(|ft| ft.is_dir()).unwrap_or(false) {
            if is_git_repository(path) {
                repos.push(path.to_path_buf());
            }
        }
    }

    repos.sort();
    repos.dedup();

    Ok(repos)
}

fn is_git_repository(path: &Path) -> bool {
    let git_dir = path.join(".git");
    git_dir.is_dir() && git_dir.join("HEAD").exists()
}
