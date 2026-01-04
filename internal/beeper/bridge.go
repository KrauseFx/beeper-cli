package beeper

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BridgeLookup resolves DM names via platform bridge databases.
type BridgeLookup struct {
	platformDBs map[string]string
	cache       map[string]string
}

// NewBridgeLookup discovers megabridge.db files under the Beeper support directory.
func NewBridgeLookup(indexDBPath string, overrideRoot string) (*BridgeLookup, error) {
	root := overrideRoot
	if root == "" {
		root = filepath.Dir(indexDBPath)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	platformDBs := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "local-") {
			continue
		}
		path := filepath.Join(root, name, "megabridge.db")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		platformDBs[normalizePlatform(name)] = path
	}

	return &BridgeLookup{
		platformDBs: platformDBs,
		cache:       map[string]string{},
	}, nil
}

// LookupDMName attempts to resolve a DM name for the given room ID.
func (b *BridgeLookup) LookupDMName(ctx context.Context, roomID string, accountID string) (string, bool, error) {
	if b == nil || len(b.platformDBs) == 0 {
		return "", false, nil
	}
	if cached, ok := b.cache[roomID]; ok {
		if cached == "" {
			return "", false, nil
		}
		return cached, true, nil
	}

	candidate := ""
	if accountID != "" {
		candidate = b.platformDBs[normalizePlatform(accountID)]
	}

	if candidate != "" {
		name, ok, err := queryBridgeName(ctx, candidate, roomID)
		if err != nil {
			return "", false, err
		}
		b.cache[roomID] = name
		return name, ok, nil
	}

	for _, path := range b.platformDBs {
		name, ok, err := queryBridgeName(ctx, path, roomID)
		if err != nil {
			return "", false, err
		}
		if ok {
			b.cache[roomID] = name
			return name, true, nil
		}
	}

	b.cache[roomID] = ""
	return "", false, nil
}

func queryBridgeName(ctx context.Context, dbPath string, roomID string) (string, bool, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", dbPath)
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return "", false, err
	}
	defer func() {
		_ = conn.Close()
	}()
	conn.SetMaxOpenConns(1)

	var otherUser string
	row := conn.QueryRowContext(ctx, "SELECT other_user_id FROM portal WHERE mxid = ? AND other_user_id IS NOT NULL LIMIT 1", roomID)
	if err := row.Scan(&otherUser); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}

	var name string
	row = conn.QueryRowContext(ctx, "SELECT name FROM ghost WHERE id = ? AND name != '' LIMIT 1", otherUser)
	if err := row.Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return "", false, nil
	}
	return name, true, nil
}

func normalizePlatform(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	platform = strings.TrimPrefix(platform, "local-")
	return platform
}

// Paths returns bridge database paths discovered for the current user.
func (b *BridgeLookup) Paths() []string {
	if b == nil {
		return nil
	}
	paths := make([]string, 0, len(b.platformDBs))
	for _, path := range b.platformDBs {
		paths = append(paths, path)
	}
	return paths
}
