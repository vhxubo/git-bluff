use crate::commit::CommitInfo;
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

    // 2. 移除常用的 stage 前缀 (feat:, fix: 等)
    let stages = [
        "feat", "fix", "chore", "docs", "refactor", "test", "style", "perf",
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

    output.push_str("═══════════════════════════════════════════════════════════════\n");
    output.push_str("                    DAILY GIT REPORT\n");
    output.push_str("═══════════════════════════════════════════════════════════════\n\n");

    for commit in commits {
        write!(output, "\n📍 Repository Path: {}\n", commit.path).unwrap();
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
