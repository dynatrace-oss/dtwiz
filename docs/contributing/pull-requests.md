# Pull Requests

## Table of Contents

- [Prerequisites](#prerequisites)
  - [Step 1: Complete local setup](#step-1-complete-local-setup)
  - [Step 2: Branches](#step-2-branches)
- [Making Code Changes](#making-code-changes)
  - [Step 3: OpenSpec](#step-3-openspec)
  - [Step 4: Folder structure](#step-4-folder-structure)
- [Development](development.md)

## Prerequisites

### Step 1: Complete local setup

Before contributing, make sure you have the following installed:

- **Go 1.26 or later** — [download](https://go.dev/dl/)
- **Git**
- **golangci-lint** — the linter used in CI:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

Clone the repository and install dependencies:

```bash
git clone https://github.com/dynatrace-oss/dtwiz.git
cd dtwiz
go mod download
```

Verify everything works:

```bash
make build # build the binary
make lint # run code linting
make test # run unit tests
```

### Step 2: Branches

Please use the following branch naming conventions. They allow us to understand the intent behind your changes.

| Branch prefix | Use for |
|---|---|
| `feature/<name>` | New functionality |
| `bugfix/<name>` | Bug fixes |
| `hotfix/<name>` | Urgent fixes that need to go out immediately |
| `chore/<name>` | Maintenance: dependency updates, CI, docs, tooling |

The repository ships a shared git alias file at [`.github/gitconfig-shared-aliases`](../../.github/gitconfig-shared-aliases) which provides shorthand commands for creating branches with the correct prefix.

You can add it to the local git configuration of this repository:

```bash
cd dtwiz
git config --local include.path ../.github/gitconfig-shared-aliases
```

When you start working on a new feature, a bugfix or a chore, you should create your own branch with a descriptive name for the changes you're about to make.

| Command | Creates branch |
|---|---|
| `git feature <name>` | `feature/<name>` |
| `git bugfix <name>` | `bugfix/<name>` |
| `git hotfix <name>` | `hotfix/<name>` |
| `git chore <name>` | `chore/<name>` |

Example:

```bash
git feature add-integration-testing-setup
# equivalent to: git switch -c feature/add-integration-testing-setup
```

## Making Code Changes

### Step 3: OpenSpec

[OpenSpec](https://openspec.dev/) is a lightweight, spec-driven framework that helps developers plan and document feature changes through living specifications stored directly in the codebase, compatible with multiple coding agents and AI tools.

We use OpenSpec for significant changes — new commands, new packages, architectural decisions, or meaningful behavior changes. This gives us and our AI agents proper context during the development process.

See the full guide: [openspec.md](openspec.md).

### Step 4: Development guide

Before writing code, please refer to [development.md](development.md) — it covers folder structure, Go best practices, and feature flags.
