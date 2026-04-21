# copycards — PRD (final v1.0)

Go CLI to copy Flowboards tickets between **identical boards** in different organizations that live on different endpoints with different API keys.

API reference: https://fb.mauvable.com/public/rest-help.html

## 1. Purpose

Repeatable, scriptable migration/duplication of Flowboards tickets across orgs. Replaces ad-hoc MCP-driven copies (slow, tokens, rate-limited).

## 2. Goals

- Copy all tickets of a board (checklists + comments + attachments + parent/child links) from source org/board → destination org/board.
- Copy a single ticket (and descendants) by ID.
- Dry-run mode (print plan, no writes).
- Idempotent re-runs (skip tickets already copied, detected via a mapping file).
- Multiple named org profiles in config; select source & destination per command.
- v1.0 assumes source & destination boards are **identical** (same bin names, exact match). v2.0+ may introduce a bin-mapping config for non-identical boards.

## 3. Non-goals

- Bi-directional sync.
- Live/continuous replication (webhooks).
- User account provisioning (invite-user).
- UI. CLI only.
- Preserving original `_id` values across orgs (destination gets new IDs).
- Creating bins or boards in destination (must pre-exist).
- Copying board-level metadata (color, size, user-group ACLs).

## 4. CLI surface

Binary: `copycards`

```
copycards <command> [flags]

Global flags:
  --config <path>        default: $XDG_CONFIG_HOME/copycards/config.toml
  --from <profile>       source org profile name
  --to   <profile>       destination org profile name
  --dry-run              print plan, no writes
  --verbose / -v
  --mapping <path>       ID mapping store (default: ./.copycard/<from>-<to>-<src-board-id>.json)
  --concurrency <n>      worker threads (default 4, max 1-500)
```

Commands:

```
copycards orgs list
    List configured org profiles (name + org_id + cached endpoint, if any).

copycards boards list --from <profile>
    Interactive: numbered menu of boards. User picks by number or enters board _id.
    Output: [1] Board Name [board-id-123]
            [2] Another Board [board-id-456]
            Pick a board (number or ID):

copycards board verify --from <src> --to <dst> \
  --src-board <id|choice> --dst-board <id|choice>
    Dry-run: check bin names (exact match), ticket-types, custom-fields match 1:1.
    Exit 0 if identical, exit 1 with diff if not.
    No writes.

copycards tickets copy --from <src> --to <dst> \
  --src-board <id|choice> --dst-board <id|choice> \
  [--dry-run] [--include-attachments] [--include-comments] \
  [--concurrency 4] [--verbose]
    Copy all tickets from src-board → dst-board.
    - Loads mapping file (create if missing).
    - Runs preflight (board verify).
    - Enumerates src tickets, topological sort, copy in order.
    - **FAIL on unmapped users or unknown fields/types** (fail-fast, no partial data).
    - Skip already-copied tickets (per mapping).
    - Write .copycard/<from>-<to>-<src-board-id>.json.

copycards ticket copy <ticket-id> --from <src> --to <dst> \
  --dst-board <id|choice> \
  [--with-children] [--include-attachments] [--include-comments] \
  [--dry-run]
    Copy single ticket (+ children if flag set).

copycards diff --from <src> --to <dst> \
  --src-board <id|choice> --dst-board <id|choice>
    Show which tickets exist in src but not in mapping.
    Useful before running `tickets copy`.

copycards mapping show [--from <src> --to <dst> --src-board <id>]
    Display current mapping file (or all, if no filters).

copycards mapping reset [--from <src> --to <dst> --src-board <id>]
    Delete mapping file(s). Prompt for confirmation.
```

## 5. Config file (TOML)

```toml
default_from = "msl"
default_to   = "demo"

[orgs.msl]
org_id   = "msl"
api_key  = "env:FB_KEY_MSL"           # or literal string
# endpoint auto-discovered via /rest-directory/2/<org_id>; override:
# endpoint = "https://s23.fb.mauvable.com/rest/2/msl"

[orgs.demo]
org_id  = "demo"
api_key = "env:FB_KEY_DEMO"
```

- `env:NAME` prefix → read from env var.
- Endpoint discovery cached in `~/.cache/copycards/directory.json` (TTL 24h).

## 6. Copy semantics

### 6.1 Entity resolution (boards assumed identical)

