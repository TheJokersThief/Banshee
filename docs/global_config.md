**Table of contents:**

<!-- TOC -->

- [GitHub github](#github-github)
- [Options options](#options-options)
    - [Assigning code reviewers assign_code_reviewer_if_none_assigned](#assigning-code-reviewers-assign_code_reviewer_if_none_assigned)
    - [Show the output from git commands show_git_output](#show-the-output-from-git-commands-show_git_output)
    - [Caching repos cache_repos](#caching-repos-cache_repos)
- [Defaults defaults](#defaults-defaults)

<!-- /TOC -->


# GitHub (`github`)

This software is built entirely on the basis that you're performing these migrations for GitHub organisations/repos. The `github` section of the config defines how you want to connect.

```yaml
github:
  use_github_app: false
  token: "gha-12345656787800"

  app_id: 0
  app_installation_id: 0
  app_private_key_filepath: ""
```

There are two choices:

* [Personal Access Tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token): Suitable for using your personal account, or a service account. These have a rate limit of 5000 req/s, which is good to keep in mind.
* [Github Apps](https://docs.github.com/en/apps/creating-github-apps/setting-up-a-github-app/creating-a-github-app): More suitable for mid-to-large organisations. These have a rate limit of 15000 req/s.

# Options (`options`)

```yaml
options:
  # Assign a team to review code if no reviewers are already assigned by a CODEOWNERS file
  assign_code_reviewer_if_none_assigned: false
  # Show the output from all git actions (e.g. clones, pulls and fetches)
  show_git_output: false
  cache_repos:
    # If enabled, will store all downloaded repos permanently
    enabled: false
    # Location for storing the repos cache
    directory: "repos.cache"
  merging:
    strategy: "merge" # "merge", "squash", "rebase"
    append_title: "[CI SKIP]" # A string to append to the merge commit message
```

## Assigning code reviewers (`assign_code_reviewer_if_none_assigned`)

If there are new reviewers assigned automatically via CODEOWNERS files, this will assign the [**team**](https://docs.github.com/en/organizations/organizing-members-into-teams/creating-a-team) you've chosen as a reviewer.

## Show the output from git commands (`show_git_output`)

Enabling this will show output like you would get from your local `git` CLI.

Example of similar output:

```
[main 58616c8] Okay, I'll just edit the image instead
 2 files changed, 1 insertion(+), 1 deletion(-)
Warning: Permanently added 'github.com' (ED25519) to the list of known hosts.
Enumerating objects: 11, done.
Counting objects: 100% (11/11), done.
Delta compression using up to 6 threads
Compressing objects: 100% (6/6), done.
Writing objects: 100% (6/6), 1.48 MiB | 2.34 MiB/s, done.
Total 6 (delta 2), reused 0 (delta 0), pack-reused 0
remote: Resolving deltas: 100% (2/2), completed with 2 local objects.
To github.com:TheJokersThief/Banshee.git
   9245979..58616c8  main -> main
```

## Caching repos (`cache_repos`)

Cloning every repo each time you want to perform a migration can be costly in network and time. To speed things up, you can choose to clone the repo once into the `directory` you choose. Then, when we need to run any migration in the future, it will first pull any new changes from the repo's default branch before running your actions.

This is particularly appealing if your organisation has several monorepos with long git histories.

# Defaults (`defaults`)

These are defaults used when a value doesn't exist.

```yaml
defaults:
  # The author and commit email
  git_email: "no-reply@example.com"
  # The author and commit name
  git_name: "Example User"
  # GitHub organisation slug
  organisation: "github"
  # Team slug to be added as a reviewer
  code_reviewer: "no-one"
```
