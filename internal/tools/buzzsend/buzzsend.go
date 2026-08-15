// Package buzzsend implements a domain.Tool wrapping the `buzz messages
// send` CLI command — the only way a NuimanBot ACP session's reply actually
// becomes a visible Buzz message. Buzz's own system prompt (bundled into
// every session/prompt call by the buzz-acp host) explicitly instructs the
// agent: "If your turn produced anything worth knowing, you MUST publish it.
// Use `buzz messages send`" — the plain ACP text response alone is not
// suffient; confirmed via a live production integration where every reply
// showed only as a transient "replying" status in Buzz's UI and never
// became a real, persisted message. Registered only for the ACP entrypoint
// (cmd/nuimanbot/acp.go) — the `buzz` CLI and its BUZZ_PRIVATE_KEY/
// BUZZ_AUTH_TAG/BUZZ_RELAY_URL credentials are only present in a
// buzz-acp-spawned process's environment, and this tool is meaningless on
// every other platform (Telegram, Slack, CLI, web Chats).
package buzzsend

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"nuimanbot/internal/domain"
)

// commandTimeout bounds how long a single `buzz messages send` invocation
// may run — generous for a relay round-trip, bounded so a hung CLI process
// can't wedge the tool-calling loop indefinitely.
const commandTimeout = 30 * time.Second

// Tool implements domain.Tool for publishing a message to the current Buzz
// channel/DM via the `buzz` CLI.
type Tool struct {
	config domain.ToolConfig
}

// New creates a new buzzsend Tool instance.
func New() *Tool {
	return &Tool{config: domain.ToolConfig{Enabled: true}}
}

// Name returns the tool name.
func (t *Tool) Name() string {
	return "buzz_send_message"
}

// Description returns a description of the tool.
func (t *Tool) Description() string {
	return "Publish a message to the current Buzz channel or DM by calling `buzz messages send`. " +
		"This is the only way your reply becomes visible — required at the end of any turn that produced " +
		"something worth reporting, per the system prompt's publish instructions."
}

// InputSchema returns the JSON schema for the tool's input parameters.
func (t *Tool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"channel": map[string]any{
				"type":        "string",
				"description": "The Buzz channel UUID to publish to — from the [Context] block's Channel line.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The message text to publish. GitHub-flavored Markdown is supported.",
			},
			"reply_to": map[string]any{
				"type":        "string",
				"description": "Optional: the event ID to reply to — from [Context]'s \"--reply-to <event-id>\" hint, when replying in an existing thread rather than posting a new top-level message.",
			},
			"mentions": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional: hex pubkeys or npubs to notify, matching any @Name mentions written in content.",
			},
		},
		"required": []string{"channel", "content"},
	}
}

// RequiredPermissions returns the permissions required to use this tool —
// deliberately none. Every Buzz contact who can reach the agent at all
// (including a first-message Guest, per chat.Service's
// defaultRoleForPlatform) needs this tool to work, or their conversation
// never produces a visible reply; gating it the way coding_agent gates
// PermissionShell (admin-only) would silently break replies for everyone
// but the bot's own admin. Its scope is narrow by construction instead —
// exactly one CLI subcommand, invoked with argv-separated arguments (no
// shell involved, so no command-injection surface regardless of content).
func (t *Tool) RequiredPermissions() []domain.Permission {
	return []domain.Permission{}
}

// Config returns the tool's configuration.
func (t *Tool) Config() domain.ToolConfig {
	return t.config
}

// Execute runs `buzz messages send` with the given parameters.
func (t *Tool) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	channel, _ := params["channel"].(string)
	if channel == "" {
		return &domain.ExecutionResult{Error: "missing or invalid 'channel' parameter"}, nil
	}
	content, _ := params["content"].(string)
	if content == "" {
		return &domain.ExecutionResult{Error: "missing or invalid 'content' parameter"}, nil
	}

	args := []string{"messages", "send", "--channel", channel}

	// Multiline content must go through stdin (--content -): per the buzz
	// CLI's own documented gotcha, a single-quoted shell string preserves
	// "\n" literally instead of a real newline byte. Not a concern for this
	// tool specifically (exec.CommandContext never invokes a shell), but
	// stdin is still the only way this CLI accepts real newline bytes in
	// content at all, so multiline replies require it regardless.
	useStdin := strings.Contains(content, "\n")
	if useStdin {
		args = append(args, "--content", "-")
	} else {
		args = append(args, "--content", content)
	}

	if replyTo, _ := params["reply_to"].(string); replyTo != "" {
		args = append(args, "--reply-to", replyTo)
	}
	if mentionsRaw, ok := params["mentions"].([]any); ok {
		for _, m := range mentionsRaw {
			if mention, ok := m.(string); ok && mention != "" {
				args = append(args, "--mention", mention)
			}
		}
	}

	cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "buzz", args...) // #nosec G204 -- "buzz" is a hardcoded literal, not attacker-controlled; args are passed argv-style (no shell involved), so no element of args can be reinterpreted as a shell command regardless of content
	if useStdin {
		cmd.Stdin = strings.NewReader(content)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &domain.ExecutionResult{
			Error: "buzz messages send failed: " + err.Error() + ": " + stderr.String(),
			Metadata: map[string]any{
				"exit_error": err.Error(),
				"stderr":     stderr.String(),
			},
		}, nil
	}

	return &domain.ExecutionResult{
		Output: stdout.String(),
		Metadata: map[string]any{
			"channel": channel,
		},
	}, nil
}
