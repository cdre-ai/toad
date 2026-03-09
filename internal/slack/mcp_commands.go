package slack

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/slack-go/slack"

	"github.com/scaler-tech/toad/internal/agent"
	"github.com/scaler-tech/toad/internal/config"
	"github.com/scaler-tech/toad/internal/state"
)

// SlashCommandHandler processes /toad slash commands.
type SlashCommandHandler struct {
	db    *state.DB
	api   *slack.Client
	cfg   config.MCPConfig
	agent agent.Provider
	model string
}

// NewSlashCommandHandler creates a new handler for /toad commands.
func NewSlashCommandHandler(db *state.DB, api *slack.Client, cfg config.MCPConfig, ag agent.Provider, model string) *SlashCommandHandler {
	return &SlashCommandHandler{
		db:    db,
		api:   api,
		cfg:   cfg,
		agent: ag,
		model: model,
	}
}

// handleSlashCommand dispatches /toad slash commands.
func handleSlashCommand(c *Client, cmd slack.SlashCommand) {
	if c.mcpHandler == nil {
		slog.Debug("slash command received but no handler configured")
		return
	}

	args := strings.Fields(strings.ToLower(strings.TrimSpace(cmd.Text)))
	if len(args) == 0 {
		c.mcpHandler.handleHelp(cmd)
		return
	}

	switch args[0] {
	case "mcp":
		if !c.mcpHandler.cfg.Enabled {
			c.mcpHandler.ephemeral(cmd, "MCP server is not enabled. Add `mcp.enabled: true` to your toad config to use MCP commands.")
			return
		}
		if len(args) < 2 {
			c.mcpHandler.handleMCPHelp(cmd)
			return
		}
		switch args[1] {
		case "connect":
			c.mcpHandler.handleMCPConnect(cmd)
		case "revoke":
			c.mcpHandler.handleMCPRevoke(cmd)
		case "status":
			c.mcpHandler.handleMCPStatus(cmd)
		case "ping":
			c.mcpHandler.handleMCPPing(cmd)
		default:
			c.mcpHandler.handleMCPHelp(cmd)
		}
	case "status":
		c.mcpHandler.handleStatus(cmd)
	case "joke":
		c.mcpHandler.handleJoke(cmd)
	case "help":
		c.mcpHandler.handleHelp(cmd)
	default:
		c.mcpHandler.ephemeral(cmd, fmt.Sprintf("Unknown command: `/toad %s`. Try `/toad help` to see what I can do.", strings.Join(args, " ")))
	}
}

// --- /toad status ---

func (h *SlashCommandHandler) handleStatus(cmd slack.SlashCommand) {
	stats, err := h.db.ReadDaemonStats()
	if err != nil {
		slog.Error("failed to read daemon stats", "error", err)
		h.ephemeral(cmd, "Sorry, I couldn't read daemon status.")
		return
	}
	if stats == nil {
		h.ephemeral(cmd, "Toad daemon is not running (no heartbeat found).")
		return
	}

	uptime := time.Since(stats.StartedAt).Truncate(time.Second)
	age := time.Since(stats.Heartbeat).Truncate(time.Second)

	status := "running"
	if stats.Draining {
		status = "draining"
	}
	if age > 30*time.Second {
		status = fmt.Sprintf("stale (last heartbeat %s ago)", age)
	}

	text := fmt.Sprintf("*Toad Daemon Status*\n"+
		"• Status: *%s*\n"+
		"• Version: %s\n"+
		"• Uptime: %s\n"+
		"• Ribbits: %d\n"+
		"• Triages: %d (bug: %d, feature: %d, question: %d)",
		status,
		stats.Version,
		uptime,
		stats.Ribbits,
		stats.Triages,
		stats.TriageByCategory["bug"],
		stats.TriageByCategory["feature"],
		stats.TriageByCategory["question"],
	)

	if stats.DigestEnabled {
		mode := ""
		if stats.DigestDryRun {
			mode = " (dry run)"
		}
		text += fmt.Sprintf("\n• Digest: enabled%s — %d processed, %d opportunities", mode, stats.DigestProcessed, stats.DigestOpps)
	}

	h.ephemeral(cmd, text)
}

// --- /toad mcp ... ---

func (h *SlashCommandHandler) handleMCPConnect(cmd slack.SlashCommand) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		slog.Error("failed to generate MCP token", "error", err)
		h.ephemeral(cmd, "Sorry, I couldn't generate a token. Please try again.")
		return
	}
	token := "toad_" + hex.EncodeToString(b)

	role := "user"
	for _, dev := range h.cfg.Devs {
		if dev == cmd.UserID {
			role = "dev"
			break
		}
	}

	// Revoke any existing tokens before issuing a new one
	_ = h.db.RevokeMCPToken(cmd.UserID)

	tok := &state.MCPToken{
		Token:       token,
		SlackUserID: cmd.UserID,
		SlackUser:   cmd.UserName,
		Role:        role,
		CreatedAt:   time.Now(),
	}
	if err := h.db.SaveMCPToken(tok); err != nil {
		slog.Error("failed to save MCP token", "error", err)
		h.ephemeral(cmd, "Sorry, I couldn't save your token. Please try again.")
		return
	}

	slog.Info("MCP token issued", "user", cmd.UserName, "role", role)

	snippet := fmt.Sprintf(`{
  "mcpServers": {
    "toad": {
      "url": "%s://%s:%d/mcp",
      "headers": {
        "Authorization": "Bearer %s"
      }
    }
  }
}`, h.mcpScheme(), h.cfg.Host, h.cfg.Port, token)

	text := fmt.Sprintf("Your MCP token has been created (role: *%s*).\n\nToken:\n```\n%s\n```\n\nAdd this to your Claude Desktop config:\n```\n%s\n```\n\nKeep this token secret — it grants access to toad on your behalf.", role, token, snippet)
	if h.cfg.Message != "" {
		text += "\n\n" + h.cfg.Message
	}
	h.ephemeral(cmd, text)
}

