package chat

import (
	"context"
	"html"
	"strings"

	"nuimanbot/internal/domain"
)

// convertSkillsToTools converts a list of skills to LLM tool definitions
func convertSkillsToTools(skills []domain.Tool) []domain.ToolDefinition {
	tools := make([]domain.ToolDefinition, 0, len(skills))

	for _, skill := range skills {
		tool := domain.ToolDefinition{
			Name:        skill.Name(),
			Description: skill.Description(),
			InputSchema: skill.InputSchema(),
		}
		tools = append(tools, tool)
	}

	return tools
}

// toolDefined reports whether name is among tools -- used to gate
// runToolLoop's publish-nudge on the publish tool actually being offered
// this turn, not just on platform (see processTurn's enforcePublish and
// buzzSendMessagePublishTool).
func toolDefined(tools []domain.ToolDefinition, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

// publishDestinationKey returns a stable key identifying where a
// buzz_send_message call would publish to, used by partitionPublishCalls to
// detect the model attempting to send more than one reply to the same
// destination within a single turn.
//
// Keyed on reply_to alone when present, not channel+reply_to: reply_to is a
// canonical Nostr event ID, but the model's channel value is not reliably
// consistent across calls in the same turn -- confirmed live, a single turn
// published the same content to the same reply_to twice because the first
// call used the channel UUID ("3a6a69fc-05a5-55cb-a601-0e12afc77c07") and
// the second used the channel's plain name ("general"), which an
// exact-string channel+reply_to key failed to recognize as the same
// destination. Falls back to channel for a top-level (non-reply) post,
// where no reply_to exists to key on.
func publishDestinationKey(args map[string]any) string {
	if replyTo, _ := args["reply_to"].(string); replyTo != "" {
		return "reply:" + replyTo
	}
	channel, _ := args["channel"].(string)
	return "channel:" + channel
}

// duplicatePublishSkipMessage is the synthetic tool result content returned
// in place of actually executing a redundant buzz_send_message call (see
// partitionPublishCalls) -- fed back to the model as this call's result so
// it understands the skip and doesn't keep retrying, rather than silently
// dropping it.
const duplicatePublishSkipMessage = "Skipped: a reply was already published to this destination earlier in this turn. " +
	"Sending another would be a duplicate — if you have something new to add, wait for the human's next message instead."

// partitionPublishCalls splits toolCalls into those that should actually
// execute and synthetic skip results for redundant buzz_send_message calls
// targeting a destination already claimed this turn. Guards a real
// production bug: a model that second-guesses itself mid-turn can otherwise
// publish two near-duplicate replies to the same DM/thread -- confirmed
// live, a single "say hello" turn published two differently-worded
// greetings to the same channel/reply_to 13 seconds apart, neither via
// runToolLoop's enforcePublish nudge (which only acts when a round has NO
// tool calls) but from the model voluntarily calling the tool twice across
// two ordinary rounds.
//
// claimed is mutated in place so same-round duplicates (the model
// requesting two buzz_send_message calls to the same destination in one
// response) are caught too, not just cross-round ones. A destination is
// claimed as soon as its first call is chosen for execution, regardless of
// whether that execution ultimately succeeds or errors -- an error already
// gets reported back to the model as a normal tool result and explained to
// the human in its own right, and does not need a second contradictory
// destination-claim policy layered on top.
func partitionPublishCalls(toolCalls []domain.ToolCall, claimed map[string]bool) (toExecute []domain.ToolCall, skipped []domain.ToolResult) {
	toExecute = make([]domain.ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		if tc.ToolName != buzzSendMessagePublishTool {
			toExecute = append(toExecute, tc)
			continue
		}
		key := publishDestinationKey(tc.Arguments)
		if claimed[key] {
			skipped = append(skipped, domain.ToolResult{
				ToolName: tc.ToolName,
				Output:   duplicatePublishSkipMessage,
			})
			continue
		}
		claimed[key] = true
		toExecute = append(toExecute, tc)
	}
	return toExecute, skipped
}

// executeToolCalls executes a list of tool calls on behalf of user and
// returns their results. Uses ExecuteWithUser (not the unchecked Execute) so
// RBAC, rate limiting, and audit logging are enforced for every platform
// (FR-011), and Part C's confirmation gate is applied, keyed on
// (user.ID, conversationID) (FR-001 fix — see
// specs/260803-improve-nuimanbot-security-auto-review).
func (s *Service) executeToolCalls(ctx context.Context, user *domain.User, conversationID string, toolCalls []domain.ToolCall) []domain.ToolResult {
	results := make([]domain.ToolResult, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		result, err := s.toolExecService.ExecuteWithUser(ctx, user, conversationID, toolCall.ToolName, toolCall.Arguments)

		toolResult := domain.ToolResult{
			ToolName: toolCall.ToolName,
		}

		switch {
		case err != nil:
			toolResult.Error = err.Error()
		case result.Error != "":
			// Skill returned an error in the result
			toolResult.Error = result.Error
		case result.Status == domain.StatusPendingConfirmation:
			// Part C (specs/260802-improve-nuimanbot-security, FR-013): the
			// tool call has been paused pending confirmation rather than
			// executed. Surfaced via Metadata (mirroring the existing
			// injection_flagged convention) rather than widening
			// domain.ToolResult's own fields, so ProcessMessage's tool loop
			// can end the turn immediately — see pendingConfirmationFrom.
			toolResult.Metadata = map[string]any{
				pendingConfirmationMetaFlag:    true,
				pendingConfirmationMetaID:      result.ConfirmationID,
				pendingConfirmationMetaSummary: result.Summary,
			}
		default:
			toolResult.Output = result.Output
			toolResult.Metadata = result.Metadata
		}

		results = append(results, toolResult)
	}

	return results
}

// formatToolResults formats tool results into a text representation for the
// LLM. Every result is wrapped in `<tool_output source="TOOLNAME">...
// </tool_output>` delimiters, uniformly across all tools and regardless of
// whether Phase 1's OutputValidator flagged/annotated the content, so the
// structural boundary is always present. This is defense-in-depth alongside
// Phase 1's pattern-matching and Phase 2's PromptComposer guardrail
// (specs/260802-improve-nuimanbot-security, FR-006).
//
// result.Output, result.Error, and result.ToolName are raw/lightly-processed
// content from third parties (web pages, search results, MCP servers) and
// must never be trusted to not contain the delimiter tags themselves.
// Without escaping, a tool result containing a literal "</tool_output>"
// could forge a premature close of the untrusted-data boundary Part B's
// guardrail depends on, and a tool name containing `"` could break out of
// the source="..." attribute (specs/260803-improve-nuimanbot-security-auto-
// review, FR-004/FR-R04). Each field is HTML-escaped via html.EscapeString
// before insertion: no bespoke escaping helper exists elsewhere in this
// codebase for plain-string content (the web admin UI's html/template
// templates auto-escape contextually at render time, not via a reusable
// string function - see research.md Q5), and html.EscapeString's standard
// `< > & " '` escaping is sufficient and simpler than a bespoke
// delimiter-specific escaper here. The resulting `&lt;`/`&gt;` sequences make
// "</tool_output>" structurally unreproducible by content between the tags,
// and escaped `"` (`&#34;`) makes the source attribute unbreakable, while
// leaving the underlying text intact and readable to the model.
func formatToolResults(results []domain.ToolResult) string {
	if len(results) == 0 {
		return "No tool results."
	}

	var formatted strings.Builder
	for i, result := range results {
		if i > 0 {
			formatted.WriteString("\n\n")
		}

		formatted.WriteString(`<tool_output source="`)
		formatted.WriteString(html.EscapeString(result.ToolName))
		formatted.WriteString("\">\n")

		if result.Error != "" {
			formatted.WriteString("Error: ")
			formatted.WriteString(html.EscapeString(result.Error))
		} else {
			formatted.WriteString("Result: ")
			formatted.WriteString(html.EscapeString(result.Output))
		}

		formatted.WriteString("\n</tool_output>")
	}

	return formatted.String()
}

// Metadata keys used to surface a pending-confirmation result (Part C,
// FR-013) from a domain.ToolResult through to ProcessMessage's tool loop.
// See executeToolCalls and pendingConfirmationFrom.
const (
	pendingConfirmationMetaFlag    = "pending_confirmation"
	pendingConfirmationMetaID      = "confirmation_id"
	pendingConfirmationMetaSummary = "confirmation_summary"
)

// pendingConfirmationFrom extracts the confirmation ID/summary a
// domain.ToolResult's Metadata carries when the underlying tool call was
// paused pending confirmation (see executeToolCalls). ok is false for any
// ordinary (non-pending) result.
func pendingConfirmationFrom(metadata map[string]any) (id string, summary string, ok bool) {
	if metadata == nil {
		return "", "", false
	}
	pending, isBool := metadata[pendingConfirmationMetaFlag].(bool)
	if !isBool || !pending {
		return "", "", false
	}
	id, _ = metadata[pendingConfirmationMetaID].(string)
	summary, _ = metadata[pendingConfirmationMetaSummary].(string)
	return id, summary, true
}

// pendingConfirmationEntry reports whether metadata marks its
// domain.ToolResult as the one carrying a pending-confirmation status (see
// pendingConfirmationFrom). Used by runToolLoop to skip recording that
// specific call's (empty) Output into collectedToolOutputs — its
// ID/Summary are surfaced separately via pendingConfirmationInfo — without
// affecting how any OTHER call in the same round is recorded (FR-010/FR-R10,
// specs/260803-improve-nuimanbot-security-auto-review).
func pendingConfirmationEntry(metadata map[string]any) bool {
	_, _, ok := pendingConfirmationFrom(metadata)
	return ok
}

// firstPendingConfirmation scans results for the first one carrying a
// pending-confirmation (see pendingConfirmationFrom) and returns its
// confirmation ID/summary. Used by runToolLoop to end the current turn
// immediately (Part C, FR-013) rather than continuing to iterate.
func firstPendingConfirmation(results []domain.ToolResult) (id string, summary string, ok bool) {
	for _, r := range results {
		if id, summary, ok := pendingConfirmationFrom(r.Metadata); ok {
			return id, summary, true
		}
	}
	return "", "", false
}

// isInjectionFlagged reports whether a domain.ToolResult's Metadata marks its
// content as flagged by OutputValidator (injection_flagged: true). Used to
// exclude flagged tool output from the input passed to
// MemoryCurator.ExtractMemoryCells, closing the second-order/stored-injection
// path (FR-005/FR-008 of specs/260802-improve-nuimanbot-security).
func isInjectionFlagged(metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	flagged, ok := metadata["injection_flagged"].(bool)
	return ok && flagged
}

// buildCacheKey creates a stable cache key from conversation messages.
// The key is a concatenation of all message roles and content.
func buildCacheKey(messages []domain.Message) string {
	var builder strings.Builder
	for i, msg := range messages {
		if i > 0 {
			builder.WriteString("\n---\n")
		}
		builder.WriteString(msg.Role)
		builder.WriteString(": ")
		builder.WriteString(msg.Content)
	}
	return builder.String()
}
