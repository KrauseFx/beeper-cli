package beeper

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestListThreadsLabels(t *testing.T) {
	path := createTestDB(t, false)
	store, err := OpenWithOptions(path, StoreOptions{BridgeLookup: false})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	threads, err := store.ListThreads(ctx, ThreadListOptions{Label: LabelInbox})
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	if len(threads) != 2 || threads[0].ID != "!room1:beeper.local" || threads[1].ID != "!room4:beeper.local" {
		t.Fatalf("expected inbox threads room1+room4, got %+v", ids(threads))
	}

	threads, err = store.ListThreads(ctx, ThreadListOptions{Label: LabelArchive})
	if err != nil {
		t.Fatalf("list archive: %v", err)
	}
	if len(threads) != 1 || threads[0].ID != "!room2:beeper.local" {
		t.Fatalf("expected archive thread2 only, got %+v", ids(threads))
	}

	threads, err = store.ListThreads(ctx, ThreadListOptions{Label: LabelFavourite, IncludeLowPriority: true})
	if err != nil {
		t.Fatalf("list favourite: %v", err)
	}
	if len(threads) != 1 || threads[0].ID != "!room3:beeper.local" {
		t.Fatalf("expected favourite thread3 only, got %+v", ids(threads))
	}

	threads, err = store.ListThreads(ctx, ThreadListOptions{Label: LabelUnread})
	if err != nil {
		t.Fatalf("list unread: %v", err)
	}
	if len(threads) != 1 || threads[0].ID != "!room1:beeper.local" {
		t.Fatalf("expected unread thread1 only, got %+v", ids(threads))
	}
}