| Entity          | Match key              | Missing in dst → |
|-----------------|------------------------|------------------|
| board           | user-supplied id (interactive choice) | fail |
| bin             | **exact name match** (case-sensitive, whitespace-sensitive) | **fail** (board not identical) |
| ticket-type     | exact name match        | fail |
| custom-field    | exact name + type match | fail |
| user            | email                  | **fail if assigned/watched** (fail-fast, no partial data) |
| ticket          | source `_id`           | create (fresh dst `_id`) |

The preflight (`board verify`) runs these checks and prints a diff before any write.

Mapping file shape (per src-org/dst-org/src-board/dst-board):

```json
{
  "from": "msl",
  "to": "demo",
  "srcBoard": "<src_board_id>",
  "dstBoard": "<dst_board_id>",
  "users":         { "<src_id>": "<dst_id>" },
  "ticketTypes":   { "<src_id>": "<dst_id>" },
  "customFields":  { "<src_id>": "<dst_id>" },
  "bins":          { "<src_id>": "<dst_id>" },
  "tickets":       { "<src_id>": "<dst_id>" },
  "comments":      { "<src_id>": "<dst_id>" },
  "attachments":   { "<src_id>": "<dst_id>" }
}
```

### 6.2 Ticket copy algorithm

1. Resolve endpoints + auth for both orgs (discover + cache).
2. Load/seed mapping file from `.copycard/<from>-<to>-<src_board_id>.json`.
3. Preflight: `GET /boards/<src>` and `GET /boards/<dst>`; build bin-name map; **fail if any src bin name absent in dst** (exact match). Build ticket-type + custom-field maps. Build user map via `/users`.
4. Enumerate src tickets: for each src bin → `GET /tickets?bin_id=`.
5. Topologically sort by parent_id (roots first).
6. For each ticket (skip if mapping exists, unless `--force`):
   a. `GET /tickets/<id>` full object.
   b. Translate: `bin_id`, `ticketType_id`, `assigned_ids`, `watch_ids`, `customFields` keys, `enclosed_id`.
   c. **FAIL if any assigned/watched user unmapped** (fail-fast).
   d. Allocate new `_id` from dst `/ids`.
   e. `POST /tickets/<new_id>` with translated fields + checklists + standard fields.
   f. Record mapping.
7. Second pass: parent/child links via `PUT /tickets/addParent` (**within-board only; drop out-of-scope links silently**).
8. Attachments (unless `--skip-attachments`): for each src ticket → `GET /attachments/<id>` → `POST /attachments` on dst. **Skip by default; opt-in with `--include-attachments`**.
9. Comments (unless `--skip-comments`): `GET /ticket-comments?ticket_id=` → `POST /ticket-comments/<new_id>` on dst. **Skip by default; opt-in with `--include-comments`**. Prepend `[orig author: <name> @ <ISO-date-time>] ` to body (full timestamp for traceability).

### 6.3 Idempotency

- Every write checks mapping first; if src→dst mapping exists, skip (unless `--force`).
- On ticket create, 400 `{_id: [["already_exists"]]}` → treat as success, record mapping.
- Comments and attachments tracked by src-id → dst-id in mapping file to avoid duplicates on re-run.
- **Re-run policy:** load mapping, copy only new tickets, skip old ones. If a ticket's `assigned_ids` changed in src after initial copy, re-run will skip it (no sync/update of existing copies).

### 6.4 Concurrency & rate limits

- Default worker pool: 4 concurrent HTTP requests per org.
- Exponential backoff on 429/5xx (base 500ms, max 30s, 6 retries).
- `--concurrency N` flag (clamp to 1-500).

## 7. Field handling details

- `order`: preserved (float).
- `estimatedDuration`, `estimatedEffort`, `actualCost`: tuples copied verbatim.
- `plannedStartDate`, `dueDate`, etc.: ISO date strings copied verbatim.
- `checklists`: full tree copied; inner IDs regenerated (fresh, per spec, since IDs are ticket-scoped).
- `customFields`: map src field_id → dst field_id via preflight; values copied as-is (no option-label remap; assumes identical field definitions).
- `assigned_ids` / `watch_ids`: translated via user map (email). **Unmapped → fail ticket copy** (fail-fast).
- Archived tickets: skipped unless `--include-archived` (future flag; v1.0 skips all).
- Out-of-board parent links: **silently dropped** (within-board hierarchy only).

