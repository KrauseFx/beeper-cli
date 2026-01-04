package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/KrauseFx/beeper-cli/internal/beeper"
	"github.com/spf13/cobra"
)

func newThreadsCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "threads",
		Short: "List and inspect conversations",
	}

	cmd.AddCommand(newThreadsListCmd(app))
	cmd.AddCommand(newThreadsShowCmd(app))

	return cmd
}

func newThreadsListCmd(app *App) *cobra.Command {
	var days int
	var limit int
	var accountID string
	var label string
	var includeLowPriority bool
	var withParticipants bool
	var withStats bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List threads ordered by last activity",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()
			store, _, err := app.openStore()
			if err != nil {
				return err
			}
			defer func() {
				_ = store.Close()
			}()

			threads, err := store.ListThreads(ctx, beeper.ThreadListOptions{
				Days:               days,
				Limit:              limit,
				AccountID:          accountID,
				Label:              beeper.ThreadLabel(label),
				IncludeLowPriority: includeLowPriority,
				WithParticipants:   withParticipants,
				WithStats:          withStats,
			})
			if err != nil {
				return err
			}

			if app.JSON {
				return writeJSON(threads)
			}

			w := newTabWriter()
			if err := writeLine(w, "TIME\tACCOUNT\tTHREAD\tTHREAD_ID"); err != nil {
				return err
			}
			for _, thread := range threads {
				if err := writef(w, "%s\t%s\t%s\t%s\n", formatTime(thread.LastActivity), safe(thread.AccountID), safe(thread.DisplayName), thread.ID); err != nil {
					return err
				}
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVar(&days, "days", 0, "only include threads active in the last N days")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of threads to return")
	cmd.Flags().StringVar(&accountID, "account", "", "filter by account/platform ID")
	cmd.Flags().StringVar(&label, "label", string(beeper.LabelAll), "filter by label: inbox|archive|favourite|unread|all")
	cmd.Flags().BoolVar(&includeLowPriority, "include-low-priority", false, "include low-priority threads")
	cmd.Flags().BoolVar(&withParticipants, "with-participants", false, "include participants in JSON output")
	cmd.Flags().BoolVar(&withStats, "with-stats", false, "include message stats in JSON output")

	return cmd
}

func newThreadsShowCmd(app *App) *cobra.Command {
	var threadID string
	var withStats bool
	var withLast int
	var format string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details for a single thread",
		RunE: func(_ *cobra.Command, args []string) error {
			if threadID == "" && len(args) > 0 {
				threadID = args[0]
			}
			if threadID == "" {
				return fmt.Errorf("thread ID is required")
			}

			ctx := context.Background()
			store, _, err := app.openStore()
			if err != nil {
				return err
			}
			defer func() {
				_ = store.Close()
			}()

			formatValue, err := parseMessageFormat(format)
			if err != nil {
				return err
			}

			thread, err := store.GetThread(ctx, threadID, withStats)
			if err != nil {
				return err
			}

			if app.JSON {
				if withLast > 0 {
					messages, err := store.ListMessages(ctx, beeper.MessageListOptions{
						ThreadID: threadID,
						Limit:    withLast,
						Format:   formatValue,
					})
					if err != nil {
						return err
					}
					return writeJSON(map[string]any{
						"thread":   thread,
						"messages": messages,
					})
				}
				return writeJSON(thread)
			}

			w := newTabWriter()
			if err := writeLine(w, "FIELD\tVALUE"); err != nil {
				return err
			}
			if err := writef(w, "ID\t%s\n", thread.ID); err != nil {
				return err
			}
			if err := writef(w, "Account\t%s\n", safe(thread.AccountID)); err != nil {
				return err
			}
			if err := writef(w, "Name\t%s\n", safe(thread.DisplayName)); err != nil {
				return err
			}
			if err := writef(w, "Type\t%s\n", safe(thread.Type)); err != nil {
				return err
			}
			if err := writef(w, "Last Activity\t%s\n", formatTime(thread.LastActivity)); err != nil {
				return err
			}
			if err := writef(w, "Archived\t%t\n", thread.IsArchived); err != nil {
				return err
			}
			if err := writef(w, "Low Priority\t%t\n", thread.IsLowPriority); err != nil {
				return err
			}
			if err := writef(w, "Unread\t%t\n", thread.IsUnread); err != nil {
				return err
			}
			if err := writef(w, "Unread Count\t%d\n", thread.UnreadCount); err != nil {
				return err
			}
			if err := writef(w, "Unread Mentions\t%d\n", thread.UnreadMentions); err != nil {
				return err
			}
			if len(thread.Tags) > 0 {
				if err := writef(w, "Tags\t%s\n", strings.Join(thread.Tags, ",")); err != nil {
					return err
				}
			}
			if err := w.Flush(); err != nil {
				return err
			}

			if len(thread.Participants) > 0 {
				fmt.Println()
				fmt.Println("Participants:")
				for _, p := range thread.Participants {
					suffix := ""
					if p.IsSelf {
						suffix = " (you)"
					}
					fmt.Printf("- %s%s\n", strings.TrimSpace(p.Name), suffix)
				}
			}

			if withLast > 0 {
				fmt.Println()
				fmt.Println("Recent messages:")
				messages, err := store.ListMessages(ctx, beeper.MessageListOptions{
					ThreadID: threadID,
					Limit:    withLast,
					Format:   formatValue,
				})
				if err != nil {
					return err
				}
				for _, msg := range messages {
					sender := msg.SenderName
					if sender == "" {
						sender = msg.SenderID
					}
					fmt.Printf("- %s %s: %s\n", formatTime(msg.Timestamp), sender, msg.Text)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&threadID, "id", "", "thread ID (room ID)")
	cmd.Flags().BoolVar(&withStats, "with-stats", false, "include message stats")
	cmd.Flags().IntVar(&withLast, "with-last", 0, "include last N messages")
	cmd.Flags().StringVar(&format, "format", string(beeper.FormatRich), "message format: plain|rich")

	return cmd
}

func safe(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
