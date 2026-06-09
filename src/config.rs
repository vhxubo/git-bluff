use serde::Deserialize;
use std::path::Path;

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
    pub fn load(path: &Path) -> Result<Self, anyhow::Error> {
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
        for project in &self.projects {
            // Priority 1: explicit repository entries
            for repo in &project.repositories {
                if path_matches(&repo.repo_path, commit_path) {
                    return Some((
                        project.project_code.clone(),
                        project.project_name.clone(),
                        repo.alias.clone(),
                    ));
                }
            }

            // Priority 2: directory prefix match
            if let Some(ref dir) = project.directory {
                let dir_prefix = format!("{}/", dir.trim_end_matches('/'));
                if commit_path.starts_with(&dir_prefix) {
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
/// Otherwise use path-component-aware matching — avoids `/git/ec`
/// falsely matching `/git/ecology` while still matching `/git/ec/frontend`.
fn path_matches(pattern: &str, path: &str) -> bool {
    if has_glob_chars(pattern) {
        glob::Pattern::new(pattern)
            .map(|p| p.matches(path))
            .unwrap_or(false)
    } else {
        path_component_match(pattern, path)
    }
}

/// Component-aware path matching.
///
/// One path must contain the other at a `/` boundary:
/// - `/git/ec/frontend` matches `/git/ec` (pattern is prefix of path)
/// - `/git/ec` does NOT match `/git/ecology` (no component boundary)
/// - `/home/user/repo` matches `/home/user/repo/` (trailing slash ignored)
fn path_component_match(pattern: &str, path: &str) -> bool {
    // Wrap with '/' so containment checks are component-boundary-aware
    let p = format!("/{}/", pattern.trim_matches('/'));
    let q = format!("/{}/", path.trim_matches('/'));
    q.contains(&p) || p.contains(&q)
}

/// Check if a string contains glob metacharacters.
fn has_glob_chars(s: &str) -> bool {
    s.contains('*') || s.contains('?') || s.contains('[')
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_path_matches_exact() {
        assert!(path_matches("/git/ec/frontend", "/git/ec/frontend"));
    }

    #[test]
    fn test_path_matches_subpath() {
        assert!(path_matches("/git/ec/frontend", "/home/user/git/ec/frontend"));
    }

    #[test]
    fn test_path_matches_trailing_slash() {
        assert!(path_matches("/home/user/git/ec/frontend", "/home/user/git/ec/frontend/"));
    }

    #[test]
    fn test_path_no_false_prefix_match() {
        // /git/ec should NOT match /git/ecology
        assert!(!path_matches("/git/ec", "/git/ecology/repo"));
        assert!(!path_matches("/git/ec", "/git/ecology"));
    }

    #[test]
    fn test_path_matches_reverse_contains() {
        // pattern longer than path: pattern starts with path
        assert!(path_matches("/home/user/git/ec/frontend", "/home/user/git/ec"));
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
    fn test_directory_no_false_prefix() {
        let config = Config {
            projects: vec![Project {
                project_name: "Test".into(),
                project_code: "T-1".into(),
                directory: Some("/git/ec".into()),
                repositories: vec![],
            }],
        };

        assert!(config.find_matching_repo("/git/ecology/repo").is_none());
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
