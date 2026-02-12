use crate::commit::CommitInfo;
use crate::config::Config;
use std::fmt::Write;

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

fn clean_commit_line(line: &str) -> Option<String> {
    let line = line.trim();
    if line.is_empty() {
        return None;
    }

    // 1. 移除 git-svn-id 及其之后的所有内容
    let base_content = if let Some((before, _)) = line.split_once("git-svn-id:") {
        before.trim()
    } else {
        line
    };

    if base_content.is_empty() {
        return None;
    }

    // 2. 移除 conventional commits 前缀
    // ref: https://www.conventionalcommits.org/en/v1.0.0/
    // @commitlint/config-conventional (Angular convention) 推荐:
    // feat, fix, build, chore, ci, docs, perf, refactor, style, test, revert
    let stages = [
        "feat", "fix", "build", "chore", "ci", "docs", "perf", "refactor", "style", "test",
        "revert",
    ];

    let final_content = if let Some((prefix, content)) = base_content.split_once(':') {
        if stages.contains(&prefix.to_lowercase().trim()) {
            content.trim()
        } else {
            base_content
        }
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
        let cleaned_lines: Vec<String> = commit
            .message
            .lines()
            .filter_map(clean_commit_line) // 自动移除空行并执行清理逻辑
            .collect();

        if !cleaned_lines.is_empty() {
            output.push_str(&cleaned_lines.join("\n")); // 后续行缩进
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
            let cleaned_lines: Vec<String> = commit
                .message
                .lines()
                .filter_map(clean_commit_line)
                .collect();

            for line in cleaned_lines {
                line_num += 1;
                output.push_str(&format!("{}. {}\n", line_num, line));
            }
        }
    }

    output
}
