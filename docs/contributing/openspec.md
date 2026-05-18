# OpenSpec

## Table of Contents

- [Overview](#overview)
- [When to Use OpenSpec](#when-to-use-openspec)
- [Workflow](#workflow)
- [PR Structure](#pr-structure)

## Overview

OpenSpec is a lightweight design-first workflow built into this repository. It requires you to articulate the problem, design the approach, and agree on scope — all before writing production code. This avoids large PRs being rejected late due to architectural disagreement, and gives maintainers a chance to flag concerns or suggest alternatives early.

OpenSpec artifacts live under `openspec/` and are managed through skills available in Claude Code, GitHub Copilot, and OpenCode.

## When to Use OpenSpec

| Required | Not required |
|---|---|
| New CLI commands | Typo or documentation-only fixes |
| New packages | Minor bug fixes |
| Significant behavior changes | Trivial chores |
| Architectural decisions | |

When in doubt, open a GitHub Issue first and ask.

## Workflow

| Step | Skill | What it does | Output |
|---|---|---|---|
| 1 | `/opsx:explore` | Think through the problem, gather context | No files |
| 2 | `/opsx:propose` | Generate proposal, design, and task list | `openspec/changes/<name>/` with `proposal.md`, `design.md`, `tasks.md`, and `specs/` |
| 3 | `/opsx:apply` | Implement the tasks | Feature code and tests; `tasks.md` updated with progress |
| 4 | `/opsx:archive` | Finalize and archive the completed change | Moves change to `openspec/changes/archive/<date>-<name>/`; copies specs to `openspec/specs/<name>/` |

Configuration lives in `openspec/config.yaml`.

## PR Structure

1. **Proposal PR** *(optional)* — covers steps 1–2. Merge the OpenSpec artifacts before starting implementation. If skipped, include the artifacts in the first implementation PR instead.
2. **Implementation PR** — covers step 3.
3. **Archive PR** — covers step 4.