## 8. Logging & output

- Default: one line per ticket copied (`TICKET <src_id> → <dst_id> "<name>"`).
- `-v`: full request/response on failures.
- End-of-run summary: `Copied: N, Skipped: M, Failed: K`.
- Dry-run: `WOULD COPY: ticket <id> (<name>)` per ticket, no writes.
- Failures written to `./.copycard/errors-<timestamp>.jsonl` (one JSON per line, resumable by re-run).

## 9. Tech stack

- Go 1.25 (per go.mod).
- Stdlib `net/http` + `encoding/json`.
- `github.com/BurntSushi/toml` for config.
- CLI: stdlib `flag` (zero-dep, keep simple).
- No ORM, no codegen; hand-rolled typed structs.

Package layout:

```
cmd/copycards/main.go              # Entry point, flag setup, command dispatch
internal/
  config/
    config.go                       # TOML parsing, org profiles, env expansion
    discovery.go                    # Endpoint discovery + caching
  fbclient/
    client.go                       # HTTP wrapper, auth, retries, pagination
    types.go                        # Struct definitions for API responses
  mapping/
    mapping.go                      # Load/persist JSON mapping files
  copier/
    preflight.go                    # board verify logic
    ticket.go                       # Single ticket copy orchestration
    board.go                        # Full board copy + topological sort
  cli/
    boards.go                       # boards list, board verify commands
    tickets.go                      # tickets copy, ticket copy commands
    mapping.go                      # mapping show, mapping reset commands
    orgs.go                         # orgs list command
tests/
  config/config_test.go
  fbclient/client_test.go
  mapping/mapping_test.go
  copier/preflight_test.go, ticket_test.go, board_test.go
  cli/integration_test.go
```

## 10. Out of scope (v1.0)

- Webhooks, flows, board-level metadata.
- Bin/board creation in dst.
- Non-identical boards (bin mapping config arrives in v2.0).
- Partial-update / sync of already-copied tickets (force-recopy only).
- Archived ticket copying.
- Custom-field option-label remap (assumes identical definitions).
- Resume command (rely on mapping file + re-run idempotency).

## 11. Milestones

1. ✅ Module init, types, config, endpoint discovery.
2. ✅ REST client (retries, pagination, concurrency).
3. ✅ Mapping file persistence.
4. ✅ Board preflight (bin, type, field resolution).
5. ✅ Ticket field translation (unmapped-user fail-fast).
6. ✅ Full board copy + topological sort + parent/child.
7. 📝 CLI commands (interactive board selection, all 6 commands).
8. 📝 Integration test (mocked API end-to-end).
9. 📝 Polish, README, final testing.

## 12. Design decisions finalized (from brainstorming)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Q1: Board selection** | Interactive menu (C) | User-friendly discovery; no manual ID lookup. |
| **Q2: Unmapped users** | Fail-fast (A) | Data integrity > convenience. No partial data. |
| **Q3: Idempotent re-runs** | Automatic (A) | Load mapping, copy only new tickets, skip old. |
| **Q4: Bin name match** | Exact (A) | Explicit, no hidden surprises from fuzzy matching. |
| **Q5: Field/type name match** | Exact (A) | No mapping config; user aligns names first. |
| **Q6: Attachments** | Opt-in, buffer in memory (D+B) | Default safe; simple buffer OK for demo boards. |
| **Q7: Comments** | Opt-in, full timestamp (D+A) | Default safe; traceability with `[orig author: … @ …]`. |
| **Q8: Cross-board parent links** | Drop silently (B) | Focus on within-board hierarchy. |
| **Q9: Error resumability** | Mapping file (A) | Idempotent re-run; no explicit resume command. |
| **Q10: Module naming** | `copycards` (plural) | Consistent everywhere (dir, binary, module). |

## 13. Success criteria

- v1.0 ships with: config + endpoint discovery, preflight validation, full board copy (tickets + checklists + parent/child links), idempotent re-runs, fail-fast on unmapped data.
- Opt-in attachments & comments (skipped by default).
- Interactive board selection via numbered menu.
- Dry-run mode works end-to-end (no writes).
- Mapping file persists & resumes on re-run (skip already-copied).
- Exit codes: 0 = success, 1 = validation fail, 2 = execution error.
