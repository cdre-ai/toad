package slack

import (
	"testing"

	goslack "github.com/slack-go/slack"
)

func TestHandleSlashCommand_NoHandler(t *testing.T) {
	// Should not panic when mcpHandler is nil
	c := &Client{}
	handleSlashCommand(c, goslack.SlashCommand{Text: "connect"})
}
