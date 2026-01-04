package beeper

import "time"

const (
	defaultLimit         = 50
	defaultContextWindow = time.Hour
)

// MessageFormat controls how message text is rendered.
type MessageFormat string

const (
	// FormatPlain returns the raw/plain text only.
	FormatPlain MessageFormat = "plain"
	// FormatRich renders attachments and non-text messages with placeholders.
	FormatRich MessageFormat = "rich"
)

// ThreadLabel filters conversation lists.
type ThreadLabel string

const (
	// LabelAll returns all threads.
	LabelAll ThreadLabel = "all"
	// LabelInbox returns inbox threads.
	LabelInbox ThreadLabel = "inbox"
	// LabelArchive returns archived threads.
	LabelArchive ThreadLabel = "archive"
	// LabelFavourite returns favourite threads.
	LabelFavourite ThreadLabel = "favourite"
	// LabelUnread returns threads with unread messages.
	LabelUnread ThreadLabel = "unread"
)

// StoreOptions configures Store behavior.
type StoreOptions struct {
	BridgeLookup bool
	BridgeRoot   string
}

// Thread describes a conversation.
type Thread struct {
	ID             string        `json:"id"`
	AccountID      string        `json:"accountId"`
	Title          string        `json:"title,omitempty"`
	Name           string        `json:"name,omitempty"`
	Type           string        `json:"type,omitempty"`
	DisplayName    string        `json:"displayName"`
	LastActivity   time.Time     `json:"lastActivity"`
	LastMessage    time.Time     `json:"lastMessageTime,omitempty"`
	LastOpen       time.Time     `json:"lastOpenTime,omitempty"`
	IsUnread       bool          `json:"isUnread"`
	IsMarkedUnread bool          `json:"isMarkedUnread"`
	IsLowPriority  bool          `json:"isLowPriority"`
	IsArchived     bool          `json:"isArchived"`
	UnreadCount    int           `json:"unreadCount,omitempty"`
	UnreadMentions int           `json:"unreadMentions,omitempty"`
	TotalMessages  int           `json:"totalMessages,omitempty"`
	Tags           []string      `json:"tags,omitempty"`
	Participants   []Participant `json:"participants,omitempty"`
}

// Participant represents a user in a thread.
type Participant struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	IsSelf bool   `json:"isSelf"`
}

// Message represents a message row from Beeper's store.
type Message struct {
	ID         int64     `json:"id"`
	EventID    string    `json:"eventId"`
	ThreadID   string    `json:"threadId"`
	ThreadName string    `json:"threadName,omitempty"`
	AccountID  string    `json:"accountId,omitempty"`
	SenderID   string    `json:"senderId"`
	SenderName string    `json:"senderName,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	IsSentByMe bool      `json:"isSentByMe"`
	Type       string    `json:"type"`
	Text       string    `json:"text"`
	Score      float64   `json:"score,omitempty"`
}

// SearchResult is a match plus optional surrounding context.
type SearchResult struct {
	Match   Message   `json:"match"`
	Context []Message `json:"context,omitempty"`
}

// ThreadListOptions controls thread list filtering.
type ThreadListOptions struct {
	Days               int
	Limit              int
	AccountID          string
	Label              ThreadLabel
	IncludeLowPriority bool
	WithParticipants   bool
	WithStats          bool
}

// MessageListOptions controls message list filtering.
type MessageListOptions struct {
	ThreadID string
	Limit    int
	After    *time.Time
	Before   *time.Time
	Format   MessageFormat
}

// SearchOptions controls full-text search behavior.
type SearchOptions struct {
	Query     string
	ThreadID  string
	Days      int
	Limit     int
	AccountID string
	Context   int
	Window    time.Duration
	Format    MessageFormat
}
