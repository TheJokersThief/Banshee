![](images/banshee.png)

![](https://badgers.space/github/release/TheJokersThief/Banshee?scale=1.25)
![](https://badgers.space/github/license/TheJokersThief/Banshee?scale=1.25)
# Banshee

A CLI for managing large-scale code changes across a GitHub organisation, and 
the lifecycle of Pull Requests created.

## Why

As any company grows, you accumulate small pieces of tech debt, particularly for
products that use software built by infrastructure and platform teams.

For 50 engineers, it's acceptable to ask for a 10-line change. At 500 engineers,
it's a lot less reasonable. 

Hopefully this CLI helps manage those code changes and the pull request lifecycle
required to get them applied.

## Highlighted Features

* 🤖 **Github App creds** for higher ratelimits and impersonal attribution.
* 🚚 **Progress saving** which lets you stop, start and resume migrations any time, or in the event of an error.
* 🍞 **Batching** so you can space out the work over multiple days/weeks.
* ✅ **Assign reviewers** as a default if codeowners don't get assigned, never leave a PR unreviewed again
* 📋 View a **list of PRs** related to the migration, in any state

# Installation

## Supported Architectures

- `darwin-arm64` (macOS with Apple Silicon)
- `darwin-amd64` (macOS with Intel)
- `linux-amd64` (Linux x86_64)

## Install from Release

```bash
export ARCH="darwin-arm64"  # Change to your architecture
curl -o banshee -L "https://github.com/TheJokersThief/Banshee/releases/latest/download/banshee-${ARCH}" \
  && chmod +x banshee \
  && sudo mv banshee /usr/local/bin/
```

Verify installation:

```bash
banshee version
```


# Quickstart

## 1. Create Configuration Files

Create your global configuration (`config.yaml`):

```yaml
github:
  use_github_app: false
  token: "gha_YOUR_PERSONAL_ACCESS_TOKEN"

defaults:
  git_email: "your-email@example.com"
  git_name: "Your Name"
  organisation: "your-org"
  code_reviewer: "team-slug"

options:
  log_level: info
  save_progress:
    enabled: true
    directory: ".banshee"
```

Create your migration configuration (`migration.yaml`):

```yaml
search_query: "file:old_config.json"
organisation: "your-org"
branch_name: "chore/update-config"

actions:
  - action: replace
    description: "Update old config references"
    input:
      glob: "**/*.json"
      old: "old_config"
      new: "new_config"

pr_title: "Update config references"
pr_body_file: "pr_body.md"
```

## 2. Clone Repositories

Before running migrations, clone the target repositories:

```bash
banshee clone migration.yaml --config config.yaml
```

## 3. Run Migration

Execute the migration across all matching repositories:

```bash
banshee migrate migration.yaml --config config.yaml
```

## 4. Review and Merge

List all PRs created by the migration:

```bash
banshee list migration.yaml --config config.yaml
```

Automatically merge PRs that pass all branch protections:

```bash
banshee merge migration.yaml --config config.yaml
```

# Commands

## Global Flags

- `-c, --config` Path to global CLI config file (default: `./config.yaml`)
- `-h, --help` Show help message

## Available Commands

- `version` - Print banshee CLI version
- `clone <path>` - Clone all repositories involved in the migration
- `migrate <path>` - Run the migration actions across repositories
  - `-j, --concurrency <n>` - Number of repos to process in parallel (requires `cache_repos.enabled: true`)
- `list <path>` - List PRs associated with a migration
- `merge <path>` - Merge PRs that pass all branch protections

For detailed help on any command, run `banshee <command> --help`

## Examples

### Pre-cloning repos for migration

```bash
banshee clone examples/migration_config/migration.yaml \
    --config examples/global_config/config.yaml \
```

### Running a migration

```bash
banshee migrate examples/migration_config/migration.yaml \
    --config examples/global_config/config.yaml
```

### Listing all PRs for a migration

```bash
banshee list examples/migration_config/migration.yaml \
    --config examples/global_config/config.yaml \
    --state all \
    --format json
```

### Merging any PRs not blocked by branch protections

This assumes that you block merge to mainline branches with branch protections like 
"requires a PR", "required approvers: 1", "required status checks" and that they're 
not handled on the honour system.

This just checks the GitHub "mergeable state" is "clean" (terms determined by GitHub)  
to see if the PR can be merged.

```bash
banshee merge examples/migration_config/migration.yaml \
    --config examples/global_config/config.yaml
```

# Configuration

Banshee requires two configuration files:

## Global Configuration

The global config file (`config.yaml`) contains:
- GitHub authentication (Personal Access Token or GitHub App credentials)
- Default values (git email, name, organization, code reviewer)
- Options (logging level, caching, progress saving, merge strategy)

See [docs/global_config.md](docs/global_config.md) for complete documentation and [examples/global_config/config.yaml](examples/global_config/config.yaml) for a full example.

## Migration Configuration

Each migration has its own config file that defines:
- Repository selection (by search query, list, or all repos in org)
- Actions to perform (find/replace, run commands, YAML modifications, add files)
- PR details (title, body, draft status)

See [docs/migrations.md](docs/migrations.md) for action documentation and [examples/migration_config/migration.yaml](examples/migration_config/migration.yaml) for a complete example.

### Real-World Examples

- [Bash script and file template](examples/001_bash-script-add-file-template/) - Run a bash script and add files
- [CODEOWNERS addition](examples/002_add-codeowners/) - Add CODEOWNERS file to repos
- [JSON package version bump](examples/003_json-package-version/) - Update package.json version, add a field, and remove a deprecated field

# Documentation

For more information, see [docs/](docs/):
- [Global Configuration Reference](docs/global_config.md)
- [Migration Actions Reference](docs/migrations.md)