func TestSearchWithContext(t *testing.T) {
	path := createTestDB(t, true)
	store, err := OpenWithOptions(path, StoreOptions{BridgeLookup: false})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	results, err := store.SearchMessages(ctx, SearchOptions{
		Query:   "christmas",
		Limit:   5,
		Context: 1,
		Window:  time.Hour,
		Format:  FormatPlain,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Match.Text == "" {
		t.Fatalf("expected match text")
	}
	if len(results[0].Context) != 2 {
		t.Fatalf("expected 2 context messages, got %d", len(results[0].Context))
	}
}

func TestSearchFallbackLike(t *testing.T) {
	path := createTestDB(t, false)
	store, err := OpenWithOptions(path, StoreOptions{BridgeLookup: false})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	results, err := store.SearchMessages(ctx, SearchOptions{Query: "invoice"})
	if err != nil {
		t.Fatalf("search fallback: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestBridgeLookupDMName(t *testing.T) {
	path := createTestDB(t, false)
	bridgeRoot := createBridgeDB(t)

	store, err := OpenWithOptions(path, StoreOptions{BridgeLookup: true, BridgeRoot: bridgeRoot})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	thread, err := store.GetThread(ctx, "!room4:beeper.local", false)
	if err != nil {
		t.Fatalf("get thread: %v", err)
	}
	if thread.DisplayName != "Bridge Name" {
		t.Fatalf("expected bridge name, got %q", thread.DisplayName)
	}
}

func createTestDB(t *testing.T, withFTS bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "index.db")
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	statements := []string{
		`CREATE TABLE threads (threadID TEXT PRIMARY KEY, accountID TEXT, thread JSON NOT NULL, timestamp INTEGER DEFAULT 0);`,
		`CREATE TABLE breadcrumbs (id TEXT PRIMARY KEY, lastOpenTime INTEGER);`,
		`CREATE TABLE participants (account_id TEXT NOT NULL, room_id TEXT NOT NULL, id TEXT NOT NULL, full_name TEXT, nickname TEXT, is_self INTEGER);`,
		`CREATE TABLE mx_room_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			roomID TEXT NOT NULL,
			eventID TEXT NOT NULL,
			senderContactID TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			isDeleted INTEGER NOT NULL DEFAULT 0,
			type TEXT NOT NULL,
			hsOrder INTEGER NOT NULL,
			isSentByMe INTEGER NOT NULL,
			message JSON,
			text_content TEXT
		);`,
	}

	for _, stmt := range statements {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}

	if withFTS {
		if _, err := conn.Exec(`CREATE VIRTUAL TABLE mx_room_messages_fts USING fts5(text_content);`); err != nil {
			t.Skipf("fts5 not available: %v", err)
		}
	}

	threads := []struct {
		id        string
		accountID string
		thread    string
		ts        int64
	}{
		{"!room1:beeper.local", "whatsapp", `{"title":"Team Chat","type":"group","isUnread":1,"isMarkedUnread":0,"isLowPriority":0,"unreadCount":2,"unreadMentionsCount":1}`, 1700000000000},
		{"!room2:beeper.local", "telegram", `{"title":"Archived","type":"group","isUnread":0,"isMarkedUnread":0,"isLowPriority":0,"extra":{"isArchivedUpto":5}}`, 1700000001000},
		{"!room3:beeper.local", "signal", `{"title":"Fav","type":"group","isUnread":0,"isMarkedUnread":0,"isLowPriority":1,"extra":{"isArchivedUpto":5,"tags":["favourite"]}}`, 1700000002000},
		{"!room4:beeper.local", "whatsapp", `{"type":"single"}`, 1700000003000},
	}

	for _, row := range threads {
		if _, err := conn.Exec("INSERT INTO threads (threadID, accountID, thread, timestamp) VALUES (?, ?, ?, ?)", row.id, row.accountID, row.thread, row.ts); err != nil {
			t.Fatalf("insert thread: %v", err)
		}
	}

	if _, err := conn.Exec("INSERT INTO breadcrumbs (id, lastOpenTime) VALUES (?, ?)", "!room1:beeper.local", 1700000000500); err != nil {
		t.Fatalf("insert breadcrumbs: %v", err)
	}

	if _, err := conn.Exec("INSERT INTO participants (account_id, room_id, id, full_name, nickname, is_self) VALUES (?, ?, ?, ?, ?, ?)", "whatsapp", "!room1:beeper.local", "@alice:beeper.local", "Alice", "", 0); err != nil {
		t.Fatalf("insert participant: %v", err)
	}

	messages := []struct {
		id      int
		roomID  string
		eventID string
		sender  string
		ts      int64
		typeVal string
		hsOrder int
		isMe    int
		message string
		text    string
	}{
		{1, "!room1:beeper.local", "$evt1", "@alice:beeper.local", 1700000000100, "TEXT", 6, 0, `{"text":"hello"}`, "hello"},
		{2, "!room1:beeper.local", "$evt2", "@alice:beeper.local", 1700000000200, "TEXT", 7, 0, `{"text":"christmas party"}`, "christmas party"},
		{3, "!room1:beeper.local", "$evt3", "@alice:beeper.local", 1700000000300, "TEXT", 8, 0, `{"text":"see you"}`, "see you"},
		{4, "!room2:beeper.local", "$evt4", "@bob:beeper.local", 1700000000400, "TEXT", 5, 0, `{"text":"archived"}`, "archived"},
		{5, "!room3:beeper.local", "$evt5", "@eve:beeper.local", 1700000000500, "TEXT", 5, 0, `{"text":"fav"}`, "fav"},
		{6, "!room4:beeper.local", "$evt6", "@bridge:beeper.local", 1700000000600, "TEXT", 1, 0, `{"text":"dm"}`, "dm"},
		{7, "!room1:beeper.local", "$evt7", "@alice:beeper.local", 1700000000700, "TEXT", 9, 0, `{"text":"invoice due"}`, "invoice due"},
	}

	for _, msg := range messages {
		_, err := conn.Exec(
			"INSERT INTO mx_room_messages (id, roomID, eventID, senderContactID, timestamp, isDeleted, type, hsOrder, isSentByMe, message, text_content) VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?)",
			msg.id, msg.roomID, msg.eventID, msg.sender, msg.ts, msg.typeVal, msg.hsOrder, msg.isMe, msg.message, msg.text,
		)
		if err != nil {
			t.Fatalf("insert message: %v", err)
		}
		if withFTS {
			if _, err := conn.Exec("INSERT INTO mx_room_messages_fts (rowid, text_content) VALUES (?, ?)", msg.id, msg.text); err != nil {
				t.Fatalf("insert fts: %v", err)
			}
		}
	}

	return path
}

func createBridgeDB(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bridgeDir := filepath.Join(root, "local-whatsapp")
	if err := os.MkdirAll(bridgeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(bridgeDir, "megabridge.db")
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open bridge: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	if _, err := conn.Exec(`CREATE TABLE portal (mxid TEXT, other_user_id TEXT);`); err != nil {
		t.Fatalf("create portal: %v", err)
	}
	if _, err := conn.Exec(`CREATE TABLE ghost (id TEXT, name TEXT);`); err != nil {
		t.Fatalf("create ghost: %v", err)
	}
	if _, err := conn.Exec("INSERT INTO portal (mxid, other_user_id) VALUES (?, ?)", "!room4:beeper.local", "user-1"); err != nil {
		t.Fatalf("insert portal: %v", err)
	}
	if _, err := conn.Exec("INSERT INTO ghost (id, name) VALUES (?, ?)", "user-1", "Bridge Name"); err != nil {
		t.Fatalf("insert ghost: %v", err)
	}

	return root
}

func ids(threads []Thread) []string {
	list := make([]string, 0, len(threads))
	for _, thread := range threads {
		list = append(list, thread.ID)
	}
	return list
}
