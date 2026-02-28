# Context Map

Index of all project context files. Updated when new domains or files are added.

## Core Documentation

- [`summary.md`](summary.md) — What, Architecture, Core Flow, System State, Capabilities, Tech Stack
- [`terminology.md`](terminology.md) — Term definitions (PRD, Task, Iteration, Step, Workflow, State, etc.)
- [`practices.md`](practices.md) — Development workflow, TDD requirements, quality checks, code organization

## Domain: Prompts

LLM prompt templates and rendering functions.

- [`prompts/prompts.md`](prompts/prompts.md) — Embedded prompt templates (Implement, Ensure Completeness, Lint and Test, Code Review, Apply Fixes, Update Docs, Commit, Memory Update, Task Summary); template pattern; integration points

## Domain: UI

Text formatting, styling, and terminal output.

- [`ui/formatting.md`](ui/formatting.md) — Header, step, status, duration formatting functions; color/style system; integration points

## Domain: Snapshot

Git stash-based workflow checkpoints.

- [`snapshot/snapshots.md`](snapshot/snapshots.md) — Snapshot capture implementation, integration with runner, use cases, git interactions

## Domain: Workflow

Task orchestration, runner, state management, and task discovery.

- [`workflow/runner.md`](workflow/runner.md) — Runner overview, 10-step iteration workflow, snapshot capture, task duration tracking, state management, control flow
- [`workflow/tasks.md`](workflow/tasks.md) — Task file format and naming, task scanning, discovery diagnostics (case mismatch, PRD headers), error formatting, integration points

## Domain: CLI

Command-line interface features and functionality.

- [`cli/run.md`](cli/run.md) — Run command with session support, named sessions, auto-detection, legacy fallback, session resolution logic, testing
- [`cli/plan.md`](cli/plan.md) — Plan command, two-phase planning pipeline, conflict guard with tap.Select/tap.Text, interactive input via tap.Textarea (TTY) and buffered scanner input (pipes), autonomous document generation, --from flag, session resolution, plan resumption, provider integration
- [`cli/status.md`](cli/status.md) — Status command, session status display, task completion state, step progress, session resolution, output formatting
- [`cli/versioning.md`](cli/versioning.md) — Version flag implementation, build-time injection via ldflags, E2E testing, usage examples
- [`cli/provider.md`](cli/provider.md) — Provider CLI validation, pre-flight checks, error formatting, provider metadata, cross-provider support
- [`cli/signals.md`](cli/signals.md) — Signal handling, OS interrupt flow, exit code mapping, graceful shutdown, signal safety
- [`cli/show-state.md`](cli/show-state.md) — State inspection, human-readable summary, JSON output, step name mapping, use cases
- [`cli/color.md`](cli/color.md) — Color output control, NO_COLOR environment variable, TTY detection, dynamic evaluation, E2E testing
- [`cli/sessions.md`](cli/sessions.md) — Session management, named workspaces, session creation/deletion/listing, status derivation, confirmation prompts, integration tests

## Domain: Infrastructure

Build, deployment, CI/CD infrastructure, and post-run operations.

- [`infra/ci.md`](infra/ci.md) — GitHub Actions CI workflow, lint and race-condition testing, YAML validation tests
- [`infra/release.md`](infra/release.md) — Release automation workflow, GoReleaser configuration, version injection, multi-platform builds, release testing
- [`infra/postrun.md`](infra/postrun.md) — Post-completion workflow, git remote detection, auto-push to origin, GitHub PR creation with LLM-generated title and body, gh CLI integration

---

**Structure**: One topic per file, ~250 lines max, relative links
**Rules**: No dates, commits, status; current-state only; code is source of truth
