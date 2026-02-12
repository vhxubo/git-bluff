# git-bluff

Generate formatted daily reports from git commits with project grouping support.

## Features

- Collect commits from single or multiple git repositories
- Filter by date and author
- Group commits by project using YAML configuration
- Clean commit messages (remove git-svn-id, conventional commit prefixes)
- Format output with numbered commit lines

## Usage

```bash
git-bluff [OPTIONS]
```

### Options

| Flag | Description |
|------|-------------|
| `-d, --directory <PATH>` | Starting directory to scan (default: `.`) |
| `-r, --recursive` | Recursively scan for git repositories |
| `--date <DATE>` | Filter commits by single date (format: YYYY-MM-DD) |
| `--from <DATE>` | Start date for date range (format: YYYY-MM-DD) |
| `--to <DATE>` | End date for date range (format: YYYY-MM-DD) |
| `--author <NAME>` | Filter commits by author name |
| `--config <PATH>` | Path to YAML configuration file |
| `-v, --verbose` | Show repository paths in output |
| `--help` | Show help information |

**Date Filtering Rules:**
- `--date` filters commits for a single day
- `--from` and `--to` must be used together (filters commits within the date range)
- If only `--from` is specified, end date defaults to today
- `--date` cannot be used with `--from`/`--to` |

## Examples

### Basic Usage

```bash
# Scan current directory for today's commits
git-bluff

# Scan specific directory
git-bluff --directory /path/to/repos

# Scan recursively
git-bluff --directory /path/to/repos --recursive

# Filter by single date
git-bluff --date 2026-01-15

# Filter by date range (from to today)
git-bluff --from 2026-01-01

# Filter by date range (specific start and end)
git-bluff --from 2026-01-01 --to 2026-01-31

# Filter by author
git-bluff --author "John"

# Combine filters
git-bluff --from 2026-01-01 --author "John" --verbose
```

### With Configuration

```bash
git-bluff --config projects.yaml --directory /git
```

## Configuration File

Create a `projects.yaml` file to group repositories by project:

```yaml
projects:
  - project_name: "E-Commerce Platform"
    project_code: "EC-2026"
    repositories:
      - alias: "web_ui"
        repo_path: "/git/ec/frontend"
      - alias: "order_svc"
        repo_path: "/git/ec/order"

  - project_name: "Logistics System"
    project_code: "LOG-2026"
    repositories:
      - alias: "driver_app"
        repo_path: "/git/log/mobile"
      - alias: "wms_system"
        repo_path: "/git/log/wms"
```

### Configuration Fields

| Field | Description |
|-------|-------------|
| `project_name` | Display name for the project |
| `project_code` | Unique code for the project |
| `repositories` | List of repositories in this project |
| `alias` | Display name for the repository |
| `repo_path` | Path pattern to match repository (can be partial or full path) |

## Output Format

### Without Config (Basic)

```
Repository Path: /home/user/repo
1. Add login feature
2. Fix CSS bug
```

### With Config (Normal Mode)

```
=======================================================================
Project AlphaJ-001 PR

web_ui
1. Add login feature
2. Fix CSS bug

order_svc
1. Create order endpoint
2. Add order validation

=======================================================================
Project Beta PRJ-002

driver_app
1. Update driver profile
2. Fix navigation bug

=======================================================================
```

### With Config (Verbose Mode)

```
=======================================================================
Project Alpha PRJ-001

Repository: web_ui (/home/user/repos/web_ui)
1. Add login feature
2. Fix CSS bug

Repository: order_svc (/home/user/repos/order_svc)
1. Create order endpoint
2. Add order validation

=======================================================================
```

## Message Cleaning

The tool automatically cleans commit messages:

1. Removes `git-svn-id:` and everything after it
2. Removes conventional commit prefixes:
   - `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`, `style:`, `perf:`

## Building

```bash
cargo build --release
```

## License

MIT
