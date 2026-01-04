# 🛰️ Beeper CLI — Warp-speed access to your local Beeper universe

A read-only CLI for AI agents (and humans) to browse, open, and search your local Beeper chat history.

## Highlights
- Read-only access to your local Beeper SQLite database
- Thread listing, detail views, and message browsing
- Built-in full-text search using Beeper's FTS index (with LIKE fallback)
- Optional context windows around search matches
- Optional DM name resolution via platform bridge databases
- JSON output for easy agent integration

## Requirements
- Go 1.22+
- SQLite driver (CGO). On macOS this is typically already available.

## Install
```bash
# from the repo
cd /path/to/beeper-cli

go build ./cmd/beeper-cli
```

## Database Path
By default the CLI looks for:
- `~/Library/Application Support/BeeperTexts/index.db`
- `~/Library/Application Support/Beeper/index.db`

Override with:
- `--db /path/to/index.db`
- `BEEPER_DB=/path/to/index.db`

Disable bridge DB lookups with:
- `--no-bridge`

## Usage
```bash
beeper-cli --help

beeper-cli threads list --days 7 --limit 50
beeper-cli threads show --id "!abc123:beeper.local"

beeper-cli messages list --thread "!abc123:beeper.local" --limit 50

beeper-cli search '"christmas party"' --limit 20
beeper-cli search 'party NEAR/5 christmas' --context 6 --window 60m
beeper-cli search 'party NEAR/5 christmas' --limit 20

beeper-cli threads list --json
beeper-cli search 'invoice' --json
```

## Commands (v0.1.0)
- `threads list` — list conversations ordered by last activity
- `threads show` — show thread metadata and participants
- `messages list` — read recent messages in a thread
- `search` — full-text search across messages (FTS5)
- `db info` — show resolved database path and FTS availability
- `version` — print the current version

## Full-Text Search Notes
Beeper already ships an FTS5 index (`mx_room_messages_fts`) populated by triggers. The CLI uses that table directly, so no importer is required for keyword or phrase search. If the table doesn't exist, it falls back to a basic `LIKE` search on message text.

Examples:
- Phrase search: `"christmas party"`
- Proximity: `party NEAR/5 christmas`
- Prefix: `christ*`

## Vector/Semantic Search Options (future)
Vector search is not built-in to Beeper's SQLite schema, so it requires an additional index:

Option A — Local embeddings + SQLite table
- Build a small local embedding store (message_id -> vector)
- Use a SQLite extension (e.g., sqlite-vss) for cosine similarity
- Pros: stays local; simple deployments
- Cons: requires extra setup and embedding model

Option B — Local embeddings + separate index file
- Use an on-disk index (e.g., HNSW) stored alongside the DB
- Pros: fast and accurate
- Cons: more code and versioned index management

Option C — External vector store
- Push embeddings to a vector DB or hosted index
- Pros: scalable and fast
- Cons: not fully local, extra infrastructure

## Notes
- This tool is read-only and does not send messages.
- The underlying schema may change as Beeper evolves. If that happens, the queries may need updates.

## License
MIT
