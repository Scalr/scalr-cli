# Changelog

## v0.next — UX & Scripting Overhaul

This release transforms the Scalr CLI from a raw API wrapper into a productivity tool for both interactive use and CI/CD automation. Every change is backward compatible — existing scripts that parse JSON output continue to work unchanged.

---

### Table Output (auto-detected)

Use `-format=table` to get aligned, scannable output. JSON remains the default for full backward compatibility — existing scripts are not affected.

```
$ scalr -format=table list-workspaces
ID            NAME           STATUS     TERRAFORM-VERSION  AUTO-APPLY  EXECUTION-MODE
----------    -----------    --------   -----------------  ----------  --------------
ws-abc123     production     applied    1.7.0              true        remote
ws-def456     staging        planned    1.7.0              false       remote
ws-ghi789     dev-sandbox    applied    1.6.6              true        local
(page 1 of 1, 3 total)

$ scalr list-workspaces              # JSON by default — no breaking change
[
  { "id": "ws-abc123", "name": "production", ... },
  ...
]
```

**Why this matters:** Scanning 50 workspaces in raw JSON required mental gymnastics. Tables let you find what you need at a glance.

---

### CSV Export

Pipe your infrastructure inventory straight into spreadsheets or text-processing tools.

```
$ scalr list-workspaces -format=csv > workspaces.csv
$ scalr list-environments -format=csv | cut -d',' -f1,2
```

---

### Human-Readable Errors

API errors used to dump raw JSONAPI objects. Now you get a single clear line.

**Before:**
```json
{
  "errors": [
    {
      "status": "422",
      "title": "Unprocessable Entity",
      "detail": "Name has already been taken",
      "source": { "pointer": "/data/attributes/name" }
    }
  ]
}
```

**After:**
```
Error: 422: Unprocessable Entity: Name has already been taken (field: /data/attributes/name)
```

---

### Command Aliases

Short names for the commands you type most. Full names still work.

| Alias | Expands to |
|-------|-----------|
| `ws` | `list-workspaces` |
| `envs` | `list-environments` |
| `runs` | `list-runs` |
| `vars` | `list-variables` |
| `tags` | `list-tags` |
| `accs` | `list-accounts` |
| `pols` | `list-policy-groups` |
| `sa` | `list-service-accounts` |
| `teams` | `list-teams` |
| `users` | `list-users` |
| `vcs` | `list-vcs-providers` |

```
$ scalr ws                    # same as: scalr list-workspaces
$ scalr envs -format=csv      # same as: scalr list-environments -format=csv
```

---

### Field Selection

Only show the fields you care about. Works with all output formats.

```
$ scalr list-workspaces -fields=id,name,status
ID          NAME          STATUS
--------    ----------    --------
ws-abc123   production    applied
ws-def456   staging       planned

$ scalr get-workspace -workspace=ws-abc123 -fields=name,terraform-version -format=json
{
  "name": "production",
  "terraform-version": "1.7.0"
}
```

---

### Column Control for Tables

Override which columns appear and their order.

```
$ scalr list-runs -columns=id,status,source,created-at
```

---

### Pagination Control

The CLI used to silently fetch every page. Now you can browse page by page or control the batch size.

```
$ scalr list-workspaces -page=1 -page-size=5     # first 5 results
$ scalr list-workspaces -page=2 -page-size=5     # next 5
```

Default behavior (fetch all pages) is preserved when you don't set these flags.

---

### Dot-Path Queries

Extract specific values without piping through `jq`.

```
$ scalr get-workspace -workspace=ws-abc123 -query=.name
production

$ scalr list-workspaces -query=.[].id
ws-abc123
ws-def456
ws-ghi789

$ scalr list-workspaces -query=.[].name
production
staging
dev-sandbox
```

Simple scalar values print as plain text (one per line). Complex values print as JSON.

---

### Wait for Run Completion

The most-requested feature for CI/CD pipelines. Instead of writing a shell loop that polls and parses JSON, use one command.

```
$ scalr wait-for-run -run=run-abc123
Waiting for run run-abc123...
Status: pending
pending -> planning
planning -> planned
planned -> applying
applying -> applied
Run run-abc123 completed successfully (applied)
```

The command prints status transitions to stderr and the final run data to stdout. Exit code 0 on success, 1 on failure.

It also detects states that require human action (policy approval, cost review, apply confirmation) and exits immediately instead of hanging:

