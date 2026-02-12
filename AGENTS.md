# PROJECT KNOWLEDGE BASE

**Generated:** 2026-02-12
**Commit:** 362ee74
**Branch:** main

## OVERVIEW
Rust CLI tool that generates formatted daily reports from git commits with project grouping via YAML config.

## STRUCTURE
```
git-bluff/
├── Cargo.toml          # Dependencies: git2, clap, chrono, serde
├── src/
│   ├── main.rs         # Entry point + workflow orchestration
│   ├── args.rs         # CLI args (clap derive)
│   ├── commit.rs       # Git commit extraction + filtering
│   ├── config.rs       # YAML project config parsing
│   ├── git_repo.rs     # Git repository discovery (ignore crate)
│   └── report.rs       # Output formatting + message cleaning
├── .github/workflows/  # Release builds (Linux + Windows)
└── .cargo/config.toml # Windows linking flags
```

## WHERE TO LOOK
| Task | Location |
|------|----------|
| Add CLI arg | `src/args.rs` (clap derive) |
| Modify date filtering | `src/commit.rs` (lines 56-75) |
| Change output format | `src/report.rs` (`format_text_summary*` functions) |
| Add config fields | `src/config.rs` (serde Deserialize) |
| Git repository detection | `src/git_repo.rs` (WalkBuilder) |

## CODE MAP
| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `Args` | struct | args.rs:5 | CLI argument parsing |
| `CommitInfo` | struct | commit.rs:6 | Commit data container |
| `Config` | struct | config.rs:18 | Project grouping config |
| `get_commits()` | fn | commit.rs:16 | Fetch commits from repo |
| `find_git_repositories()` | fn | git_repo.rs:5 | Discover .git directories |
| `generate_report()` | fn | report.rs:10 | Format commit output |

## CONVENTIONS
- **Error handling**: `anyhow::Result` + `anyhow::bail!`
- **Module pattern**: Flat `src/*.rs` (no `mod.rs`)
- **CLI derive**: `clap` Parser derive
- **Config format**: YAML with serde_yaml
- **Date handling**: `chrono::NaiveDate`

## ANTI-PATTERNS (THIS PROJECT)
- **Mixed Chinese/English comments**: commit.rs contains Chinese comments (lines 23, 36-37, 48, 81) - prefer English
- **Empty `src/modules/` directory**: Unused, should be removed
- **No tests**: `assert_cmd` and `predicates` in dev-dependencies but unused
- **Chinese error messages**: `commit.rs:23` uses Chinese - stick to English

## UNIQUE STYLES
- **Commit message cleaning**: Removes `git-svn-id:` and conventional prefixes (feat:, fix:, etc.)
- **Config path matching**: Uses `contains()` for flexible repo path matching
- **Depth-based scanning**: `ignore::WalkBuilder` with configurable max_depth

## COMMANDS
```bash
cargo build --release    # Production build
cargo run -- --help     # Test CLI
cargo test              # No tests yet (infra exists)
```

## GOTCHAS
- `--from` requires `--to` (cannot use alone)
- `--date` mutually exclusive with `--from`/`--to`
- `--depth 0` scans only starting directory
- `OPENSSL_NO_VENDOR=1` required for cross-compilation
- Release CI only triggers on `v*` tags (no PR checks)
