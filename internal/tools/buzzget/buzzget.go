// Package buzzget implements a domain.Tool wrapping the `buzz messages get`
// CLI command — a real way for an ACP session to read a Buzz channel's
// message history, as opposed to the model only assuming the capability
// exists from Buzz's own system prompt (which documents a much richer CLI
// than NuimanBot actually exposes as callable tools). Confirmed live: asked
// to recall channel history, the model fabricated having run `buzz messages
// get`/`buzz feed get` and gotten a permissions error — it had no real tool
// to call at all, so it narrated a plausible-sounding failure instead of
// admitting it lacked the capability. This tool closes that specific gap.
// Registered only for the ACP entrypoint (cmd/nuimanbot/acp.go), matching
// buzzsend — the `buzz` CLI is only present in a buzz-acp-spawned process's
// environment.
package buzzget

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"nuimanbot/internal/domain"
)

// commandTimeout bounds how long a single `buzz messages get` invocation
// may run — generous for a relay round-trip, bounded so a hung CLI process
// can't wedge the tool-calling loop indefinitely.
const commandTimeout = 30 * time.Second

// Tool implements domain.Tool for reading a Buzz channel's message history
// via the `buzz` CLI.
type Tool struct {
	config domain.ToolConfig
}

// New creates a new buzzget Tool instance.
func New() *Tool {
	return &Tool{config: domain.ToolConfig{Enabled: true}}
}

// Name returns the tool name.
func (t *Tool) Name() string {
	return "buzz_get_messages"
}

// Description returns a description of the tool.
func (t *Tool) Description() string {
	return "Retrieve recent messages from a Buzz channel by calling `buzz messages get`. " +
		"Use this to recall what was actually said earlier in a channel or DM instead of guessing."
}

// InputSchema returns the JSON schema for the tool's input parameters.
func (t *Tool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"channel": map[string]any{
				"type":        "string",
				"description": "The Buzz channel UUID to read — from the [Context] block's Channel line.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Optional: maximum number of messages to return.",
			},
			"before": map[string]any{
				"type":        "integer",
				"description": "Optional: Unix timestamp — return messages before this time.",
			},
			"since": map[string]any{
				"type":        "integer",
				"description": "Optional: Unix timestamp — return messages after this time.",
			},
			"kinds": map[string]any{
				"type":        "string",
				"description": "Optional: comma-separated Nostr event kinds to filter (e.g. \"1,1984\").",
			},
		},
		"required": []string{"channel"},
	}
}

// RequiredPermissions returns the permissions required to use this tool —
// deliberately none, matching buzz_send_message: any Buzz contact who can
// reach Iman should be able to prompt it to recall channel context,
// including a first-message Guest.
func (t *Tool) RequiredPermissions() []domain.Permission {
	return []domain.Permission{}
}

// Config returns the tool's configuration.
func (t *Tool) Config() domain.ToolConfig {
	return t.config
}

// intParam extracts an optional integer-valued parameter. LLM tool-call
// arguments decode numbers as float64 (see domain.ToolCall.Arguments); ok is
// false when the key is absent or not numeric.
func intParam(params map[string]any, key string) (int64, bool) {
	v, ok := params[key].(float64)
	if !ok {
		return 0, false
	}
	return int64(v), true
}

// Execute runs `buzz messages get` with the given parameters.
func (t *Tool) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	channel, _ := params["channel"].(string)
	if channel == "" {
		return &domain.ExecutionResult{Error: "missing or invalid 'channel' parameter"}, nil
	}

	args := []string{"messages", "get", "--channel", channel}

	if limit, ok := intParam(params, "limit"); ok {
		args = append(args, "--limit", fmt.Sprintf("%d", limit))
	}
	if before, ok := intParam(params, "before"); ok {
		args = append(args, "--before", fmt.Sprintf("%d", before))
	}
	if since, ok := intParam(params, "since"); ok {
		args = append(args, "--since", fmt.Sprintf("%d", since))
	}
	if kinds, _ := params["kinds"].(string); kinds != "" {
		args = append(args, "--kinds", kinds)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "buzz", args...) // #nosec G204 -- "buzz" is a hardcoded literal, not attacker-controlled; args are passed argv-style (no shell involved), so no element of args can be reinterpreted as a shell command regardless of content
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &domain.ExecutionResult{
			Error: "buzz messages get failed: " + err.Error() + ": " + stderr.String(),
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
