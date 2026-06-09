use serde::Deserialize;
use std::path::{Path, PathBuf};

#[derive(Debug, Deserialize, Clone)]
pub struct Repository {
    pub alias: String,
    pub repo_path: String,
}

#[derive(Debug, Deserialize, Clone)]
pub struct Project {
    pub project_name: String,
    pub project_code: String,
    /// Optional: match all git repos under this directory prefix
    pub directory: Option<String>,
    #[serde(default)]
    pub repositories: Vec<Repository>,
}

#[derive(Debug, Deserialize, Clone)]
pub struct Config {
    pub projects: Vec<Project>,
}

impl Config {
    pub fn load(path: &PathBuf) -> Result<Self, anyhow::Error> {
        let content = std::fs::read_to_string(path)?;
        let config = serde_yaml::from_str(&content)?;
        Ok(config)
    }

    /// Find matching repository and return (project_code, project_name, alias)
    ///
    /// Matching priority:
    /// 1. `repositories` entries (exact path substring or glob pattern)
    /// 2. `directory` prefix match (alias = repo folder name)
    pub fn find_matching_repo(&self, commit_path: &str) -> Option<(String, String, String)> {
        // Phase 1: repositories matching (higher priority)
        for project in &self.projects {
            for repo in &project.repositories {
                if path_matches(&repo.repo_path, commit_path) {
                    return Some((
                        project.project_code.clone(),
                        project.project_name.clone(),
                        repo.alias.clone(),
                    ));
                }
            }
        }

        // Phase 2: directory prefix matching
        for project in &self.projects {
            if let Some(ref dir) = project.directory {
                let dir_norm = dir.trim_end_matches('/');
                if commit_path.starts_with(dir_norm) {
                    let alias = Path::new(commit_path)
                        .file_name()
                        .map(|n| n.to_string_lossy().to_string())
                        .unwrap_or_else(|| commit_path.to_string());
                    return Some((
                        project.project_code.clone(),
                        project.project_name.clone(),
                        alias,
                    ));
                }
            }
        }

        None
    }
}

/// Match a configured path pattern against an actual commit path.
///
/// If the pattern contains glob wildcards (`*`, `?`, `[`), use glob matching.
/// Otherwise fall back to the original substring containment logic.
fn path_matches(pattern: &str, path: &str) -> bool {
    if pattern.contains('*') || pattern.contains('?') || pattern.contains('[') {
        glob::Pattern::new(pattern)
            .map(|p| p.matches(path))
            .unwrap_or(false)
    } else {
        path.contains(pattern) || pattern.contains(path.trim_end_matches('/'))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_path_matches_exact_substring() {
        assert!(path_matches("/git/ec/frontend", "/home/user/git/ec/frontend"));
        assert!(path_matches("/git/ec/frontend", "/git/ec/frontend"));
    }

    #[test]
    fn test_path_matches_reverse() {
        // Original behavior: pattern contains the (trimmed) path
        assert!(path_matches("/home/user/git/ec/frontend", "/home/user/git/ec/frontend/"));
    }

    #[test]
    fn test_path_matches_glob_star() {
        assert!(path_matches("/git/ec/*/order-svc", "/git/ec/sub/order-svc"));
        assert!(!path_matches("/git/ec/*/order-svc", "/git/ec/sub/frontend"));
    }

    #[test]
    fn test_path_matches_glob_question() {
        assert!(path_matches("/git/ec/frontend-?", "/git/ec/frontend-1"));
        assert!(!path_matches("/git/ec/frontend-?", "/git/ec/frontend-12"));
    }

    #[test]
    fn test_directory_prefix_match() {
        let config = Config {
            projects: vec![Project {
                project_name: "Test".into(),
                project_code: "T-1".into(),
                directory: Some("/git/ec".into()),
                repositories: vec![],
            }],
        };

        let result = config.find_matching_repo("/git/ec/frontend");
        assert!(result.is_some());
        let (code, name, alias) = result.unwrap();
        assert_eq!(code, "T-1");
        assert_eq!(name, "Test");
        assert_eq!(alias, "frontend");
    }

    #[test]
    fn test_repos_higher_priority_than_directory() {
        let config = Config {
            projects: vec![Project {
                project_name: "Test".into(),
                project_code: "T-1".into(),
                directory: Some("/git/ec".into()),
                repositories: vec![Repository {
                    alias: "custom_alias".into(),
                    repo_path: "/git/ec/frontend".into(),
                }],
            }],
        };

        let result = config.find_matching_repo("/git/ec/frontend");
        assert!(result.is_some());
        let (_, _, alias) = result.unwrap();
        assert_eq!(alias, "custom_alias");
    }

    #[test]
    fn test_glob_repo_path() {
        let config = Config {
            projects: vec![Project {
                project_name: "Test".into(),
                project_code: "T-1".into(),
                directory: None,
                repositories: vec![Repository {
                    alias: "order".into(),
                    repo_path: "/git/*/order-svc".into(),
                }],
            }],
        };

        let result = config.find_matching_repo("/git/ec/order-svc");
        assert!(result.is_some());
        assert_eq!(result.unwrap().2, "order");

        assert!(config.find_matching_repo("/git/ec/frontend").is_none());
    }
}
