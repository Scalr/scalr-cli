# Changelog

## v0.next — UX & Scripting Overhaul

This release adds output formatting, better errors, CI/CD-friendly defaults, and a `wait-for-run` command — without breaking any existing usage. JSON remains the default output format and exit code 1 still covers all error cases.

> **Note on flag order:** Global flags (`-format`, `-fields`, `-page`, etc.) must appear **before** the operation name. This matches Go's `flag` package convention. Sub-flags specific to an operation come **after** it.
>
> Correct: `scalr -format=table get-workspaces`
> Wrong:   `scalr get-workspaces -format=table`

---

### Table Output

Use `-format=table` for aligned, scannable output in your terminal. JSON stays the default, so scripts parsing stdout continue to work unchanged.

```
$ scalr -format=table get-workspaces
ID                    NAME       ENVIRONMENT-ID         TERRAFORM-VERSION  AUTO-APPLY
--                    ----       --------------         -----------------  ----------
ws-v0p7nqiupjln2e1dv  ws-1       env-v0ord4r0sthdi9es5  1.7.5              false
ws-v0p7ns9tcjerbbp3d  test       env-v0p7nr1375dh87uk4  1.7.5              true
(4 total)

$ scalr get-workspaces                      # JSON by default — scripts unaffected
[ { "id": "ws-v0p7nqiupjln2e1dv", ... } ]
```

Why this matters: scanning dozens of workspaces as raw JSON is painful. Tables let you spot what you need at a glance.

---

### CSV Export

```
$ scalr -format=csv get-workspaces > workspaces.csv
$ scalr -format=csv list-environments | cut -d',' -f1,2
```

CSV is RFC 4180-compliant and guards against spreadsheet formula injection — values starting with `=`, `+`, `-`, `@` are safely prefixed.

---

### Human-Readable Errors

API errors used to dump raw JSONAPI objects. Now you get a single readable line.

**Before:**
```json
{
  "errors": [
    { "status": "422", "title": "Unprocessable Entity", "detail": "Name has already been taken", "source": { "pointer": "/data/attributes/name" } }
  ]
}
```

**After:**
```
Error: 422: Unprocessable Entity: Name has already been taken (field: /data/attributes/name)
```

---

### Command Aliases

Short names for frequently used commands. Full operation names still work.

| Alias | Expands to |
|-------|-----------|
| `ws` | `get-workspaces` |
| `envs` | `list-environments` |
| `runs` | `get-runs` |
| `vars` | `get-variables` |
| `tags` | `list-tags` |
| `accs` | `get-accounts` |
| `pols` | `list-policy-groups` |
| `sa` | `get-service-accounts` |
| `teams` | `get-teams` |
| `users` | `get-users` |
| `vcs` | `list-vcs-providers` |
| `mods` | `list-modules` |

```
$ scalr ws                      # same as: scalr get-workspaces
$ scalr -format=csv envs        # same as: scalr -format=csv list-environments
```

The inconsistency between `get-*` and `list-*` prefixes reflects the underlying Scalr API spec — some list operations are named `get_<plural>`, others `list_<plural>`.

---

### Field Selection

Only show the fields you care about. Works across all output formats and controls column order in table/CSV.

```
$ scalr -fields=id,name get-workspaces
[
  { "id": "ws-v0p7nqiupjln2e1dv", "name": "ws-1" },
  { "id": "ws-v0p7ns9tcjerbbp3d", "name": "test" }
]

$ scalr -format=table -fields=id,name get-workspaces
ID                    NAME
--                    ----
ws-v0p7nqiupjln2e1dv  ws-1
ws-v0p7ns9tcjerbbp3d  test

$ scalr -fields=id,name get-workspace -workspace=test
Resolved workspace 'test' -> ws-v0p7ns9tcjerbbp3d
{ "id": "ws-v0p7ns9tcjerbbp3d", "name": "test" }
```

---

### Pagination Control

The CLI still fetches all pages by default. Now you can also browse page-by-page or fetch a specific page for faster responses on large datasets.

