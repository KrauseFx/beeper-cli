package cli

import (
	"context"
	"fmt"

	"github.com/KrauseFx/beeper-cli/internal/beeper"
	"github.com/spf13/cobra"
)

func newMessagesCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Read messages from a conversation",
	}

	cmd.AddCommand(newMessagesListCmd(app))

	return cmd
}

func newMessagesListCmd(app *App) *cobra.Command {
	var threadID string
	var limit int
	var days int
	var after string
	var before string
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent messages in a thread",
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

			afterTime, err := parseTimeFlag(after, days)
			if err != nil {
				return err
			}
			beforeTime, err := parseTimePtr(before)
			if err != nil {
				return err
			}
			formatValue, err := parseMessageFormat(format)
			if err != nil {
				return err
			}

			messages, err := store.ListMessages(ctx, beeper.MessageListOptions{
				ThreadID: threadID,
				Limit:    limit,
				After:    afterTime,
				Before:   beforeTime,
				Format:   formatValue,
			})
			if err != nil {
				return err
			}

			if app.JSON {
				return writeJSON(messages)
			}

			w := newTabWriter()
			if err := writeLine(w, "TIME\tSENDER\tTEXT"); err != nil {
				return err
			}
			for _, msg := range messages {
				sender := msg.SenderName
				if sender == "" {
					sender = msg.SenderID
				}
				if err := writef(w, "%s\t%s\t%s\n", formatTime(msg.Timestamp), sender, msg.Text); err != nil {
					return err
				}
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&threadID, "thread", "", "thread ID (room ID)")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of messages to return")
	cmd.Flags().IntVar(&days, "days", 0, "only include messages from the last N days")
	cmd.Flags().StringVar(&after, "after", "", "only include messages after this RFC3339 timestamp")
	cmd.Flags().StringVar(&before, "before", "", "only include messages before this RFC3339 timestamp")
	cmd.Flags().StringVar(&format, "format", string(beeper.FormatRich), "message format: plain|rich")

	return cmd
}
