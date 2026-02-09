use anyhow::Result;
use std::path::{Path, PathBuf};
use walkdir::WalkDir;

pub fn find_git_repositories(directory: &Path, recursive: bool) -> Result<Vec<PathBuf>> {
    let mut repos = Vec::new();
    if !recursive {
        if is_git_repository(directory) {
            repos.push(directory.to_path_buf());
        }
    } else {
        for entry in WalkDir::new(directory).max_depth(4).follow_links(false) {
            let entry = entry?;
            let path = entry.path();

            if path.is_dir() && is_git_repository(path) {
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
