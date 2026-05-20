# Pull Requests

## Table of Contents

- [Prerequisites](#prerequisites)
  - [Step 1: Complete local setup](#step-1-complete-local-setup)
  - [Step 2: Branches](#step-2-branches)
- [Making Code Changes](#making-code-changes)
  - [Step 3: OpenSpec](#step-3-openspec)
  - [Step 4: Development guide](#step-4-development-guide)
  - [Step 5: Conventional commits](#step-5-conventional-commits)
  - [Step 6: Rebasing](#step-6-rebasing)
  - [Step 7: Testing](#step-7-testing)
  - [Step 8: Opening a Pull Request](#step-8-opening-a-pull-request)
  - [Step 9: Squash Merging](#step-9-squash-merging)
- [CI Workflows](#ci-workflows)

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

### Step 5: Conventional commits

All commits in this repository follow the [Conventional Commits](https://www.conventionalcommits.org/) specification. This keeps the history readable and makes it possible to generate changelogs with the help of AI agents, by providing them with proper context via our own commit messages.

Format: `<type>(<scope>): <subject>`

| Type | Use for |
|---|---|
| `feat` | New functionality |
| `fix` | Bug fixes |
| `docs` | Documentation-only changes |
| `chore` | Maintenance: dependencies, CI, tooling, release commits |
| `refactor` | Code restructuring with no behavior change |
| `test` | Adding or updating tests |
| `ci` | CI/CD pipeline changes |
| `perf` | Performance improvements |

The scope is optional but encouraged for larger codebases — use the package or command name (`otel`, `installer`, `analyzer`).

```text
feat(otel): add Python auto-instrumentation support
fix(installer): handle missing token gracefully
docs: update OpenSpec workflow guide
chore: release v0.3.0
```

The subject is a verb phrase in lowercase, present tense, no trailing period — describe what the commit does, not what it is (`add support for X`, not `X support`).

### Step 6: Rebasing

We use rebase instead of merge to keep a linear history. Before opening a PR, bring your branch up to date with `main`:

```bash
git fetch origin
git rebase origin/main
```

If you need to push after a local rebase:

```bash
git push --force-with-lease
```

`--force-with-lease` is safer than `--force` — it aborts if someone else pushed to the branch since your last fetch.

When you hit a conflict, resolve it file by file, then continue:

```bash
git add <resolved-file>
git rebase --continue
```

To abort the rebase and restore your branch to its pre-rebase state:

```bash
git rebase --abort
```

For conflicts in complex areas — the same installer file, a shared template, or anything touching core command logic — don't force a resolution that could silently break behavior. In that case, please open a PR as a draft and tag a maintainer for guidance.

### Step 7: Testing

Every change should include tests appropriate to its scope — unit tests for logic changes, integration tests for new commands or installer behavior. Before opening a PR, make sure all tests pass and no new lint issues are introduced. See the full guide: [testing.md](testing.md).

### Step 8: Opening a Pull Request

Once you're done with each commit, you can push the new changes to your remote branch:

```bash
git push -u origin HEAD
```

Once you're done with all the changes, you can open a Pull Request (PR). This gives us opportunity to review your code before it reaches production.

The repository provides a `git open-pr` alias that prints a GitHub comparison URL, pre-filled with the right PR template for your branch type, mentioned in the [branches](#step-2-branches) section:

```bash
git open-pr # run the command and click the recommended link to a new Github PR
```

PR templates live in [`.github/PULL_REQUEST_TEMPLATE/`](../../.github/PULL_REQUEST_TEMPLATE/):

| Template | Used for |
|---|---|
| `feature.md` | `feature/*` branches |
| `bugfix.md` | `bugfix/*`, `fix/*`, `hotfix/*` branches |
| `chore.md` | `chore/*`, `docs/*`, `refactor/*`, and similar maintenance branches |

**Before marking the PR as ready for review:**

1. Assign yourself to the PR
2. Fill in the template — description, motivation, and any other fields it asks for
3. Complete the checklist in the template
4. For `feature/*` and `bugfix/*` PRs, make sure Copilot has reviewed your code before requesting a maintainer review

### Step 9: Squash Merging

When a PR is approved and ready to merge, it should be **squash merged** into `main`. This means all commits in your branch should be combined into a single commit, preserving your PR description as the merge commit message.

## CI Workflows

The following checks run on every PR and must all pass before merging.

| Workflow | Trigger | What it does |
|---|---|---|
| **Build** | PR, manual | Compiles the binary with `make build` |
| **Tests** | Push to `main`, PR, manual | Runs `make test-coverage` on Ubuntu, macOS, and Windows; enforces a minimum coverage threshold; uploads a coverage report as a build artifact |
| **Go Lint** | PR, manual | Runs `golangci-lint` with the project's `.golangci.yml` configuration |
| **Markdown Linting** | PR, manual | Runs `make markdownlint` to lint Markdown files |
| **Dependency Review** | PR | Checks that no new dependency introduces a known vulnerability |
| **PR Title Check** | PR | Validates that the PR title follows Conventional Commits format and contains no ticket numbers |
| **PR Checklist** | PR | Validates that the PR description checklist is filled out |

Workflow definitions live in [`.github/workflows/`](../../.github/workflows/).

The coverage threshold is defined in the **Tests** workflow and enforced by `make test-coverage`. It will be raised gradually over time as coverage improves.
