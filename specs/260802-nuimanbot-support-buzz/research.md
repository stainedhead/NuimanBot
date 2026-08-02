# Research: NuimanBot Support for Buzz

**Feature:** nuimanbot-support-buzz
**Created:** 2026-08-02
**Source PRD:** [nuimanbot-support-buzz-PRD.md](./nuimanbot-support-buzz-PRD.md) (§9 Open Questions)

## Research Questions

1. **[SPIKE — resolve before Phase 1 coding] Which Nostr client/signing library should NuimanBot depend on?**
   Options per PRD §6.1/§9:
   - Hand-rolled minimal NIP-01 client (full control, more code to maintain and audit).
   - `github.com/nbd-wtf/go-nostr` (full Nostr client library — event handling, relay pool, NIPs support).
   - `github.com/btcsuite/btcd/btcec` for the secp256k1 signing primitive only, paired with a hand-rolled or minimal NIP-01 event layer.
   Evaluation criteria: library maturity/audit status, dependency weight, fit with Clean Architecture layering (infrastructure-only concern), maintenance activity, license compatibility (project is presumably permissively licensed given Apache-2.0 Buzz itself). This is the single blocking research item for Phase 1 — resolve via a short spike before any Phase 1 implementation task begins.

2. **What is the exact loop-prevention heuristic?**
   Time-window based (don't auto-reply to agent-authored messages within N seconds/minutes of this agent's own reply) vs. event-tag based (rely on Buzz's own agent-identity tagging mechanism) vs. content-based detection. Needs to be finalized during Phase 2 implementation with test coverage for a simulated N-message runaway exchange terminating rather than running indefinitely (see spec.md Phase 2 exit criteria).

3. **Does Buzz's "approved automations" concept require a new permission tier beyond `RoleGuest`/`RoleUser`/`RoleAdmin`, or does it map cleanly onto existing tool RBAC?**
   Relevant to Phase 3 (FR-010). Current assumption per PRD §6.5/§6.7 is that existing RBAC + rate-limiting + audit pipeline applies unchanged, with no bypass for Buzz-originated requests. Needs confirmation once Buzz's automation-approval protocol details are better understood (Buzz repo is open-source; may require reading its source/docs directly).

4. **Multi-relay consistency: is best-effort fanout sufficient, or does NuimanBot need to de-duplicate the same event arriving from multiple relays?**
   PRD leans "likely yes — dedupe by event ID" (already captured as FR-004 / NFR reliability requirement). Confirm dedup strategy (in-memory LRU of recent event IDs vs. persistent store) is sufficient in a multi-relay, possibly multi-process deployment — single-process assumption should be validated.

5. **[SPIKE — resolve before Phase 1 coding, see tasks.md P0.2] What NIP-01 filter fields and event kind(s) does Buzz actually require for channel-message and channel-subscription semantics?**
   Buzz uses Nostr as its transport but channels/threads/DMs are Buzz-specific concepts layered on top of raw Nostr events (kinds, tags). Needs a read of Buzz's protocol spec/source (referenced as open-source in PRD §2) to confirm the event `Kind` used for channel messages, the channel-ID tag name, and the agent-identity tag name/format — before `event.go` (Kind field, P1.1), `subscription.go` (filter tag names, P1.3), and the agent-tagging path (P2.2) can be implemented precisely rather than guessed. This is a second hard blocker for Phase 1, alongside Q1 — both are now scoped as P0 spikes in tasks.md (P0.1 and P0.2) rather than deferred into a later task's "refactor" step.

6. **[OPEN — flagged during spec review, not resolved here] Does Buzz-triggered tool execution (FR-010, Phase 3) get genuine RBAC/rate-limit enforcement, or does it inherit an existing gap present for all platforms?**
   Verified against the current codebase during spec review (2026-08-02): `usecase/chat/tool_conversion.go:31` calls `toolExecService.Execute()` — the *unchecked* execution path. The RBAC/rate-limit-checked path, `tool.Service.ExecuteWithUser()` (`internal/usecase/tool/service.go:119`, which calls `checkPermission` + rate limiting + audit), is **not** currently invoked anywhere in the chat-message pipeline (`ChatService.ProcessMessage` → tool conversion), for Telegram, Slack, or CLI-via-skill-handler. `ListTools` (`internal/usecase/tool/service.go:261`) also ignores the `userID` parameter entirely (marked `TODO: Implement user-specific tool filtering`). This means the Phase 3 exit criterion "under the existing RBAC + rate-limiting pipeline, with no bypass for Buzz-originated requests" (spec.md) is, as of this review, true only in the degenerate sense that Buzz would be exactly as unenforced as every other platform.
   **This needs a product/architecture decision before P3.1 begins:**
   - (a) Phase 3 also wires `ChatService`'s tool-invocation path to `ExecuteWithUser` for all platforms — closes the gap, but changes existing Telegram/Slack/CLI behavior and is a larger scope than "Buzz gateway integration."
   - (b) Buzz explicitly inherits the current non-enforcement for this initial implementation, and the security narrative in PRD §6.7 / spec.md Goals ("treating other agents as a potentially adversarial input source") is understood to apply only to `ValidateInput()` (which *is* correctly wired and unchanged for Buzz), not to tool-execution RBAC, until a follow-up spec closes this gap.
   Not resolved in this spec-review pass — left here for the implementer/team lead to decide before Phase 3 tasks begin.

## Industry Standards

[TBD — Nostr NIP-01 (event/relay protocol) and NIP-05 (identifier verification) are the relevant standards; see nostr.com and the NIPs repo for full spec once the spike (Q1) begins.]

## Existing Implementations

- `internal/adapter/gateway/slack/gateway.go` — reference pattern for `domain.Gateway` implementation (Socket Mode connect/reconnect, event handling loop, message-handler dispatch).
- `internal/adapter/gateway/telegram/` — second reference pattern, particularly for `DMPolicy` handling.
- `internal/infrastructure/crypto/vault.go`, `internal/infrastructure/crypto/versioned_vault.go` — existing AES-256-GCM `domain.SecureString` vault, to be extended with a generate-if-absent helper rather than a new subsystem.

## API Documentation

[TBD — pending Nostr library spike (Q1) and Buzz protocol read (Q5).]

## Best Practices

[TBD]

## Open Questions

See Research Questions above (all six are currently open). Questions 1 and 5 are hard blockers for Phase 1 (both scoped as P0 spikes — tasks.md P0.1, P0.2). Question 6 is a design decision that must be made before Phase 3 (P3.1) begins, but does not block Phase 1/2.

## References

- Source PRD: [nuimanbot-support-buzz-PRD.md](./nuimanbot-support-buzz-PRD.md)
- Nostr protocol: [nostr.com](https://nostr.com/)
- `github.com/nbd-wtf/go-nostr`
- `github.com/btcsuite/btcd/btcec`
