package beeper

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // sqlite3 driver
)

// Store provides read-only access to Beeper's SQLite database.
type Store struct {
	db     *sql.DB
	bridge *BridgeLookup
}

// Open opens a read-only store with bridge lookups enabled.
func Open(path string) (*Store, error) {
	return OpenWithOptions(path, StoreOptions{BridgeLookup: true})
}

// OpenWithOptions opens a read-only store with the provided options.
func OpenWithOptions(path string, opts StoreOptions) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, err
	}

	var bridge *BridgeLookup
	if opts.BridgeLookup {
		if b, err := NewBridgeLookup(path, opts.BridgeRoot); err == nil {
			bridge = b
		}
	}

	return &Store{db: db, bridge: bridge}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// BridgeDBs returns discovered platform bridge database paths.
func (s *Store) BridgeDBs() []string {
	if s == nil || s.bridge == nil {
		return nil
	}
	return s.bridge.Paths()
}

// HasFTS reports whether the FTS table exists.
func (s *Store) HasFTS(ctx context.Context) (bool, error) {
	row := s.db.QueryRowContext(ctx, "SELECT 1 FROM sqlite_master WHERE type='table' AND name='mx_room_messages_fts'")
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListThreads returns threads filtered by the provided options.
func (s *Store) ListThreads(ctx context.Context, opts ThreadListOptions) ([]Thread, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	label := opts.Label
	if label == "" {
		label = LabelAll
	}

	query := strings.Builder{}
	query.WriteString(`SELECT t.threadID, t.accountID, t.timestamp,
		json_extract(t.thread,'$.title') AS title,
		json_extract(t.thread,'$.name') AS name,
		json_extract(t.thread,'$.type') AS type,
		json_extract(t.thread,'$.isUnread') AS isUnread,
		json_extract(t.thread,'$.isMarkedUnread') AS isMarkedUnread,
		json_extract(t.thread,'$.isLowPriority') AS isLowPriority,
		json_extract(t.thread,'$.unreadCount') AS unreadCount,
		json_extract(t.thread,'$.unreadMentionsCount') AS unreadMentionsCount,
		json_extract(t.thread,'$.extra.isArchivedUpto') AS isArchivedUpto,
		json_extract(t.thread,'$.extra.isArchivedUpToOrder') AS isArchivedUpToOrder,
		json_extract(t.thread,'$.extra.tags') AS tags,
		b.lastOpenTime AS lastOpenTime,
		(SELECT MAX(timestamp) FROM mx_room_messages WHERE roomID = t.threadID AND type NOT IN ('HIDDEN','REACTION')) AS lastMessageTime,
		(SELECT MAX(hsOrder) FROM mx_room_messages WHERE roomID = t.threadID AND type != 'HIDDEN') AS latestHsOrder,
		(SELECT COUNT(*) FROM mx_room_messages WHERE roomID = t.threadID AND type NOT IN ('HIDDEN','REACTION')) AS totalMessages
		FROM threads t
		LEFT JOIN breadcrumbs b ON t.threadID = b.id`)

	conds := []string{}
	args := []any{}

	if opts.AccountID != "" {
		conds = append(conds, "t.accountID = ?")
		args = append(args, opts.AccountID)
	}

	if opts.Days > 0 {
		cutoff := time.Now().AddDate(0, 0, -opts.Days).UnixMilli()
		conds = append(conds, "t.timestamp >= ?")
		args = append(args, cutoff)
	}

	if len(conds) > 0 {
		query.WriteString(" WHERE ")
		query.WriteString(strings.Join(conds, " AND "))
	}

	query.WriteString(" ORDER BY COALESCE(lastMessageTime, lastOpenTime, t.timestamp) DESC LIMIT ?")
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	threads := []Thread{}
	threadIDs := []string{}

	for rows.Next() {
		var thread Thread
		var accountID sql.NullString
		var title sql.NullString
		var name sql.NullString
		var threadType sql.NullString
		var isUnread sql.NullInt64
		var isMarkedUnread sql.NullInt64
		var isLowPriority sql.NullInt64
		var unreadCount sql.NullInt64
		var unreadMentions sql.NullInt64
		var archivedUpto sql.NullString
		var archivedUpToOrder sql.NullString
		var tagsRaw sql.NullString
		var lastOpen sql.NullInt64
		var lastMessage sql.NullInt64
		var latestHsOrder sql.NullInt64
		var totalMessages sql.NullInt64
		var ts int64

		if err := rows.Scan(
			&thread.ID,
			&accountID,
			&ts,
			&title,
			&name,
			&threadType,
			&isUnread,
			&isMarkedUnread,
			&isLowPriority,
			&unreadCount,
			&unreadMentions,
			&archivedUpto,
			&archivedUpToOrder,
			&tagsRaw,
			&lastOpen,
			&lastMessage,
			&latestHsOrder,
			&totalMessages,
		); err != nil {
			return nil, err
		}

		thread.AccountID = accountID.String
		thread.Title = strings.TrimSpace(title.String)
		thread.Name = strings.TrimSpace(name.String)
		thread.Type = strings.TrimSpace(threadType.String)
		thread.IsUnread = isUnread.Valid && isUnread.Int64 != 0
		thread.IsMarkedUnread = isMarkedUnread.Valid && isMarkedUnread.Int64 != 0
		thread.IsLowPriority = isLowPriority.Valid && isLowPriority.Int64 != 0
		if unreadCount.Valid {
			thread.UnreadCount = int(unreadCount.Int64)
		}
		if unreadMentions.Valid {
			thread.UnreadMentions = int(unreadMentions.Int64)
		}
		thread.Tags = parseTags(tagsRaw.String)

		thread.LastOpen = unixMillisOrZero(lastOpen)
		thread.LastMessage = unixMillisOrZero(lastMessage)
		thread.LastActivity = maxTime(thread.LastMessage, thread.LastOpen, unixMillis(ts))

		archived := computeArchived(
			archivedUpto,
			archivedUpToOrder,
			latestHsOrder,
			lastMessage,
		)
		thread.IsArchived = archived
		if opts.WithStats {
			if totalMessages.Valid {
				thread.TotalMessages = int(totalMessages.Int64)
			}
		} else {
			thread.LastMessage = time.Time{}
			thread.LastOpen = time.Time{}
		}

		if !shouldIncludeThread(label, thread, archived, opts.IncludeLowPriority) {
			continue
		}

		threads = append(threads, thread)
		threadIDs = append(threadIDs, thread.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	participantsByRoom, err := s.participantsByRoom(ctx, threadIDs)
	if err != nil {
		return nil, err
	}

	for i := range threads {
		threadParticipants := participantsByRoom[threads[i].ID]
		threads[i].DisplayName = s.displayName(ctx, threads[i], threadParticipants)
		if opts.WithParticipants {
			threads[i].Participants = threadParticipants
		}
	}

	return threads, nil
}

// GetThread returns a single thread by ID.
func (s *Store) GetThread(ctx context.Context, threadID string, withStats bool) (Thread, error) {
	query := `SELECT t.threadID, t.accountID, t.timestamp,
		json_extract(t.thread,'$.title') AS title,
		json_extract(t.thread,'$.name') AS name,
		json_extract(t.thread,'$.type') AS type,
		json_extract(t.thread,'$.isUnread') AS isUnread,
		json_extract(t.thread,'$.isMarkedUnread') AS isMarkedUnread,
		json_extract(t.thread,'$.isLowPriority') AS isLowPriority,
		json_extract(t.thread,'$.unreadCount') AS unreadCount,
		json_extract(t.thread,'$.unreadMentionsCount') AS unreadMentionsCount,
		json_extract(t.thread,'$.extra.isArchivedUpto') AS isArchivedUpto,
		json_extract(t.thread,'$.extra.isArchivedUpToOrder') AS isArchivedUpToOrder,
		json_extract(t.thread,'$.extra.tags') AS tags,
		b.lastOpenTime AS lastOpenTime,
		(SELECT MAX(timestamp) FROM mx_room_messages WHERE roomID = t.threadID AND type NOT IN ('HIDDEN','REACTION')) AS lastMessageTime,
		(SELECT MAX(hsOrder) FROM mx_room_messages WHERE roomID = t.threadID AND type != 'HIDDEN') AS latestHsOrder,
		(SELECT COUNT(*) FROM mx_room_messages WHERE roomID = t.threadID AND type NOT IN ('HIDDEN','REACTION')) AS totalMessages
		FROM threads t
		LEFT JOIN breadcrumbs b ON t.threadID = b.id
		WHERE t.threadID = ? LIMIT 1`

	var thread Thread
	var accountID sql.NullString
	var title sql.NullString
	var name sql.NullString
	var threadType sql.NullString
	var isUnread sql.NullInt64
	var isMarkedUnread sql.NullInt64
	var isLowPriority sql.NullInt64
	var unreadCount sql.NullInt64
	var unreadMentions sql.NullInt64
	var archivedUpto sql.NullString
	var archivedUpToOrder sql.NullString
	var tagsRaw sql.NullString
	var lastOpen sql.NullInt64
	var lastMessage sql.NullInt64
	var latestHsOrder sql.NullInt64
	var totalMessages sql.NullInt64
	var ts int64

	row := s.db.QueryRowContext(ctx, query, threadID)
	if err := row.Scan(
		&thread.ID,
		&accountID,
		&ts,
		&title,
		&name,
		&threadType,
		&isUnread,
		&isMarkedUnread,
		&isLowPriority,
		&unreadCount,
		&unreadMentions,
		&archivedUpto,
		&archivedUpToOrder,
		&tagsRaw,
		&lastOpen,
		&lastMessage,
		&latestHsOrder,
		&totalMessages,
	); err != nil {
		return Thread{}, err
	}

	thread.AccountID = accountID.String
	thread.Title = strings.TrimSpace(title.String)
	thread.Name = strings.TrimSpace(name.String)
	thread.Type = strings.TrimSpace(threadType.String)
	thread.IsUnread = isUnread.Valid && isUnread.Int64 != 0
	thread.IsMarkedUnread = isMarkedUnread.Valid && isMarkedUnread.Int64 != 0
	thread.IsLowPriority = isLowPriority.Valid && isLowPriority.Int64 != 0
	if unreadCount.Valid {
		thread.UnreadCount = int(unreadCount.Int64)
	}
	if unreadMentions.Valid {
		thread.UnreadMentions = int(unreadMentions.Int64)
	}
	thread.Tags = parseTags(tagsRaw.String)
	thread.LastOpen = unixMillisOrZero(lastOpen)
	thread.LastMessage = unixMillisOrZero(lastMessage)
	thread.LastActivity = maxTime(thread.LastMessage, thread.LastOpen, unixMillis(ts))
	thread.IsArchived = computeArchived(
		archivedUpto,
		archivedUpToOrder,
		latestHsOrder,
		lastMessage,
	)
	if withStats && totalMessages.Valid {
		thread.TotalMessages = int(totalMessages.Int64)
	}

	participantsByRoom, err := s.participantsByRoom(ctx, []string{threadID})
	if err != nil {
		return Thread{}, err
	}
	thread.Participants = participantsByRoom[threadID]
	thread.DisplayName = s.displayName(ctx, thread, thread.Participants)

	if !withStats {
		thread.LastMessage = time.Time{}
		thread.LastOpen = time.Time{}
		thread.TotalMessages = 0
	}

	return thread, nil
}

// ListMessages returns messages for a thread.
func (s *Store) ListMessages(ctx context.Context, opts MessageListOptions) ([]Message, error) {
	if opts.ThreadID == "" {
		return nil, errors.New("thread ID is required")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	query := strings.Builder{}
	query.WriteString(`SELECT id, eventID, roomID, senderContactID, timestamp, isSentByMe, type,
		COALESCE(text_content, '') AS text_content,
		COALESCE(message, '') AS message
		FROM mx_room_messages
		WHERE roomID = ?
		AND isDeleted = 0
		AND type NOT IN ('HIDDEN','REACTION')`)

	args := []any{opts.ThreadID}

	if opts.After != nil {
		query.WriteString(" AND timestamp >= ?")
		args = append(args, opts.After.UnixMilli())
	}
	if opts.Before != nil {
		query.WriteString(" AND timestamp <= ?")
		args = append(args, opts.Before.UnixMilli())
	}

	query.WriteString(" ORDER BY timestamp DESC LIMIT ?")
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	messages := []Message{}
	for rows.Next() {
		var msg Message
		var ts int64
		var isSentByMe int
		var msgType sql.NullString
		var textContent sql.NullString
		var rawMessage sql.NullString
		if err := rows.Scan(
			&msg.ID,
			&msg.EventID,
			&msg.ThreadID,
			&msg.SenderID,
			&ts,
			&isSentByMe,
			&msgType,
			&textContent,
			&rawMessage,
		); err != nil {
			return nil, err
		}
		msg.Timestamp = unixMillis(ts)
		msg.IsSentByMe = isSentByMe != 0
		msg.Type = strings.TrimSpace(msgType.String)
		msg.Text = ResolveMessageText(rawMessage.String, msg.Type, textContent.String, opts.Format)
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	participantsByRoom, err := s.participantsByRoom(ctx, []string{opts.ThreadID})
	if err != nil {
		return nil, err
	}
	participants := participantsByRoom[opts.ThreadID]
	participantIndex := indexParticipants(participants)

	for i := range messages {
		if p, ok := participantIndex[messages[i].SenderID]; ok {
			messages[i].SenderName = p.Name
		}
	}

	return messages, nil
}

// SearchMessages searches messages using FTS (or LIKE fallback).
func (s *Store) SearchMessages(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	if strings.TrimSpace(opts.Query) == "" {
		return nil, errors.New("search query is required")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	useFTS, err := s.HasFTS(ctx)
	if err != nil {
		return nil, err
	}

	buildQuery := func(useFTS bool) (string, []any) {
		query := strings.Builder{}
		args := []any{}

		if useFTS {
			query.WriteString(`SELECT m.id, m.eventID, m.roomID, m.senderContactID, m.timestamp, m.isSentByMe, m.type,
				COALESCE(m.text_content, '') AS text_content,
				COALESCE(m.message, '') AS message,
				bm25(f) AS rank
				FROM mx_room_messages_fts f
				JOIN mx_room_messages m ON m.id = f.rowid
				WHERE f.text_content MATCH ?
				AND m.isDeleted = 0
				AND m.type NOT IN ('HIDDEN','REACTION')`)
			args = append(args, opts.Query)
		} else {
			query.WriteString(`SELECT m.id, m.eventID, m.roomID, m.senderContactID, m.timestamp, m.isSentByMe, m.type,
				COALESCE(m.text_content, '') AS text_content,
				COALESCE(m.message, '') AS message,
				0 as rank
				FROM mx_room_messages m
				WHERE json_extract(m.message,'$.text') LIKE ?
				AND m.isDeleted = 0
				AND m.type NOT IN ('HIDDEN','REACTION')`)
			args = append(args, "%"+opts.Query+"%")
		}

		if opts.ThreadID != "" {
			query.WriteString(" AND m.roomID = ?")
			args = append(args, opts.ThreadID)
		}

		if opts.AccountID != "" {
			query.WriteString(" AND m.roomID IN (SELECT threadID FROM threads WHERE accountID = ?)")
			args = append(args, opts.AccountID)
		}

		if opts.Days > 0 {
			cutoff := time.Now().AddDate(0, 0, -opts.Days).UnixMilli()
			query.WriteString(" AND m.timestamp >= ?")
			args = append(args, cutoff)
		}

		query.WriteString(" ORDER BY rank ASC, m.timestamp DESC LIMIT ?")
		args = append(args, limit)
		return query.String(), args
	}

	queryStr, args := buildQuery(useFTS)
	rows, err := s.db.QueryContext(ctx, queryStr, args...)
	if err != nil && useFTS && isFTSError(err) {
		queryStr, args = buildQuery(false)
		rows, err = s.db.QueryContext(ctx, queryStr, args...)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	matches := []Message{}
	roomIDs := []string{}

	for rows.Next() {
		var msg Message
		var ts int64
		var isSentByMe int
		var msgType sql.NullString
		var textContent sql.NullString
		var rawMessage sql.NullString
		if err := rows.Scan(
			&msg.ID,
			&msg.EventID,
			&msg.ThreadID,
			&msg.SenderID,
			&ts,
			&isSentByMe,
			&msgType,
			&textContent,
			&rawMessage,
			&msg.Score,
		); err != nil {
			return nil, err
		}
		msg.Timestamp = unixMillis(ts)
		msg.IsSentByMe = isSentByMe != 0
		msg.Type = strings.TrimSpace(msgType.String)
		msg.Text = ResolveMessageText(rawMessage.String, msg.Type, textContent.String, opts.Format)
		matches = append(matches, msg)
		roomIDs = append(roomIDs, msg.ThreadID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	threadInfo, err := s.threadInfoByID(ctx, uniqueStrings(roomIDs))
	if err != nil {
		return nil, err
	}

	participantsByRoom, err := s.participantsByRoom(ctx, uniqueStrings(roomIDs))
	if err != nil {
		return nil, err
	}
	participantIndexByRoom := map[string]map[string]Participant{}
	for roomID, participants := range participantsByRoom {
		participantIndexByRoom[roomID] = indexParticipants(participants)
	}

	for i := range matches {
		info := threadInfo[matches[i].ThreadID]
		matches[i].AccountID = info.AccountID
		matches[i].ThreadName = s.displayName(ctx, Thread{ID: matches[i].ThreadID, Title: info.Title, Name: info.Name, Type: info.Type, AccountID: info.AccountID}, participantsByRoom[matches[i].ThreadID])
		if participantIndex, ok := participantIndexByRoom[matches[i].ThreadID]; ok {
			if p, ok := participantIndex[matches[i].SenderID]; ok {
				matches[i].SenderName = p.Name
			}
		}
	}

	results := make([]SearchResult, 0, len(matches))
	for _, match := range matches {
		result := SearchResult{Match: match}
		if opts.Context > 0 || opts.Window > 0 {
			contextMessages, err := s.fetchContextMessages(ctx, match, opts, participantsByRoom, threadInfo)
			if err != nil {
				return nil, err
			}
			result.Context = contextMessages
		}
		results = append(results, result)
	}

	return results, nil
}

func (s *Store) fetchContextMessages(
	ctx context.Context,
	match Message,
	opts SearchOptions,
	participantsByRoom map[string][]Participant,
	threadInfo map[string]threadInfo,
) ([]Message, error) {
	window := opts.Window
	if window == 0 {
		window = defaultContextWindow
	}

	start := match.Timestamp.Add(-window).UnixMilli()
	end := match.Timestamp.Add(window).UnixMilli()

	query := `SELECT id, eventID, roomID, senderContactID, timestamp, isSentByMe, type,
		COALESCE(text_content, '') AS text_content,
		COALESCE(message, '') AS message
		FROM mx_room_messages
		WHERE roomID = ?
		AND timestamp BETWEEN ? AND ?
		AND isDeleted = 0
		AND type NOT IN ('HIDDEN','REACTION')
		ORDER BY timestamp ASC`

	rows, err := s.db.QueryContext(ctx, query, match.ThreadID, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	messages := []Message{}
	for rows.Next() {
		var msg Message
		var ts int64
		var isSentByMe int
		var msgType sql.NullString
		var textContent sql.NullString
		var rawMessage sql.NullString
		if err := rows.Scan(
			&msg.ID,
			&msg.EventID,
			&msg.ThreadID,
			&msg.SenderID,
			&ts,
			&isSentByMe,
			&msgType,
			&textContent,
			&rawMessage,
		); err != nil {
			return nil, err
		}
		msg.Timestamp = unixMillis(ts)
		msg.IsSentByMe = isSentByMe != 0
		msg.Type = strings.TrimSpace(msgType.String)
		msg.Text = ResolveMessageText(rawMessage.String, msg.Type, textContent.String, opts.Format)
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	participants := participantsByRoom[match.ThreadID]
	participantIndex := indexParticipants(participants)
	info := threadInfo[match.ThreadID]
	threadName := s.displayName(ctx, Thread{ID: match.ThreadID, Title: info.Title, Name: info.Name, Type: info.Type, AccountID: info.AccountID}, participants)

	for i := range messages {
		messages[i].AccountID = info.AccountID
		messages[i].ThreadName = threadName
		if p, ok := participantIndex[messages[i].SenderID]; ok {
			messages[i].SenderName = p.Name
		}
	}

	if opts.Context > 0 {
		return trimContext(messages, match.ID, opts.Context), nil
	}

	return messages, nil
}

type threadInfo struct {
	AccountID string
	Title     string
	Name      string
	Type      string
}

func (s *Store) threadInfoByID(ctx context.Context, ids []string) (map[string]threadInfo, error) {
	info := map[string]threadInfo{}
	if len(ids) == 0 {
		return info, nil
	}
	query := fmt.Sprintf(`SELECT threadID, accountID,
		json_extract(thread,'$.title') AS title,
		json_extract(thread,'$.name') AS name,
		json_extract(thread,'$.type') AS type
		FROM threads WHERE threadID IN (%s)`, placeholders(len(ids)))

	rows, err := s.db.QueryContext(ctx, query, stringSliceToAny(ids)...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id string
		var accountID sql.NullString
		var title sql.NullString
		var name sql.NullString
		var threadType sql.NullString
		if err := rows.Scan(&id, &accountID, &title, &name, &threadType); err != nil {
			return nil, err
		}
		info[id] = threadInfo{
			AccountID: accountID.String,
			Title:     strings.TrimSpace(title.String),
			Name:      strings.TrimSpace(name.String),
			Type:      strings.TrimSpace(threadType.String),
		}
	}

	return info, rows.Err()
}

func (s *Store) participantsByRoom(ctx context.Context, roomIDs []string) (map[string][]Participant, error) {
	participantsByRoom := map[string][]Participant{}
	roomIDs = uniqueStrings(roomIDs)
	if len(roomIDs) == 0 {
		return participantsByRoom, nil
	}

	query := fmt.Sprintf(`SELECT room_id, id, full_name, nickname, is_self
		FROM participants WHERE room_id IN (%s)`, placeholders(len(roomIDs)))

	rows, err := s.db.QueryContext(ctx, query, stringSliceToAny(roomIDs)...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var roomID, id string
		var fullName sql.NullString
		var nickname sql.NullString
		var isSelf sql.NullInt64
		if err := rows.Scan(&roomID, &id, &fullName, &nickname, &isSelf); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(fullName.String)
		if name == "" {
			name = strings.TrimSpace(nickname.String)
		}
		if name == "" {
			name = id
		}
		participantsByRoom[roomID] = append(participantsByRoom[roomID], Participant{
			ID:     id,
			Name:   name,
			IsSelf: isSelf.Valid && isSelf.Int64 != 0,
		})
	}

	return participantsByRoom, rows.Err()
}

func (s *Store) displayName(ctx context.Context, thread Thread, participants []Participant) string {
	if thread.Title != "" {
		return thread.Title
	}
	if thread.Name != "" {
		return thread.Name
	}

	if s.bridge != nil && (thread.Type == "single" || thread.Type == "dm") {
		if name, ok, err := s.bridge.LookupDMName(ctx, thread.ID, thread.AccountID); err == nil && ok {
			return name
		}
	}

	nonSelf := []string{}
	for _, p := range participants {
		if p.IsSelf {
			continue
		}
		nonSelf = append(nonSelf, p.Name)
	}

	if len(nonSelf) == 0 {
		return "(unknown)"
	}

	if thread.Type == "single" || thread.Type == "dm" {
		return nonSelf[0]
	}

	if len(nonSelf) <= 3 {
		return strings.Join(nonSelf, ", ")
	}

	return fmt.Sprintf("%s +%d", strings.Join(nonSelf[:3], ", "), len(nonSelf)-3)
}

func indexParticipants(participants []Participant) map[string]Participant {
	index := map[string]Participant{}
	for _, p := range participants {
		index[p.ID] = p
	}
	return index
}

func shouldIncludeThread(label ThreadLabel, thread Thread, archived bool, includeLowPriority bool) bool {
	if !includeLowPriority && thread.IsLowPriority {
		return false
	}

	switch label {
	case LabelInbox:
		if containsTag(thread.Tags, "favourite") {
			return true
		}
		if archived {
			return false
		}
		return true
	case LabelArchive:
		if !archived {
			return false
		}
		if containsTag(thread.Tags, "favourite") {
			return false
		}
		return true
	case LabelFavourite:
		return containsTag(thread.Tags, "favourite")
	case LabelUnread:
		return thread.IsUnread || thread.IsMarkedUnread
	case LabelAll:
		return true
	default:
		return true
	}
}

func parseTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var tags []string
	if err := jsonUnmarshalStrings(raw, &tags); err == nil {
		return tags
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "favourite") {
		return []string{"favourite"}
	}
	return nil
}

func computeArchived(
	archivedUpto sql.NullString,
	archivedUpToOrder sql.NullString,
	latestHsOrder sql.NullInt64,
	lastMessage sql.NullInt64,
) bool {
	if order, ok := parseArchivedValue(archivedUpToOrder); ok && latestHsOrder.Valid {
		return latestHsOrder.Int64 <= order
	}
	if ts, ok := parseArchivedValue(archivedUpto); ok {
		if ts > 1_000_000_000_000 {
			if lastMessage.Valid {
				return lastMessage.Int64 <= ts
			}
			return true
		}
		if latestHsOrder.Valid {
			return latestHsOrder.Int64 <= ts
		}
		return true
	}
	return archivedUpto.Valid && strings.TrimSpace(archivedUpto.String) != ""
}

func parseArchivedValue(value sql.NullString) (int64, bool) {
	if !value.Valid {
		return 0, false
	}
	raw := strings.TrimSpace(value.String)
	if raw == "" {
		return 0, false
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "ts") {
		raw = raw[2:]
	}
	if raw == "" {
		return 0, false
	}
	if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return parsed, true
	}
	if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
		return int64(parsed), true
	}
	return 0, false
}

func isFTSError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such module: fts5") || strings.Contains(msg, "no such table: mx_room_messages_fts")
}

func containsTag(tags []string, target string) bool {
	if len(tags) == 0 {
		return false
	}
	for _, tag := range tags {
		if strings.EqualFold(tag, target) {
			return true
		}
	}
	return false
}

func trimContext(messages []Message, matchID int64, context int) []Message {
	if context <= 0 || len(messages) == 0 {
		return messages
	}

	idx := -1
	for i, msg := range messages {
		if msg.ID == matchID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return messages
	}

	start := idx - context
	if start < 0 {
		start = 0
	}
	end := idx + context + 1
	if end > len(messages) {
		end = len(messages)
	}

	trimmed := make([]Message, 0, end-start-1)
	for i := start; i < end; i++ {
		if i == idx {
			continue
		}
		trimmed = append(trimmed, messages[i])
	}
	return trimmed
}

func unixMillis(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func unixMillisOrZero(value sql.NullInt64) time.Time {
	if !value.Valid || value.Int64 == 0 {
		return time.Time{}
	}
	return time.UnixMilli(value.Int64)
}

func maxTime(times ...time.Time) time.Time {
	latest := time.Time{}
	for _, t := range times {
		if t.After(latest) {
			latest = t
		}
	}
	return latest
}

func placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func stringSliceToAny(values []string) []any {
	args := make([]any, 0, len(values))
	for _, v := range values {
		args = append(args, v)
	}
	return args
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		unique = append(unique, v)
	}
	return unique
}
