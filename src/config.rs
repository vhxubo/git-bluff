use serde::Deserialize;
use std::path::PathBuf;

#[derive(Debug, Deserialize, Clone)]
pub struct Repository {
    pub alias: String,
    pub repo_path: String,
}

#[derive(Debug, Deserialize, Clone)]
pub struct Project {
    pub project_name: String,
    pub project_code: String,
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
    pub fn find_matching_repo(&self, commit_path: &str) -> Option<(String, String, String)> {
        for project in &self.projects {
            for repo in &project.repositories {
                // Check if commit_path starts with or contains the repo_path pattern
                if commit_path.contains(&repo.repo_path)
                    || repo.repo_path.contains(commit_path.trim_end_matches('/'))
                {
                    return Some((
                        project.project_code.clone(),
                        project.project_name.clone(),
                        repo.alias.clone(),
                    ));
                }
            }
        }
        None
    }
}
