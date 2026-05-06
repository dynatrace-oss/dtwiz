# Contributing to dtwiz

Thank you for your interest in contributing to dtwiz! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [How to Contribute](#how-to-contribute)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing Requirements](#testing-requirements)
- [Commit Messages](#commit-messages)
- [Reporting Issues](#reporting-issues)
- [CI Requirements](#ci-requirements)

## Code of Conduct

This project adheres to a Code of Conduct that all contributors are expected to follow. Please read [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) before contributing.

## Getting Started

### Prerequisites

- Go 1.26 or later
- Git

### Development Setup

1. **Clone the repository**:

   ```bash
   git clone https://github.com/dynatrace-oss/dtwiz.git
   cd dtwiz
   ```

2. **Install dependencies**:

   ```bash
   go mod download
   ```

3. **Build the project**:

   ```bash
   make build
   ```

4. **Run tests**:

   ```bash
   make test
   ```

5. **Run linters**:

   ```bash
   make lint
   ```

6. **Install development tools**:

   ```bash
   # Install linter
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```

## How to Contribute

### Ways to Contribute

- **Report bugs** — Help us identify and fix issues
- **Suggest features** — Share ideas for improvements
- **Write documentation** — Improve guides and examples
- **Submit code** — Fix bugs or implement new features
- **Review pull requests** — Help review and test contributions

### Finding Issues to Work On

- Check the [current issues](https://github.com/dynatrace-oss/dtwiz/issues)
- Look for issues labeled `good first issue` or `help wanted`
- Ask in the issue or discussion thread if you're unsure where to start

### OpenSpec Workflow

For all major features and significant bug fixes, an OpenSpec design document is required **before** implementation begins. This ensures alignment on the approach and avoids wasted effort.

**When OpenSpec is required**: new commands, new packages, significant behavior changes, architectural decisions.

**When OpenSpec is NOT required**: typos, documentation-only changes, trivial chores, minor bug fixes.

**Skills** (run inside Claude Code, GitHub Copilot, or OpenCode):

| Step | Skill | Purpose | Output |
|------|-------|---------|--------|
| 1 | `/opsx:explore` | Think through the problem, gather context | No files |
| 2 | `/opsx:propose` | Generate proposal, design, and task list | `openspec/changes/<name>/` with `proposal.md`, `design.md`, `tasks.md`; generates `spec.md` files under `openspec/changes/<name>/specs/` |
| 3 | `/opsx:apply` | Implement the tasks from the proposal | Feature code and tests; `tasks.md` updated with progress |
| 4 | `/opsx:archive` | Finalize and archive the completed change | Moves change to `openspec/changes/archive/<date>-<name>/`; copies `spec.md` files to `openspec/specs/<name>/` for permanent reference |

OpenSpec configuration lives in `openspec/config.yaml`.

**PR structure**:

1. **Proposal PR** *(optional)* (steps 1–2): merge the OpenSpec artifacts before starting implementation; if skipped, the OpenSpec artifacts must be included in the first implementation PR
2. **Implementation PR** (step 3): implement the feature
3. **Archive PR** (step 4): archive the completed OpenSpec
