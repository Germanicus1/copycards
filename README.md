# copycards

Copycards is a command-line tool for copying Flowboards tickets between organizations. It handles field translation, user mapping, nested ticket hierarchies, and supports dry-run mode for verification before executing copies.

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/you/copycards
   cd copycards
   ```

2. Build the binary:
   ```bash
   go build -o copycards ./cmd/copycards
   ```

3. Add to your PATH:
   ```bash
   sudo mv copycards /usr/local/bin/
   # Or add the current directory to PATH
   export PATH=$PATH:$(pwd)
   ```

## Configuration

### Config File Location

Copycards looks for configuration files in this order:
1. `~/.copycards/config.toml` (user home directory)
2. `./.copycards/config.toml` (current directory)
3. Path specified via `--config` flag

### Configuration Format

Create a TOML file with organization profiles:

```toml
default_from = "src"
default_to = "dst"

[orgs.src]
org_id = "org_src_id"
api_key = "env:FB_KEY_SRC"  # References environment variable FB_KEY_SRC
endpoint = "https://api.flowboards.com/v1"  # Optional, uses default if omitted

[orgs.dst]
org_id = "org_dst_id"
api_key = "env:FB_KEY_DST"  # References environment variable FB_KEY_DST
```

### Environment Variable Expansion

API keys can reference environment variables using the `env:` prefix. This allows you to keep sensitive credentials out of configuration files:

```bash
export FB_KEY_SRC="your_source_api_key"
export FB_KEY_DST="your_destination_api_key"
```

Copycards will automatically expand these variables when loading the configuration.

## State Directory (`.copycard/`)

Copycards persists run state in a `.copycard/` directory. It's created on demand — you don't need to make it yourself, and it's listed in `.gitignore` so its contents stay out of version control.

Two locations are used:

- **`./.copycard/mapping.json`** (in the current working directory) — written after each non-dry-run copy. Stores src→dst ID translations for users, bins, ticket types, custom fields, and tickets. Re-runs consult this file and skip tickets that are already copied, which is what makes the `tickets copy` command idempotent.
- **`~/.copycard/mapping.json`** (in `$HOME`) — what `mapping show` and `mapping reset` read. If you ran the copy from a directory other than `$HOME`, the two paths will diverge; re-run those commands from the same cwd you copied from, or move the file into `$HOME/.copycard/`.
- **`~/.copycard/failed-posts/`** — forensic dump directory. Any POST or PUT that exhausts retries writes two files here: `<timestamp>-<METHOD>-<id>.json` (the full request body that was rejected) and `<timestamp>-<METHOD>-<id>.error.txt` (the terminal error, e.g. `CloudFront blocked request: max retries exceeded`). Nothing is dumped on successful runs. Best-effort — dump I/O errors are silently swallowed so they can never mask the original failure.

Delete either directory freely: mapping loss means the next copy re-creates duplicates of anything not already in the destination, and failed-post dumps are purely diagnostic.

## Usage

### Commands

#### orgs list
List all configured organization profiles.
```bash
copycards orgs list
```

#### orgs verify
Verify connectivity to an organization.
```bash
copycards orgs verify <profile>
```

#### boards list
List all boards in an organization.
```bash
copycards boards list --from <profile>
```

#### board verify
Verify compatibility between source and destination boards. Generates field, bin, and user mappings.
```bash
copycards board verify --from <src> --to <dst> --src-board <id> --dst-board <id>
```

#### tickets copy
Copy all tickets from source board to destination board.
```bash
copycards tickets copy \
  --from <src> --to <dst> \
  --src-board <id> --dst-board <id> \
  [--dry-run] \
  [--include-attachments] \
  [--include-comments] \
  [--concurrency N]
```

#### ticket copy
Copy a single ticket with optional children.
```bash
copycards ticket copy <id> \
  --from <src> --to <dst> \
  --dst-board <id> \
  [--with-children] \
  [--include-attachments] \
  [--include-comments] \
  [--dry-run]
```

#### diff
Show tickets in source board not yet copied to destination.
```bash
copycards diff --from <src> --to <dst> --src-board <id> --dst-board <id>
```

#### mapping show
Display current field and user mappings.
```bash
copycards mapping show [--from <src> --to <dst> --src-board <id>]
```

#### mapping reset
Clear stored mappings to force re-verification.
```bash
copycards mapping reset [--from <src> --to <dst> --src-board <id>]
```

## Examples

### Basic Workflow: List Organizations

```bash
$ copycards orgs list
Available organizations:
  src (org_src_id)
  dst (org_dst_id)
```

### List Boards in Source Organization

```bash
$ copycards boards list --from src
Boards in organization "src":

[1] Main Board [board_main_id]
[2] Secondary [board_secondary_id]
```

### Verify Board Compatibility

Before copying, verify that the destination board can accept tickets from the source:

```bash
$ copycards board verify --from src --to dst --src-board board_main_id --dst-board board_dst_id
Boards are compatible