```
$ scalr wait-for-run -run=run-def456
Waiting for run run-def456...
Status: pending
pending -> planning
planning -> policy_checked
Run run-def456 requires approval (status: policy_checked). Cannot proceed automatically.
```

Set a custom timeout with `-timeout=10m` (default: 30 minutes).

---

### Name-to-ID Resolution

Stop looking up IDs before every command. Use names directly — the CLI resolves them for you.

```
$ scalr get-workspace -workspace=production
Resolved workspace 'production' -> ws-abc123
{ ... }

$ scalr list-runs -workspace=staging
Resolved workspace 'staging' -> ws-def456
{ ... }
```

If the name matches multiple resources, you'll see the options:
```
Error: Multiple workspace resources match name 'test':
  ws-abc123  test
  ws-def456  test-2
Please specify the exact ID.
```

Values that already look like IDs (e.g., `ws-abc123`) skip resolution entirely — zero overhead for existing usage.

Supported resources: workspace, environment, account, tag, role, team, vcs-provider, agent-pool.

---

### Configuration Profiles

Manage multiple Scalr instances without juggling environment variables.

```
$ scalr -configure                    # creates "default" profile (backward compatible)

# Manually add profiles to ~/.scalr/scalr.conf:
{
  "default":  { "hostname": "prod.scalr.io",    "token": "...", "account": "acc-xxx" },
  "staging":  { "hostname": "staging.scalr.io",  "token": "...", "account": "acc-yyy" }
}

$ scalr ws                            # uses "default"
$ scalr ws -profile=staging           # uses "staging"
$ SCALR_PROFILE=staging scalr ws      # same thing via env var
```

Existing flat-format `scalr.conf` files continue to work without changes.

---

### Progress Spinner

A subtle spinner shows that the CLI is working during API calls. Automatically hidden when output is piped or stderr is not a terminal.

```
$ scalr list-workspaces
/ Fetching page 2...
```

---

### Scripting & CI/CD Improvements

#### Structured Exit Codes

Scripts can now distinguish between error types for proper retry logic.

| Code | Meaning | Action |
|------|---------|--------|
| **0** | Success | Continue |
| **1** | Permanent error (4xx, validation, approval needed) | Fail the pipeline |
| **2** | Usage error (missing flags, unknown command) | Fix the command |
| **3** | Transient error (5xx, network, timeout) | Retry safely |

```bash
scalr create-workspace -name=prod -environment-id=env-xxx
rc=$?
case $rc in
  0) echo "Created" ;;
  3) echo "Server issue, retrying..." && sleep 5 && retry ;;
  *) echo "Failed permanently" && exit 1 ;;
esac
```

#### Automatic Retry for Server Errors

API calls now retry automatically (up to 3 times with exponential backoff) on 5xx server errors and network failures. Client errors (4xx) fail immediately — no wasted time retrying bad requests.

#### HTTP Request Timeout

All requests have a 30-second timeout. Scripts no longer hang indefinitely on unresponsive servers.

#### Clean stdout/stderr Separation

All data output (JSON, table, CSV) goes to **stdout**. All diagnostics (errors, warnings, progress, resolution messages, verbose traces) go to **stderr**. This includes `-verbose` mode, which previously leaked HTTP debug info to stdout and broke JSON parsing.

```bash
# This now works correctly — verbose debug goes to stderr, clean JSON to stdout:
scalr list-workspaces -verbose 2>/dev/null | jq '.[0].name'
```

#### No-Color Mode

ANSI colors are automatically disabled when `CI` or `NO_COLOR` environment variables are set (GitHub Actions, GitLab CI, etc.). Also available as `-no-color` flag.

```yaml
# GitHub Actions — colors disabled automatically
- run: scalr list-workspaces

# Or explicitly:
- run: scalr list-workspaces -no-color
```

---

### Summary of New Flags

| Flag | Description |
|------|-------------|
| `-format=STRING` | Output format: `json`, `table`, `csv` (auto-detects TTY) |
| `-columns=LIST` | Columns to show in table/csv (comma-separated) |
| `-fields=LIST` | Fields to include in output, all formats (comma-separated) |
| `-query=STRING` | Dot-path expression (`.name`, `.[].id`) |
| `-page=INT` | Fetch a specific page only |
| `-page-size=INT` | Items per page (default: 100) |
| `-profile=STRING` | Named config profile from `scalr.conf` |
| `-no-color` | Disable ANSI colors |

### New Commands

| Command | Description |
|---------|-------------|
| `wait-for-run` | Poll a run until completion (flags: `-run`, `-timeout`) |
