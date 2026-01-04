package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/KrauseFx/beeper-cli/internal/beeper"
	"github.com/spf13/cobra"
)

func newSearchCmd(app *App) *cobra.Command {
	var days int
	var limit int
	var threadID string
	var accountID string
	var contextSize int
	var window string
	var format string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across messages",
		RunE: func(_ *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return fmt.Errorf("search query is required")
			}

			windowDuration, err := parseDuration(window)
			if err != nil {
				return err
			}
			formatValue, err := parseMessageFormat(format)
			if err != nil {
				return err
			}

			ctx := context.Background()
			store, _, err := app.openStore()
			if err != nil {
				return err
			}
			defer func() {
				_ = store.Close()
			}()

			results, err := store.SearchMessages(ctx, beeper.SearchOptions{
				Query:     query,
				ThreadID:  threadID,
				Days:      days,
				Limit:     limit,
				AccountID: accountID,
				Context:   contextSize,
				Window:    windowDuration,
				Format:    formatValue,
			})
			if err != nil {
				return err
			}

			if app.JSON {
				return writeJSON(results)
			}

			w := newTabWriter()
			if err := writeLine(w, "TIME\tACCOUNT\tTHREAD\tSENDER\tTEXT\tSCORE"); err != nil {
				return err
			}
			for _, msg := range results {
				match := msg.Match
				sender := match.SenderName
				if sender == "" {
					sender = match.SenderID
				}
				if err := writef(w, "%s\t%s\t%s\t%s\t%s\t%.2f\n", formatTime(match.Timestamp), safe(match.AccountID), safe(match.ThreadName), sender, match.Text, match.Score); err != nil {
					return err
				}
				if contextSize > 0 || windowDuration > 0 {
					for _, ctxMsg := range msg.Context {
						ctxSender := ctxMsg.SenderName
						if ctxSender == "" {
							ctxSender = ctxMsg.SenderID
						}
						if err := writef(w, "  %s\t%s\t%s\t%s\t%s\t\n", formatTime(ctxMsg.Timestamp), safe(ctxMsg.AccountID), safe(ctxMsg.ThreadName), ctxSender, ctxMsg.Text); err != nil {
							return err
						}
					}
				}
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVar(&days, "days", 0, "only include messages from the last N days")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of results")
	cmd.Flags().StringVar(&threadID, "thread", "", "only search within a thread (room ID)")
	cmd.Flags().StringVar(&accountID, "account", "", "filter by account/platform ID")
	cmd.Flags().IntVar(&contextSize, "context", 0, "include N messages before/after the match")
	cmd.Flags().StringVar(&window, "window", "", "context time window (e.g., 60m)")
	cmd.Flags().StringVar(&format, "format", string(beeper.FormatRich), "message format: plain|rich")

	return cmd
}
