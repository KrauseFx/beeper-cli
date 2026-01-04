# Beeper CLI Spec (v0.1.0)

## Overview
Beeper CLI is a **read-only** command-line interface for AI agents and humans to explore local Beeper Desktop data stored in SQLite. It provides thread browsing, message reading, and full-text search backed by Beeper's FTS5 index. The CLI prioritizes agent-friendly JSON output and predictable filtering.

### Design Goals
- Read-only by default
- Fast and deterministic queries
- JSON output for agents
- Optional enrichment from platform bridge databases
- Built-in full-text search (FTS5)

## Database Sources
Primary DB (required):
- `index.db` (Beeper UI-optimized message + thread index)

Secondary DBs (optional, for better names):
- `local-*/megabridge.db` (per-platform bridge stores with contact names)

## Global Flags
- `--db <path>`: override `index.db` path
- `--json`: JSON output
- `--no-bridge`: disable megabridge lookups
- `--version`: print version
- `--help`: show help for any command

## Commands

### `db`
Diagnostics and discovery.

#### `db info`
Show resolved database path and FTS availability.

**Output fields**
- `path` (string)
- `hasFts` (bool)
- `readOnly` (bool)
- `bridgeDbs` (array, when JSON)

---

### `threads`
Conversation browsing.

#### `threads list`
List threads ordered by last activity.

**Flags**
- `--limit <n>` (default: 50)
- `--days <n>` (last activity within N days)
- `--label inbox|archive|favourite|unread|all` (default: all)
- `--include-low-priority` (include low-priority threads)
- `--account <id>` (platform ID, e.g. `whatsapp`, `telegram`)
- `--with-participants` (include participant list in JSON)
- `--with-stats` (include total message counts)

**Notes**
- Inbox/archive logic uses `threads.thread` JSON (`isLowPriority`, `extra.isArchivedUpto`, `extra.tags`) and message `hsOrder`.
- Display names are resolved in priority order:
  1. `thread.title`
  2. `thread.name`
  3. `megabridge.db` (portal/ghost) for DMs (optional)
  4. `participants` names

#### `threads show`
Show one thread with metadata and participants.

**Flags**
- `--id <thread-id>`
- `--with-stats` (include total messages and last message time)
- `--with-last <n>` (inline last N messages)
- `--format plain|rich` (default: rich)

---

### `messages`
Read message history.

#### `messages list`
List messages for a thread.

**Flags**
- `--thread <thread-id>`
- `--limit <n>` (default: 50)
- `--days <n>` (last N days)
- `--before <ISO8601>`
- `--after <ISO8601>`
- `--format plain|rich` (default: rich)

**Format**
- `plain`: uses `text_content` or `$.text`
- `rich`: decodes media/file/location/contact into readable placeholders

---

### `search`
Full-text search across messages.

**Flags**
- `--limit <n>` (default: 50)
- `--days <n>`
- `--thread <thread-id>`
- `--account <platform>`
- `--context <n>` (messages before/after match)
- `--window <duration>` (time window for context; default 1h when context set)
- `--format plain|rich` (default: rich)

**Behavior**
- Uses `mx_room_messages_fts` with `MATCH` for keyword/phrase/proximity queries.
- If FTS is missing, falls back to `LIKE` on `$.text`.
- When context is requested, return a `match` + surrounding messages.

---

### `version`
Print the CLI version.

---

## Output Models
### Thread
```
{
  "id": "!abc:beeper.local",
  "accountId": "whatsapp",
  "title": "Team Chat",
  "name": "",
  "type": "group",
  "displayName": "Team Chat",
  "lastActivity": "2025-12-19T16:37:05+01:00",
  "lastMessageTime": "2025-12-19T16:37:05+01:00",
  "lastOpenTime": "2025-12-19T15:00:00+01:00",
  "isUnread": true,
  "isMarkedUnread": false,
  "isLowPriority": false,
  "isArchived": false,
  "unreadCount": 2,
  "unreadMentions": 0,
  "totalMessages": 120,
  "tags": ["favourite"],
  "participants": [
    {"id":"@user:beeper.local", "name":"Alice", "isSelf":false}
  ]
}
```

### Message
```
{
  "id": 123,
  "eventId": "$abc",
  "threadId": "!abc:beeper.local",
  "threadName": "Team Chat",
  "accountId": "whatsapp",
  "senderId": "@alice:beeper.local",
  "senderName": "Alice",
  "timestamp": "2025-12-19T16:37:05+01:00",
  "isSentByMe": false,
  "type": "TEXT",
  "text": "See you at the christmas party"
}
```

### SearchResult
```
{
  "match": { ...Message... },
  "context": [ ...Message... ]
}
```

## Vector Search Roadmap
Vector/semantic search is not built into Beeper's SQLite schema. Options:

1) **Local embeddings + sqlite-vss**
   - Create a sidecar DB with message embeddings
   - Use cosine similarity queries in SQLite

2) **Local embeddings + HNSW index**
   - Store a vector index on disk alongside metadata
   - Very fast; more code

3) **External vector store**
   - Scales well but not fully local

Recommended initial approach: **local sidecar DB** with on-device embeddings.