```
$ scalr -page=1 -page-size=5 get-workspaces     # first 5 results
$ scalr -page=2 -page-size=5 get-workspaces     # next 5
```

When all pages are fetched (default), a summary line goes to stderr:
```
(42 total)
```

When a specific page is requested, the summary includes page context:
```
(page 2 of 9, 42 total)
```

---

### Dot-Path Queries

Extract specific values without piping to `jq`.

```
$ scalr -query=.name get-workspace -workspace=ws-v0p7nqiupjln2e1dv
ws-1

$ scalr '-query=.[].id' get-workspaces
ws-v0p7nqiupjln2e1dv
ws-v0p7ns9tcjerbbp3d

$ scalr '-query=.[].name' get-workspaces
ws-1
test
```

> **Shell tip:** zsh treats `.[]` as a glob pattern. Quote the flag: `'-query=.[].id'`. Bash does not need the quotes.

Simple scalar values print as plain text (one per line). Complex values print as JSON.

---

### Open in Browser

Jump straight to the Scalr dashboard from the terminal.

```
$ scalr open account                          # account dashboard
$ scalr open environment tfenv1               # environment by name
$ scalr open workspace test                   # workspace by name
$ scalr open run run-v0p7nx...                # specific run
```

Short aliases also work: `scalr open env tfenv1`, `scalr open ws test`.

For workspaces and runs the CLI resolves parent IDs automatically (workspace's environment, run's workspace and environment). The URL is printed to stderr so you see it even in headless environments. Uses `open` (macOS), `xdg-open` (Linux), `rundll32` (Windows).

---

### Wait for Run Completion

The most-requested feature for CI/CD pipelines. One command replaces the shell-loop + JSON-parse pattern.

```
$ scalr wait-for-run -run=run-v0p7nxxxx
Waiting for run run-v0p7nxxxx...
Status: pending
pending -> planning
planning -> planned
planned -> applying
applying -> applied
Run run-v0p7nxxxx completed successfully (applied)
```

Exit code 0 on success, 1 on failure. Final run data is printed to stdout.

The command detects states that **definitely** block on human input and exits immediately instead of polling forever:

- `policy_checked` — soft-mandatory policy failed, needs override
- `policy_override` — override requested, awaiting action
- `cost_estimated` — cost estimate awaiting approval (when gated)
- `planned` — only when the run has `auto-apply=false` (otherwise it continues polling through `confirmed` -> `applying`)

```
$ scalr wait-for-run -run=run-v0p7ndxxxx
Waiting for run run-v0p7ndxxxx...
Status: pending
pending -> planning
planning -> policy_checked
Run run-v0p7ndxxxx is blocked waiting for approval (status: policy_checked). Cannot proceed automatically.
```

Custom timeout via `-timeout=10m` (default: 30 minutes).

Note: Detecting "requires approval" purely from run status is best-effort — the full answer depends on policy-check stages, cost estimation, auto-apply, and whether the run is plan-only. The detection covers the common cases.

---

### Configuration Profiles

Switch between Scalr instances (prod/staging/etc.) without juggling environment variables.

```
# ~/.scalr/scalr.conf
{
  "default":  { "hostname": "prod.scalr.io",    "token": "...", "account": "acc-xxx" },
  "staging":  { "hostname": "staging.scalr.io", "token": "...", "account": "acc-yyy" }
}
```

```
$ scalr ws                              # uses default
$ scalr -profile=staging ws             # uses staging
$ SCALR_PROFILE=staging scalr ws        # same via env var
```

Existing flat-format `scalr.conf` files (the pre-v0.next format) continue to work — they are treated as the `default` profile automatically.

---

### Progress Spinner

A subtle spinner on stderr shows the CLI is working during API calls. Automatically hidden when output is piped or stderr is not a terminal.

```
$ scalr -format=table get-workspaces
/ Fetching page 2...
```

---

### Name-to-ID Resolution

Stop looking up IDs before every command. Pass names directly — the CLI resolves them automatically for path and query parameters.