Mappings:

  Bins:
    "To Do" -> "Backlog"
    "In Progress" -> "Active"
    "Done" -> "Completed"

  Ticket Types:
    "Feature" -> "Enhancement"
    "Bug" -> "Defect"

  Users:
    user_src_1 -> user_dst_1
    user_src_2 -> user_dst_2
```

### Dry-Run: Preview What Would Be Copied

Always run with `--dry-run` first to verify the operation:

```bash
$ copycards tickets copy \
    --from src --to dst \
    --src-board board_main_id --dst-board board_dst_id \
    --dry-run
WOULD COPY: ticket FEAT-1 (Implement auth)
WOULD COPY: ticket FEAT-2 (Add logging)
WOULD COPY: ticket BUG-3 (Fix null pointer)
Copy summary: 0 copied, 0 skipped, 0 failed
```

### Real Copy: Execute the Operation

```bash
$ copycards tickets copy \
    --from src --to dst \
    --src-board board_main_id --dst-board board_dst_id
TICKET FEAT-1 → FEAT-1-copy (Implement auth)
TICKET FEAT-2 → FEAT-2-copy (Add logging)
TICKET BUG-3 → BUG-3-copy (Fix null pointer)
Copy summary: 3 copied, 0 skipped, 0 failed
```

### Copy Single Ticket with Children

```bash
$ copycards ticket copy EPIC-1 \
    --from src --to dst \
    --dst-board board_dst_id \
    --with-children
TICKET EPIC-1 → EPIC-1-copy (Q4 Roadmap)
TICKET STORY-1 → STORY-1-copy (Feature A)
TICKET STORY-2 → STORY-2-copy (Feature B)
Copy summary: 3 copied, 0 skipped, 0 failed
```

### Check Progress

See which tickets are still pending copy:

```bash
$ copycards diff --from src --to dst --src-board board_main_id --dst-board board_dst_id
Tickets in src not yet copied to dst:

  FEAT-5 - Add search
  BUG-10 - Handle edge case

Total: 2 ticket(s) remaining
```

### Include Attachments and Comments

By default, attachments and comments are skipped. To include them:

```bash
$ copycards tickets copy \
    --from src --to dst \
    --src-board board_main_id --dst-board board_dst_id \
    --include-attachments \
    --include-comments
```

### Control Concurrency

Limit API call concurrency (default: 4):

```bash
$ copycards tickets copy \
    --from src --to dst \
    --src-board board_main_id --dst-board board_dst_id \
    --concurrency 2
```

## Options

- `--dry-run`: Simulate copy without making changes (creates no tickets, no mappings written)
- `--include-attachments`: Copy file attachments from source tickets
- `--include-comments`: Copy comments from source tickets
- `--concurrency N`: Control number of concurrent API requests (default: 4)
- `--verbose`: Enable verbose logging

## Exit Codes

- `0`: Success - operation completed without errors
- `1`: Validation failure - configuration error, missing profile, board incompatibility, unmapped fields
- `2`: Execution error - API errors, network issues, permission denied, quota exceeded

## Troubleshooting

### Missing Environment Variables

**Error:** `env var FB_KEY_SRC not set for org src`

**Solution:** Set the required environment variables:
```bash
export FB_KEY_SRC="your_api_key"
export FB_KEY_DST="your_api_key"
```

### Unmapped Users

**Error:** `ticket FEAT-1: assigned to unmapped user user_src_123`

**Solution:** Users must exist in both organizations and their IDs must match. If users have different IDs:
1. Manually create a mapping by checking `copycards mapping show`
2. Edit the mapping file at `.copycard/mapping.json` (see [State Directory](#state-directory-copycard))
3. Add missing user mappings

### Bin Name Mismatch

**Error:** `incompatible boards: bin "To Do" not found in destination`

**Solution:** The source and destination boards must have compatible bin names. You can:
1. Add the missing bin to the destination board
2. Or use board verify to see the mapping and adjust destination bins to match source

### Field Type Mismatch

**Error:** `incompatible boards: field "priority" type mismatch`

**Solution:** The source and destination boards have fields with the same name but different types. Either:
1. Rename the field in one board
2. Or manually map the field using a different approach

### Network Timeouts

**Error:** `request failed: context deadline exceeded`

**Solution:** The API is slow or network is unstable. Try:
1. Reduce concurrency: `--concurrency 2`
2. Increase timeout (if available): Check environment or configuration
3. Run during off-peak hours
4. Break into smaller copy operations

### Quota Exceeded

**Error:** `request failed: 429 Too Many Requests`

**Solution:** You've exceeded API rate limits. Try:
1. Wait a few minutes before retrying
2. Reduce concurrency: `--concurrency 2`
3. Copy in smaller batches

### Mapping File Issues

**Error:** `load mapping: invalid JSON`

**Solution:** Delete the corrupted mapping file and start over:
```bash
copycards mapping reset --from src --to dst --src-board board_id
copycards board verify --from src --to dst --src-board board_id --dst-board board_id
```

## Contributing

Contributions are welcome. Please ensure all tests pass:

```bash
go test ./... -v
```

## License

[Add your license here]
