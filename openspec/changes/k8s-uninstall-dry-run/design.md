## Context

`dtwiz uninstall kubernetes` executes four destructive steps against a live cluster: deleting CRs, waiting for pods, helm uninstall, and namespace deletion. All other uninstall subcommands already support `--dry-run` via the shared `uninstallDryRun` flag on the parent command — this subcommand was the only exception.

## Goals / Non-Goals

**Goals:**
- Make `dtwiz uninstall kubernetes --dry-run` print the uninstall plan (cluster context + steps) without executing any kubectl or helm commands
- Keep the dry-run path consistent with how other uninstall subcommands behave

**Non-Goals:**
- Simulating or validating what the commands would return (e.g. checking if the namespace exists)
- Adding dry-run to the install kubernetes path (separate concern)

## Decisions

**Pass `dryRun bool` as a parameter to `UninstallKubernetes`**

The function already receives `kubeCtx` and `distro` as parameters; adding `dryRun` keeps the same style. The alternative — reading a package-level variable — would make the function harder to test in isolation. The cmd layer already holds `uninstallDryRun` and passes it to all other uninstall functions, so this is consistent.

**Extract `printK8sUninstallSteps()` helper**

The four steps are printed in both the dry-run preview and the live confirmation prompt. Extracting them avoids duplication and ensures dry-run always reflects the real execution plan.

**Early return on dry-run before confirmation prompt**

The confirmation prompt (`confirmProceed`) is only meaningful when commands will actually run. Dry-run exits after printing the plan, so the prompt is never shown — consistent with how other dry-run implementations work.

## Risks / Trade-offs

- The dry-run output does not verify whether the cluster is reachable or the namespace exists. Users may see a clean dry-run output but encounter failures on the real run. → Acceptable: this is the standard dry-run contract across the CLI.
