# copycards

Copycards is a command-line tool for copying [Flowboards](https://kanban.plus/) tickets from one organisation to another. It reconciles bins, ticket types, and custom fields by name, maps users by email, preserves parent/child hierarchies, and is idempotent across re-runs.

## What it does

Flowboards doesn't offer an official cross-organisation copy for tickets. Copycards fills that gap:

- **Preflight** compares the source and destination boards, verifying that every bin, used ticket type, and used custom field in the source has a matching item in the destination (by name). Unmapped users surface here too.
- **Copy** fetches every ticket from each bin on the source board, allocates a fresh dst-side ID from the `/ids` endpoint, translates the payload (rewriting bin, type, user, custom-field, and checklist IDs), and posts it to the destination board.
- **Parent/child links** are restored in a second pass after all tickets exist.
- **A persistent `mapping.json`** records every src→dst ID pair. Re-runs consult it and skip tickets that are already copied, so interrupted migrations resume cleanly.
- **CloudFront WAF fallbacks**: if the full create is blocked, copycards retries as a minimal-create + `$partial` update. If the update is still blocked, byte-level sanitisation of the description is attempted (zero-width spaces in SQL-like keywords, spacing around `.../`) with an audit annotation appended so the change is visible.

## Prerequisites

- **Go ≥ 1.25** (the module targets 1.25.6 — check with `go version`)
- A Flowboards account with API keys for both the source and destination organisations

## Installation

### From source

```bash
git clone git@github.com:Germanicus1/copycards.git
cd copycards
go build -o copycards ./cmd/copycards
```

Either move the binary into your `PATH`:

```bash
sudo mv copycards /usr/local/bin/
```

…or install directly with `go install`:

```bash
go install ./cmd/copycards
# binary lands in $(go env GOBIN) or $(go env GOPATH)/bin
```

## Quick start

A full end-to-end migration takes five steps.

**1. Set your API keys.** Copy `example.env` and fill in the values, then source it into your shell:

```bash
cp example.env .env
# edit .env to set FB_KEY_SRC and FB_KEY_DST
source .env
```

**2. Create a config file** at `~/.config/copycards/config.toml` (or anywhere — pass `--config <path>` to pick):

```toml
default_from = "src"
default_to   = "dst"

[orgs.src]
org_id  = "abc123"
api_key = "env:FB_KEY_SRC"

[orgs.dst]
org_id  = "xyz789"
api_key = "env:FB_KEY_DST"
```

`endpoint` is optional — when omitted, copycards uses `https://n1.flowboards.kanban.plus/rest/2/<org_id>`. Override only if your org lives on a different host or you're pointing at a mock server.

**3. Verify both orgs authenticate:**

```bash
copycards orgs verify src
copycards orgs verify dst
```

**4. Pick the boards and preflight:**

```bash
# interactive board picker for each org
copycards board verify --from src --to dst
```

Preflight lists every mapping it derived and, if the boards are incompatible, a grouped report of what's missing. Fix those on the destination (add bins, create matching types, etc.) and re-run until it reports *Boards are compatible*.

**5. Dry-run, then copy for real:**

```bash
copycards tickets copy --from src --to dst \
  --src-board <src-board-id> --dst-board <dst-board-id> --dry-run

copycards tickets copy --from src --to dst \
  --src-board <src-board-id> --dst-board <dst-board-id>
```

If the run is interrupted, re-run the same command — already-copied tickets are skipped.

## Configuration

### Config file location

By default copycards reads `~/.config/copycards/config.toml`. Override with `--config <path>` — the flag value takes precedence for the duration of the run.

### Format

```toml
default_from = "src"
default_to   = "dst"

[orgs.src]
org_id   = "org_src_id"
api_key  = "env:FB_KEY_SRC"
# endpoint = "https://n1.flowboards.kanban.plus/rest/2/..."   # optional — defaulted from org_id

[orgs.dst]
org_id  = "org_dst_id"
api_key = "env:FB_KEY_DST"
```

Fields:

- `org_id` (required) — your organisation ID in Flowboards.
- `api_key` (required) — either a literal token or `env:VARNAME` to read from an environment variable. The `env:` form is strongly recommended; plaintext keys in a config on disk are almost always a mistake.
- `endpoint` (optional) — override auto-discovery. Useful for testing against mock servers.

### Environment variable expansion

Any `api_key = "env:FOO"` is resolved by reading `$FOO` when the config is loaded. If the variable is unset, copycards refuses to start with `env var FOO not set for org <name>`. Keep your secrets in a `.env`-style file that you `source` before invoking copycards — `example.env` ships with the two variable names the default config uses.

### Endpoint resolution

The Flowboards REST endpoint for any org is deterministic: `https://n1.flowboards.kanban.plus/rest/2/<org_id>`. Copycards builds that URL from `org_id` and uses it for every call — no discovery round-trip, no cache.

Set `endpoint = "..."` in an org block to override (useful for mock servers, or if your org is on a different host).

## State directory (`.copycard/`)

Copycards persists run state in `.copycard/`. It's created on demand — you don't need to make it yourself, and it's git-ignored.

- **`./.copycard/mapping.json`** — the src→dst ID translation table. Written after each non-dry-run copy in the cwd. Re-runs consult it to skip already-copied tickets, which is what makes `tickets copy` idempotent.
- **`~/.copycard/mapping.json`** — what `mapping show` and `mapping reset` read. If you ran the copy from a directory other than `$HOME`, the two paths will diverge; run `mapping show` from the same cwd, or move the file into `$HOME/.copycard/`.
- **`~/.copycard/failed-posts/`** — forensic dump directory. Any POST/PUT that exhausts retries writes `<timestamp>-<METHOD>-<id>.json` (the full request body) and `<timestamp>-<METHOD>-<id>.error.txt` (the terminal error). Nothing is dumped on successful runs.

Delete either directory freely. Mapping loss means the next copy re-creates duplicates of anything not already in the destination; failed-post dumps are purely diagnostic.

## Commands

#### orgs list
```bash
copycards orgs list
```
Lists configured org profiles with their resolved endpoints.

#### orgs verify
```bash
copycards orgs verify <profile>
```
Confirms the API key works by fetching the board list.

#### boards list
```bash
copycards boards list --from <profile>
```
Interactive numbered menu of boards in an org.

#### board verify
```bash
copycards board verify --from <src> --to <dst> [--src-board <id>] [--dst-board <id>]
```
Runs preflight. Omitting either board flag triggers an interactive picker. Reports the derived bin/type/field/user mappings, or a grouped list of incompatibilities.

#### tickets copy
```bash
copycards tickets copy --from <src> --to <dst> \
  --src-board <id> --dst-board <id> \
  [--dry-run]
```
Full board copy: preflight → enumerate → topologically sort → create tickets → restore parent/child links. Idempotent.

#### ticket copy
```bash
copycards ticket copy <id> --from <src> --to <dst> \
  --dst-board <id> \
  [--with-children] [--dry-run]
```
Copy a single ticket. `--with-children` pulls the subtree.

#### diff
```bash
copycards diff --from <src> --to <dst> --src-board <id> --dst-board <id>
```
Lists source tickets not yet present in the mapping — useful to see what a partial run still has to pick up.

#### mapping show
```bash
copycards mapping show
```
Prints summary counts for the current `~/.copycard/mapping.json` and the first 20 ticket mappings.

#### mapping reset
```bash
copycards mapping reset
```
Deletes `~/.copycard/mapping.json` after confirmation.

## How it works

### Preflight

`copier.Preflight` calls:
- `GET /boards/<id>` on each side to get the bin-ID list.
- `GET /bins` (paginated) on each side to resolve bin IDs to names.
- `GET /tickets?bin_id=<id>` for every bin on the source, collecting the set of ticket types and custom fields actually in use.
- `GET /ticket-types`, `GET /custom-fields`, `GET /users` on each side.

Matching is name-based for bins and types, `(name, type)` composite for custom fields, and email for users. Unused source types/fields are deliberately ignored — you don't need a destination equivalent for something no ticket references.

### ID mapping & idempotency

Every discovered or derived src→dst pair is stored in `Mapping` (`internal/mapping/mapping.go`): bins, ticket types, custom fields, users, user groups, tickets, comments, attachments. Load-modify-save per run. Before copying a ticket, `CopyTicket` checks `m.GetTicketDst(srcID)` — if non-empty and `--force` isn't set, the ticket is skipped with a "SKIPPED" log line.

### WAF fallbacks

Flowboards sits behind CloudFront with AWS WAF managed rules. A full ticket POST containing natural English can occasionally trip the SQL-injection rule set (keyword density) or path-traversal rule (substrings like `.../`). Copycards layers three retry strategies:

1. **Retry with backoff.** Transient 5xx / 429 / CloudFront 403 are retried with exponential backoff up to 6 attempts.
2. **Two-step create.** On sustained WAF block, create a minimal ticket (id, name, bin, type, order) then apply the remainder via `$partial` update. The src↔dst mapping is recorded as soon as the minimal POST succeeds so re-runs don't double-create.
3. **Sanitise & retry.** If the `$partial` is itself WAF-blocked, byte-level edits are applied to the description: zero-width spaces inserted into SQL-like keywords (`select`, `from`, `where`, `union`, `table`, `having`, …), and `.../` → `... /`. An audit note is appended. The specifics go to stdout; the note itself omits trigger keywords to avoid tripping the same rules.

### Retry & resumption

The HTTP client (`internal/fbclient/client.go`) handles `req.GetBody` body rewind between attempts, pagination via the `page-token` response header, and a `copycards/1.0` User-Agent (the default Go `Go-http-client/1.1` is often blocklisted at the edge).

## What's not supported

- **Attachments.** File copy is not implemented — the corresponding flag and stub functions were removed as of the last cleanup.
- **Comments.** Same — not implemented.
- **Parallel ticket copy.** Copies run serially. Parallelism would need goroutines + bounded concurrency + mapping-file write serialisation + error aggregation; it's deferred until there's a clear need.

## Options

- `--dry-run` — simulate a copy without making changes (creates no tickets, no mappings written).
- `--config <path>` — override the default config file path.

## Exit codes

- `0` — success.
- `1` — validation failure (configuration error, missing profile, board incompatibility).
- `2` — execution error (API failures, network, permission denied, quota).

## Troubleshooting

### `env var FB_KEY_SRC not set for org src`
Source your `.env` (or equivalent) before running copycards: `source .env`.

### `incompatible boards: bin "To Do" not found in destination`
The destination board has no bin named `To Do`. Add it in the Flowboards UI, or rename the source bin.

**"exists in dst org but not on this board"** — the bin *name* does exist somewhere in the destination org, just not on the board you picked. Either add it to the board or pick a different destination board.

### `ticket FEAT-1: assigned to unmapped user user_src_123`
The assigned user on the source doesn't have a matching email in the destination org. Either invite them in Flowboards, unassign them on the source ticket, or hand-edit the mapping — `~/.copycard/mapping.json` has a `users` block you can add to. Stop, edit, restart the copy.

### Persistent CloudFront 403s
Check `~/.copycard/failed-posts/` — each file pair shows the exact payload and error. Patterns that trip WAF commonly include long SQL-keyword runs and `.../` in URLs. Copycards' sanitiser handles these automatically as a last resort, but fundamentally unmutable content (name fields, obviously structured API keys in a description, …) requires manually editing the source ticket.

### `load mapping: invalid JSON`
The mapping file is corrupted. Inspect it at `~/.copycard/mapping.json`. If unrecoverable:
```bash
copycards mapping reset
copycards board verify --from src --to dst --src-board <id> --dst-board <id>
```

## Development

### Running tests

```bash
go test ./... -v
```

The fbclient resilience tests hit real retry-backoff paths and take ~30s — expect that on a full run. Individual packages:

```bash
go test ./tests/copier/... -v          # preflight, board, ticket, sanitise
go test ./tests/fbclient/... -v        # HTTP client + pagination + WAF
go test ./tests/cli/... -v             # integration + e2e flows
```

### Project layout

```
cmd/copycards/          # entrypoint: flag parsing, subcommand dispatch
internal/cli/           # command implementations (ListOrgs, CopyTickets, …)
internal/config/        # TOML loader + endpoint discovery
internal/copier/        # preflight, board/ticket copy, WAF sanitiser
internal/fbclient/      # HTTP client for Flowboards (retry, pagination, dumps)
internal/mapping/       # persistent src↔dst ID store
tests/                  # external-package tests mirroring internal/
docs/superpowers/       # specs and plans
specs/                  # PRD
```

### Contributing

Pull requests welcome. Please keep the test suite green (`go test ./...`) and match the existing commit-message style (Conventional Commits: `fix:`, `refactor:`, `feat:`, `test:`, `docs:`).

## Issues & support

Bugs and feature requests: <https://github.com/Germanicus1/copycards/issues>.

## License

Copycards is released under the [Hippocratic License 3.0 (Core)](https://firstdonoharm.dev/version/3/0/core.txt).

In short: you may use, modify, and distribute copycards freely — for personal, educational, commercial, or research purposes — provided your use does not violate the human rights standards enumerated by the Hippocratic License (rooted in the UN Universal Declaration of Human Rights and related international law). If your use complies with those standards, no further permission is required.

The canonical, legally-binding text lives at the link above. This summary is for orientation only; the license itself governs.

**Note:** The Hippocratic License is not an OSI-approved open-source license. See the ["Important caveat"](https://opensource.org/osd) on OSI criterion #6 if that matters to your use case.