func (h *SlashCommandHandler) handleMCPRevoke(cmd slack.SlashCommand) {
	if err := h.db.RevokeMCPToken(cmd.UserID); err != nil {
		slog.Error("failed to revoke MCP token", "error", err)
		h.ephemeral(cmd, "Sorry, I couldn't revoke your token. Please try again.")
		return
	}

	slog.Info("MCP token revoked", "user", cmd.UserName)
	h.ephemeral(cmd, "Your MCP token has been revoked. Use `/toad mcp connect` to generate a new one.")
}

func (h *SlashCommandHandler) handleMCPStatus(cmd slack.SlashCommand) {
	tok, err := h.db.GetMCPTokenByUser(cmd.UserID)
	if err != nil {
		slog.Error("failed to look up MCP token", "error", err)
		h.ephemeral(cmd, "Sorry, I couldn't check your token status.")
		return
	}

	if tok == nil {
		h.ephemeral(cmd, "You don't have an MCP token. Run `/toad mcp connect` to create one.")
		return
	}

	lastUsed := "never"
	if !tok.LastUsedAt.IsZero() {
		lastUsed = tok.LastUsedAt.Format(time.RFC3339)
	}

	text := fmt.Sprintf("*MCP Token Status*\n"+
		"• Role: *%s*\n"+
		"• Created: %s\n"+
		"• Last used: %s\n"+
		"• Endpoint: `%s://%s:%d/mcp`",
		tok.Role,
		tok.CreatedAt.Format(time.RFC3339),
		lastUsed,
		h.mcpScheme(), h.cfg.Host, h.cfg.Port,
	)
	h.ephemeral(cmd, text)
}

func (h *SlashCommandHandler) handleMCPPing(cmd slack.SlashCommand) {
	h.ephemeral(cmd, fmt.Sprintf("MCP server is running at `%s://%s:%d/mcp`", h.mcpScheme(), h.cfg.Host, h.cfg.Port))
}

func (h *SlashCommandHandler) handleMCPHelp(cmd slack.SlashCommand) {
	h.ephemeral(cmd, "*Toad MCP Commands*\n"+
		"• `/toad mcp connect` — Generate an MCP token for Claude Desktop/Code\n"+
		"• `/toad mcp revoke` — Revoke your MCP token\n"+
		"• `/toad mcp status` — Check your token and endpoint\n"+
		"• `/toad mcp ping` — Check if the MCP server is running")
}

// --- /toad joke ---

func (h *SlashCommandHandler) handleJoke(cmd slack.SlashCommand) {
	if h.agent == nil {
		h.post(cmd, "What do frogs do with paper? Rip-it.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := h.agent.Run(ctx, agent.RunOpts{
		Prompt: "Tell me a single original joke about frogs, toads, or amphibians. " +
			"Just the joke, nothing else. No quotes, no attribution, no preamble.",
		Model:       h.model,
		MaxTurns:    1,
		Permissions: agent.PermissionNone,
	})
	if err != nil {
		slog.Error("failed to generate joke", "error", err)
		h.post(cmd, "Why are frogs so happy? They eat whatever bugs them. :frog:")
		return
	}

	h.post(cmd, result.Result)
}

// --- /toad help ---

func (h *SlashCommandHandler) handleHelp(cmd slack.SlashCommand) {
	text := "*Toad Commands*\n" +
		"• `/toad status` — Daemon status, version, and stats\n"
	if h.cfg.Enabled {
		text += "• `/toad mcp connect` — Generate an MCP token\n" +
			"• `/toad mcp revoke` — Revoke your MCP token\n" +
			"• `/toad mcp status` — Check your MCP token\n" +
			"• `/toad mcp ping` — Check MCP server liveness\n"
	}
	text += "• `/toad joke` — Tell a frog joke\n" +
		"• `/toad help` — Show this message"
	h.ephemeral(cmd, text)
}

// --- helpers ---

func (h *SlashCommandHandler) mcpScheme() string {
	if h.cfg.Host == "localhost" || h.cfg.Host == "127.0.0.1" {
		return "http"
	}
	return "https"
}

func (h *SlashCommandHandler) post(cmd slack.SlashCommand, text string) {
	_, _, err := h.api.PostMessage(
		cmd.ChannelID,
		slack.MsgOptionText(text, false),
	)
	if err != nil {
		slog.Error("failed to send message", "error", err, "channel", cmd.ChannelID)
	}
}

func (h *SlashCommandHandler) ephemeral(cmd slack.SlashCommand, text string) {
	_, err := h.api.PostEphemeral(
		cmd.ChannelID,
		cmd.UserID,
		slack.MsgOptionText(text, false),
	)
	if err != nil {
		slog.Error("failed to send ephemeral response", "error", err, "user", cmd.UserID)
	}
}
