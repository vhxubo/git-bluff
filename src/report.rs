use crate::commit::CommitInfo;
use crate::config::Config;
use regex::Regex;
use std::fmt::Write;
use std::sync::LazyLock;

/// Regex for conventional commit format:
/// `type(scope)!: description`
///
/// Captures: type, optional scope, optional breaking indicator, description
static CONVENTIONAL_RE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"^(?i)(?<type>[a-z]+)(?:\((?<scope>[^)]*\))?)?(?<breaking>!)?:\s*(?<desc>.+)$")
        .unwrap()
});

#[derive(Debug)]
pub struct Report {
    pub summary: String,
}

pub fn generate_report(commits: &[CommitInfo]) -> Result<Report, std::io::Error> {
    let summary = format_text_summary(commits);

    let report = Report { summary };

    Ok(report)
}

pub fn generate_report_with_config(
    commits: &[CommitInfo],
    config: &Config,
    verbose: bool,
) -> Result<Report, std::io::Error> {
    let summary = format_text_summary_with_config(commits, config, verbose);

    let report = Report { summary };

    Ok(report)
}

/// Extract the subject line from a commit message (first non-empty line).
fn commit_subject(message: &str) -> &str {
    message
        .lines()
        .find(|line| !line.trim().is_empty())
        .unwrap_or("")
}

/// Clean a conventional commit subject line, stripping type/scope prefix and git-svn-id.
///
/// Supports full conventional commit format:
/// - `feat: description`
/// - `feat(scope): description`
/// - `feat!: description` (breaking)
/// - `feat(scope)!: description` (breaking with scope)
fn clean_commit_line(line: &str) -> Option<String> {
    let line = line.trim();
    if line.is_empty() {
        return None;
    }

    // 1. Remove git-svn-id and everything after it
    let base_content = if let Some((before, _)) = line.split_once("git-svn-id:") {
        before.trim()
    } else {
        line
    };

    if base_content.is_empty() {
        return None;
    }

    // 2. Strip conventional commit prefix via regex
    let final_content = if let Some(caps) = CONVENTIONAL_RE.captures(base_content) {
        caps.name("desc").map(|m| m.as_str()).unwrap_or(base_content)
    } else {
        base_content
    };

    if final_content.is_empty() {
        None
    } else {
        Some(final_content.to_string())
    }
}

fn format_text_summary(commits: &[CommitInfo]) -> String {
    let mut output = String::new();
    let mut _path = "";

    for commit in commits {
        if commit.path != _path {
            write!(output, "\nRepository Path: {}\n", commit.path).unwrap();
        }
        _path = &commit.path;

        // Use only the subject line (first line) to avoid body noise
        let subject = commit_subject(&commit.message);
        if let Some(cleaned) = clean_commit_line(subject) {
            output.push_str(&cleaned);
            output.push('\n');
        }
    }

    output
}

fn format_text_summary_with_config(
    commits: &[CommitInfo],
    config: &Config,
    verbose: bool,
) -> String {
    let mut output = String::new();

    // Group commits by (project_code, project_name, alias)
    let mut grouped: std::collections::BTreeMap<(String, String, String), Vec<&CommitInfo>> =
        std::collections::BTreeMap::new();

    for commit in commits {
        if let Some((project_code, project_name, alias)) = config.find_matching_repo(&commit.path) {
            grouped
                .entry((project_code, project_name, alias))
                .or_insert_with(Vec::new)
                .push(commit);
        } else {
            // Unmatched repos grouped under unknown
            grouped
                .entry((
                    "UNKNOWN".to_string(),
                    "Unknown".to_string(),
                    commit.path.clone(),
                ))
                .or_insert_with(Vec::new)
                .push(commit);
        }
    }

    // Output grouped by project_code
    let mut current_project = String::new();
    for ((project_code, project_name, alias), commit_list) in grouped {
        if project_code != current_project {
            // Add separator between different projects
            if !current_project.is_empty() {
                output.push_str(
                    "=======================================================================\n",
                );
            }
            write!(output, "\n{} {}\n\n", project_name, project_code).unwrap();
            current_project = project_code.clone();
        } else {
            // Same project, different repo - add newline before repo
            output.push_str("\n");
        }

        if verbose {
            let path_str = commit_list
                .first()
                .map(|c| format!(" ({})", c.path.as_str()))
                .unwrap_or_default();
            write!(output, "Repository: {}{}\n", alias, path_str).unwrap();
        } else {
            write!(output, "{}\n", alias).unwrap();
        }

        let mut line_num = 0;
        for commit in commit_list {
            // Use only the subject line (first line) to avoid body noise
            let subject = commit_subject(&commit.message);
            if let Some(cleaned) = clean_commit_line(subject) {
                line_num += 1;
                output.push_str(&format!("{}. {}\n", line_num, cleaned));
            }
        }
    }

    output
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_clean_plain_message() {
        assert_eq!(clean_commit_line("fix the bug"), Some("fix the bug".into()));
    }

    #[test]
    fn test_clean_conventional_simple() {
        assert_eq!(clean_commit_line("feat: add login"), Some("add login".into()));
    }

    #[test]
    fn test_clean_conventional_with_scope() {
        assert_eq!(
            clean_commit_line("feat(parser): add support"),
            Some("add support".into())
        );
    }

    #[test]
    fn test_clean_conventional_breaking() {
        assert_eq!(
            clean_commit_line("feat!: breaking change"),
            Some("breaking change".into())
        );
    }

    #[test]
    fn test_clean_conventional_scope_breaking() {
        assert_eq!(
            clean_commit_line("feat(api)!: breaking change"),
            Some("breaking change".into())
        );
    }

    #[test]
    fn test_clean_case_insensitive() {
        assert_eq!(
            clean_commit_line("Feat: add login"),
            Some("add login".into())
        );
        assert_eq!(
            clean_commit_line("FIX: bug"),
            Some("bug".into())
        );
    }

    #[test]
    fn test_clean_git_svn_id() {
        assert_eq!(
            clean_commit_line("fix bug\ngit-svn-id: svn://xxx"),
            Some("fix bug".into())
        );
    }

    #[test]
    fn test_clean_empty_line() {
        assert_eq!(clean_commit_line(""), None);
        assert_eq!(clean_commit_line("   "), None);
    }

    #[test]
    fn test_commit_subject_single_line() {
        assert_eq!(commit_subject("feat: hello world"), "feat: hello world");
    }

    #[test]
    fn test_commit_subject_multi_line() {
        let msg = "feat: hello\n\nsome body text\n- bullet";
        assert_eq!(commit_subject(msg), "feat: hello");
    }

    #[test]
    fn test_commit_subject_empty_first_line() {
        let msg = "\nfeat: hello\nbody";
        assert_eq!(commit_subject(msg), "feat: hello");
    }
}
