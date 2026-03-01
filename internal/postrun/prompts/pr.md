Generate a GitHub pull request title and body for the following changes.

**Rules:**

- Title: one line, under 72 characters, describes the change concisely
- Body: explain _why_ these changes were made, using the PRD context below — not a raw diff summary
- Body should use markdown formatting
- Output format: title on the first line, then a blank line, then the body
- Output only the PR content — no preamble, no code fences

## PRD Context

{{.PRDContent}}

## Diff Summary

{{.DiffStat}}