```
$ scalr get-workspace -workspace=test
Resolved workspace 'test' -> ws-v0p7ns9tcjerbbp3d
{ ... }
```

Multiple matches produce a clear error:
```
Error: Multiple workspace resources match name 'test':
  ws-v0p7ns9tcjerbbp3d  test
  ws-v0p7nt4lgen59ni26  test-2
Please specify the exact ID.
```

Values that already look like a Scalr ID (e.g., `ws-v0p7xxx`) skip resolution entirely — zero overhead for existing scripts.

Resolution is attempted for: workspace, environment, account, tag, role, team, vcs-provider, agent-pool.

---

## Scripting & CI/CD Improvements

### Exit Codes

| Code | Meaning | Action |
|------|---------|--------|
| **0** | Success | Continue |
| **1** | Any error (bad input, 4xx, missing flags, not found, approval required) | Fail the pipeline |
| **3** | Transient error (5xx, network, timeout) | Retry safely |

Exit code 3 is new and additive. Exit code 1 is unchanged — scripts that check `!= 0` continue to work. Scripts that want smarter retry can check for 3 specifically:

```bash
scalr create-workspace -name=prod -environment-id=env-xxx
rc=$?
case $rc in
  0) echo "created" ;;
  3) echo "server issue, retrying..." && sleep 5 && retry ;;
  *) echo "failed" && exit 1 ;;
esac
```

### Automatic Retry

API calls now retry up to 3 times with exponential backoff (1s, 2s, 4s) on 5xx server errors and network failures. Client errors (4xx) fail immediately — no wasted time retrying bad requests.

POST/PATCH/DELETE request bodies are correctly preserved across retries (previous versions re-sent empty bodies).

### HTTP Request Timeout

Every request has a 5-minute timeout. Scripts no longer hang indefinitely on unresponsive servers, but long-running operations (large applies, policy checks) still have ample time to complete.

### Clean stdout / stderr Separation

Data output (JSON, table, CSV, query results) goes to **stdout**. Everything else — errors, warnings, progress indicators, resolution messages, verbose traces — goes to **stderr**. Scripts parsing stdout as JSON now work even with `-verbose`:

```bash
scalr -verbose get-workspaces 2>/dev/null | jq '.[0].name'
```

### No-Color Mode

ANSI colors are disabled automatically when `NO_COLOR` (per [no-color.org](https://no-color.org)) or `CI` env vars are set. Works out of the box in GitHub Actions, GitLab CI, CircleCI, Jenkins, etc.

```yaml
# GitHub Actions — CI is set, colors are off automatically
- run: scalr -format=table get-workspaces
```

---

## Summary of New Flags

Global flags (must appear before the operation):

| Flag | Description |
|------|-------------|
| `-format=STRING` | Output format: `json` (default), `table`, `csv` |
| `-fields=LIST` | Comma-separated field list (filters output and sets table/csv column order) |
| `-query=STRING` | Dot-path expression like `.name` or `.[].id` |
| `-page=INT` | Fetch a specific page only (default: fetch all) |
| `-page-size=INT` | Items per page (default: 100) |
| `-profile=STRING` | Named config profile from `scalr.conf` |
| `-no-color` | Disable ANSI colors (also: `NO_COLOR` or `CI` env vars) |

## New Commands

| Command | Description |
|---------|-------------|
| `wait-for-run` | Poll a run until completion (flags: `-run`, `-timeout`) |
| `open` | Open Scalr dashboard in browser (`open workspace <name>`, `open run <id>`, etc.) |

## Breaking Changes

**None intended.** Every change is additive or restores correct behavior. Specifically preserved for backward compatibility:

- JSON is still the default output format.
- Exit code 1 is still returned for every error case the previous CLI returned 1 for.
- Pagination still fetches all pages by default.
- The old flat `scalr.conf` format is still supported.
- All existing command names work unchanged.

One genuine behavior change that could affect scripts reading stdout:
- Error messages and `-verbose` output moved from stdout to stderr. This was a bug — errors and debug traces on stdout corrupted JSON parsing. If a script reads API error messages from stdout, it will need to read stderr now.
