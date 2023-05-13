![](images/banshee.png)

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

# Install


# Usage

```bash
Usage: banshee <command>

Flags:
  -h, --help                      Show context-sensitive help.
  -c, --config="./config.yaml"    Path to global CLI config

Commands:
  version
    Print banshee CLI version

  migrate <path>
    Run a migration

  list <path>
    List PRs associated with a migration
  
  merge <path>
    Merge PRs not blocked by any branch protections

  clone <path>
    Clone all of the repositories that are going to be involved in a migration

Run "main <command> --help" for more information on a command.
```

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

This just checks the GitHub [MergeableState](https://docs.github.com/en/graphql/reference/enums#mergeablestate) 
to see if the PR can be merged.

```bash
banshee merge examples/migration_config/migration.yaml \
    --config examples/global_config/config.yaml
```

# Configuration
Examples of configuration and all available options can be found in the examples
directory. I try to keep the configs well commented to explain all the features.

* [Global config](examples/global_config/config.yaml): The configuration for the 
CLI as a whole. Things like auth and defaults.
* [Migration config](examples/migration_config/migration.yaml): The configuration
for an individual migration. 

# Documentation

Have a look in [docs/](docs/) for further information.
