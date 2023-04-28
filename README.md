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
Usage: main <command>

Flags:
  -h, --help                      Show context-sensitive help.
  -c, --config="./config.yaml"    Path to global CLI config

Commands:
  version
    Print banshee CLI version

  migrate <path>
    Run a migration

Run "main <command> --help" for more information on a command.
```

## Examples

### Running a migration

```bash
banshee migrate \
    -c examples/global_config/config.yaml \
    examples/migration_config/migration.yaml
```


# Configuration
Examples of configuration and all available options can be found in the examples
directory. I try to keep the configs well commented to explain all the features.

* [Global config](examples/global_config/config.yaml): The configuration for the 
CLI as a whole. Things like auth and defaults.
* [Migration config](examples/migration_config/migration.yaml): The configuration
for an individual migration. 
