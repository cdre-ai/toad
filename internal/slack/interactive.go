package slack

import (
	"context"
	"log/slog"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

const actionIDFix = "toad_fix"

// parseFixAction extracts the "toad_fix" button click from an InteractionCallback.
// Returns (found, threadTS, channelID, userID).
func parseFixAction(cb *slack.InteractionCallback) (bool, string, string, string) {
	for _, a := range cb.ActionCallback.BlockActions {
		if a.ActionID == actionIDFix {
			return true, a.Value, cb.Channel.ID, cb.User.ID
		}
	}
	return false, "", "", ""
}

func handleInteractive(ctx context.Context, c *Client, evt socketmode.Event) {
	cb, ok := evt.Data.(slack.InteractionCallback)
	if !ok {
		return
	}

	if cb.Type != slack.InteractionTypeBlockActions {
		return
	}

	found, threadTS, channel, userID := parseFixAction(&cb)
	if !found {
		return
	}

	slog.Info("fix button clicked", "channel", channel, "user", userID, "thread", threadTS)

	// Instant feedback: replace button with processing indicator before any API calls.
	processingBlocks := SpawnedByBlocks(cb.Message.Blocks, "")
	if err := c.UpdateMessageWithBlocks(channel, cb.MessageTs, cb.Message.Text, processingBlocks); err != nil {
		slog.Warn("failed to update button message", "error", err)
	}

	go func() {
		userName := c.ResolveUserName(userID)
		finalBlocks := SpawnedByBlocks(cb.Message.Blocks, userName)
		if err := c.UpdateMessageWithBlocks(channel, cb.MessageTs, cb.Message.Text, finalBlocks); err != nil {
			slog.Warn("failed to update button message", "error", err)
		}

		msg, err := c.FetchMessage(channel, threadTS)
		if err != nil {
			slog.Error("failed to fetch thread message for fix button", "error", err)
			return
		}
		msg.IsTriggered = true
		msg.IsTadpoleRequest = true

		if c.handler != nil {
			c.handler(ctx, msg)
		}
	}()
}
