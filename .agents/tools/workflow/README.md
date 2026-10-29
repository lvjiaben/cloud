# Session audits and scoped checks

Run these commands from the repository root through the existing context entry:

```text
go run ./.agents/tools/context.go audit --thread 01a1533d-0a8d-7dd3-a0bb-c6b704a6716a
go run ./.agents/tools/context.go check --path .agents/tools --change streamline-agent-context --plan
go run ./.agents/tools/context.go check --path .agents/tools --change streamline-agent-context
```

`audit --file path/to/session.jsonl` selects a file explicitly. `--thread` finds one exact ID below `CODEX_HOME/sessions`, falling back to the user's `.codex/sessions`. Ambiguous IDs fail. The audit streams at most 1 GiB with an 8 MiB per-record limit and emits at most 8 KiB, including up to eight repeated-read paths and eight large text records. It never executes or copies source commands, messages, internal reasoning, credentials, or image encodings. Malformed or oversized records produce an incomplete report and nonzero exit.

Usage comes from the latest recorded counters. Missing counters remain null; cumulative inputs contain repeated context and cannot establish account costs. Repeated-read detection covers `cat` and `sed -n` inside JSON-quoted command arguments; it is a hotspot heuristic, not proof that a reread was unnecessary. At most 4,096 distinct source paths are counted; exceeding this reports incomplete evidence. Tool names use a fixed vocabulary with other names grouped together. Text bytes, image blocks, and unclassified serialized output are counted separately. The audit does not estimate tokens from bytes.

`check` requires explicit repeatable `--path` or `--change` scopes. It does not adopt the whole dirty worktree. `--plan` runs no validation commands and creates no logs. Execution reuses package-scoped Go tests, one official `linactl lint.go` per owning module, standalone-tool tests and `go vet`, and strict validation of related active OpenSpec changes. Changes to `go.mod` or `go.sum` select the full owning module. Supported documents receive JSON syntax, whitespace, and local inline Markdown-link existence checks; this does not validate rendered layout or anchors.

Checks run serially, retaining each command's full output under timestamped `temp/YYYYMMDD/` logs. Reports include status, exit code, and log path only. Completed processes retain their actual exit code; unavailable or forbidden commands report `not_started` with code 127 or 126. `--timeout 2m` is the default per-command limit; at most `10m` is accepted. Timeouts report status `timeout` with the conventional step code 124. Any of these failures, and unconfigured categories such as frontend behavior or SQL validation, make the overall command exit nonzero. They are never shown as passed. Domain-specific acceptance, database scenarios, external state, and required E2E still need their task-specific evidence.

To keep planning bounded, one request accepts at most twenty paths, ten changes, two thousand files, thirty-two explicit Go packages per module, and twenty-four checks. A document is limited to 1 MiB and all document bodies to 16 MiB. Narrow the task when these boundaries are reached. Plans and summaries are capped at 8 KiB; complete source and logs remain available for targeted inspection.
