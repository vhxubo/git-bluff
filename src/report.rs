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
    Regex::new(r"^(?i)[a-z]+(?:\([^)]*\))?!?:\s*(?<desc>.+)$").unwrap()
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

/// Extract additional lines from commit body that look like squash commits.
/// Returns lines that are not empty and don't start with '-', '+', or '*'.
fn extract_squash_lines(message: &str) -> Vec<String> {
    let mut lines = Vec::new();
    let mut found_subject = false;

    for line in message.lines() {
        let trimmed = line.trim();
        if !found_subject {
            if !trimmed.is_empty() {
                found_subject = true;
            }
            continue;
        }

        // Skip empty lines
        if trimmed.is_empty() {
            continue;
        }

        // Skip bullet/numbered list lines (start with -, +, *, or number)
        let is_list_item = trimmed.starts_with('-')
            || trimmed.starts_with('+')
            || trimmed.starts_with('*')
            || trimmed.chars().next().map(|c| c.is_numeric()).unwrap_or(false);

        // Skip git-svn-id lines
        if trimmed.starts_with("git-svn-id:") {
            continue;
        }

        if !is_list_item {
            lines.push(trimmed.to_string());
        }
    }

    lines
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
                .or_default()
                .push(commit);
        } else {
            // Unmatched repos grouped under unknown
            grouped
                .entry((
                    "UNKNOWN".to_string(),
                    "Unknown".to_string(),
                    commit.path.clone(),
                ))
                .or_default()
                .push(commit);
        }
    }

    // Output grouped by project_code
    let mut current_project = String::new();
    let mut total_line_num = 0;
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
            // Use the subject line (first line)
            let subject = commit_subject(&commit.message);
            if let Some(cleaned) = clean_commit_line(subject) {
                line_num += 1;
                write!(output, "{}. {}\n", line_num, cleaned).unwrap();
            }

            // Extract squash lines from body (non-list items)
            let squash_lines = extract_squash_lines(&commit.message);
            for squash_line in squash_lines {
                if let Some(cleaned) = clean_commit_line(&squash_line) {
                    line_num += 1;
                    write!(output, "{}. {}\n", line_num, cleaned).unwrap();
                }
            }
        }
        total_line_num += line_num;
    }

    // Add total commit count at the end
    write!(output, "\n=======================================================================\nTotal commits: {}\n", total_line_num).unwrap();

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

    #[test]
    fn test_extract_squash_lines_basic() {
        let msg = "feat: main feature\n\nfeat: add login\nfix: correct typo";
        let lines = extract_squash_lines(msg);
        assert_eq!(lines.len(), 2);
        assert_eq!(lines[0], "feat: add login");
        assert_eq!(lines[1], "fix: correct typo");
    }

    #[test]
    fn test_extract_squash_lines_skip_bullets() {
        let msg = "feat: main\n\n- item one\n- item two\nnormal line";
        let lines = extract_squash_lines(msg);
        assert_eq!(lines.len(), 1);
        assert_eq!(lines[0], "normal line");
    }

    #[test]
    fn test_extract_squash_lines_skip_plus_star() {
        let msg = "feat: main\n\n+ item one\n* item two\nfeat: real commit";
        let lines = extract_squash_lines(msg);
        assert_eq!(lines.len(), 1);
        assert_eq!(lines[0], "feat: real commit");
    }

    #[test]
    fn test_extract_squash_lines_skip_numbered() {
        let msg = "feat: main\n\n1. first item\n2. second item\nfeat: actual commit";
        let lines = extract_squash_lines(msg);
        assert_eq!(lines.len(), 1);
        assert_eq!(lines[0], "feat: actual commit");
    }

    #[test]
    fn test_extract_squash_lines_skip_empty() {
        let msg = "feat: main\n\n\nfeat: second\n\n\nfeat: third";
        let lines = extract_squash_lines(msg);
        assert_eq!(lines.len(), 2);
        assert_eq!(lines[0], "feat: second");
        assert_eq!(lines[1], "feat: third");
    }

    #[test]
    fn test_extract_squash_lines_no_body() {
        let msg = "feat: simple commit";
        let lines = extract_squash_lines(msg);
        assert!(lines.is_empty());
    }

    #[test]
    fn test_extract_squash_lines_all_bullets() {
        let msg = "feat: main\n\n- bullet one\n- bullet two\n+ plus item";
        let lines = extract_squash_lines(msg);
        assert!(lines.is_empty());
    }

    #[test]
    fn test_squash_chinese_commits() {
        let msg = "feat: 路检路查检测项弹窗展示\n\nfeat: 支持解析JSON格式检测数据\n\ngit-svn-id: https://192.168.4.15/svn/03_project/HB-SQ-25-106%20%E6%B2%B3%E5%8D%97%E7%9C%81%E7%94%9F%E6%80%81%E7%8E%AF%E5%A2%83%E5%8E%85%E7%A7%BB%E5%8A%A8%E6%BA%90OBD%E7%9B%91%E7%AE%A1%E5%B9%B3%E5%8F%B0%E9%A1%B9%E7%9B%AE/%E5%89%8D%E7%AB%AF%E4%BB%A3%E7%A0%81/vehicle-ui@151857 1f296117-f2e1-c044-9fbd-e3fffb978f54";

        let subject = commit_subject(msg);
        assert_eq!(subject, "feat: 路检路查检测项弹窗展示");
        assert_eq!(clean_commit_line(subject), Some("路检路查检测项弹窗展示".into()));

        let squash_lines = extract_squash_lines(msg);
        assert_eq!(squash_lines.len(), 1);
        assert_eq!(squash_lines[0], "feat: 支持解析JSON格式检测数据");
        assert_eq!(clean_commit_line(&squash_lines[0]), Some("支持解析JSON格式检测数据".into()));
    }

    #[test]
    fn test_extract_squash_lines_skip_git_svn_id() {
        let msg = "feat: main\n\nfeat: real commit\ngit-svn-id: svn://xxx\nanother: commit";
        let lines = extract_squash_lines(msg);
        assert_eq!(lines.len(), 2);
        assert_eq!(lines[0], "feat: real commit");
        assert_eq!(lines[1], "another: commit");
    }
}
