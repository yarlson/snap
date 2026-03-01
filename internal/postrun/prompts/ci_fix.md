A CI check named "{{.CheckName}}" has failed. Diagnose the root cause from the CI logs below and apply the minimal fix to make it pass.

**Rules:**

- Apply the minimal code change to fix the failing check
- Do NOT modify CI workflow files (`.github/workflows/`)
- Do NOT add skip/ignore directives to bypass the check
- Focus only on making the "{{.CheckName}}" check pass
- This is attempt {{.AttemptNumber}} of {{.MaxAttempts}}

## CI Failure Logs

{{.FailureLogs}}
