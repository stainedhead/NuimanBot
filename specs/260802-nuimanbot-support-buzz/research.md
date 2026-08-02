# Research: NuimanBot Support for Buzz

**Feature:** nuimanbot-support-buzz
**Created:** 2026-08-02
**Source PRD:** [nuimanbot-support-buzz-PRD.md](./nuimanbot-support-buzz-PRD.md) (§9 Open Questions)

## Research Questions

1. **[RESOLVED 2026-08-02, P0.1 spike] Which Nostr client/signing library should NuimanBot depend on?**
   Options per PRD §6.1/§9:
   - Hand-rolled minimal NIP-01 client (full control, more code to maintain and audit).
   - `github.com/nbd-wtf/go-nostr` (full Nostr client library — event handling, relay pool, NIPs support).
   - `github.com/btcsuite/btcd/btcec` for the secp256k1 signing primitive only, paired with a hand-rolled or minimal NIP-01 event layer.
   Evaluation criteria: library maturity/audit status, dependency weight, fit with Clean Architecture layering (infrastructure-only concern), maintenance activity, license compatibility (project is presumably permissively licensed given Apache-2.0 Buzz itself). This is the single blocking research item for Phase 1 — resolve via a short spike before any Phase 1 implementation task begins.

   **Decision: option 3 — `github.com/btcsuite/btcd/btcec/v2` + `github.com/btcsuite/btcd/btcec/v2/schnorr` for the secp256k1/BIP-340 Schnorr signing primitive, paired with a hand-rolled NIP-01 event/relay layer in `internal/infrastructure/nostr/`, over `github.com/gorilla/websocket` (promoted indirect→direct) for the WebSocket transport.**

   Rationale (verified 2026-08-02 via GitHub/pkg.go.dev):
   - `github.com/nbd-wtf/go-nostr` (MIT) is now **archived and in maintenance mode** (per its own README, as of Jan 2026): "This repository is in maintenance mode and adventurous programmers are encouraged to try `fiatjaf.com/nostr@master` instead." Taking on an archived dependency for new production code is a red flag, and the recommended successor is an unversioned `@master` import (no semver tags) — too unstable to pin for this project.
   - `github.com/btcsuite/btcd/btcec/v2/schnorr` implements BIP-340 Schnorr signatures over secp256k1 with 32-byte x-only public keys — exactly the signature scheme NIP-01 requires (Nostr event IDs/sigs are BIP-340 Schnorr, not ECDSA). btcsuite/btcd is a mature, actively maintained, widely-audited Bitcoin-ecosystem library (ISC license, permissive) — a much safer place to source the actual cryptographic primitive than a hand-rolled Schnorr implementation, which is easy to get subtly wrong.
   - The NIP-01 event layer itself (canonical JSON serialization for ID computation, event struct, `REQ`/`EVENT`/`CLOSE`/`EOSE` relay message shapes) is small (a few hundred lines) and fully within our control/audit surface — hand-rolling it avoids pulling in a general-purpose Nostr client library's much larger surface (relay pools, NIP-05, NIP-44 encryption, etc.) that Phase 1 doesn't need, and keeps the Nostr protocol concern cleanly confined to `internal/infrastructure/nostr/` per architecture.md's swappability requirement.
   - `gorilla/websocket` was already an indirect dependency (v1.5.3) per spec.md's Scope of Changes; promoting it to direct for `client.go`'s relay connections avoids adding a second WebSocket library.

   `go get github.com/btcsuite/btcd/btcec/v2` run as part of P0.1; `gorilla/websocket` promoted to direct via its use in `client.go` (P1.4) plus `go mod tidy`.

2. **What is the exact loop-prevention heuristic?**
   Time-window based (don't auto-reply to agent-authored messages within N seconds/minutes of this agent's own reply) vs. event-tag based (rely on Buzz's own agent-identity tagging mechanism) vs. content-based detection. Needs to be finalized during Phase 2 implementation with test coverage for a simulated N-message runaway exchange terminating rather than running indefinitely (see spec.md Phase 2 exit criteria).

3. **Does Buzz's "approved automations" concept require a new permission tier beyond `RoleGuest`/`RoleUser`/`RoleAdmin`, or does it map cleanly onto existing tool RBAC?**
   Relevant to Phase 3 (FR-010). Current assumption per PRD §6.5/§6.7 is that existing RBAC + rate-limiting + audit pipeline applies unchanged, with no bypass for Buzz-originated requests. Needs confirmation once Buzz's automation-approval protocol details are better understood (Buzz repo is open-source; may require reading its source/docs directly).

4. **Multi-relay consistency: is best-effort fanout sufficient, or does NuimanBot need to de-duplicate the same event arriving from multiple relays?**
   PRD leans "likely yes — dedupe by event ID" (already captured as FR-004 / NFR reliability requirement). Confirm dedup strategy (in-memory LRU of recent event IDs vs. persistent store) is sufficient in a multi-relay, possibly multi-process deployment — single-process assumption should be validated.

5. **[RESOLVED 2026-08-02, P0.2 spike] What NIP-01 filter fields and event kind(s) does Buzz actually require for channel-message and channel-subscription semantics?**
   Buzz uses Nostr as its transport but channels/threads/DMs are Buzz-specific concepts layered on top of raw Nostr events (kinds, tags). Needs a read of Buzz's protocol spec/source (referenced as open-source in PRD §2) to confirm the event `Kind` used for channel messages, the channel-ID tag name, and the agent-identity tag name/format — before `event.go` (Kind field, P1.1), `subscription.go` (filter tag names, P1.3), and the agent-tagging path (P2.2) can be implemented precisely rather than guessed. This is a second hard blocker for Phase 1, alongside Q1 — both are now scoped as P0 spikes in tasks.md (P0.1 and P0.2) rather than deferred into a later task's "refactor" step.

   **Decision — verified 2026-08-02 directly against `github.com/block/buzz` (main branch, live/actively pushed repo, Apache-2.0) via GitHub API/raw file fetch, not just secondary sources:**

   - **Channel message event kind: `9`** — `KIND_STREAM_MESSAGE` (`crates/buzz-core/src/kind.rs`), doc comment: "NIP-29 group chat message kind. V1 used kind:10001 (replaceable range — wrong), then 40001." i.e. kind 9 is the current, correct value; source also notes an agent-shutdown convention (owner sends `kind:9` with content `"!shutdown"` + `#p` tag) that reuses the same kind, not a new one.
   - **Channel-ID tag: `h`** — confirmed both in `kind.rs` comments and directly in `NOSTR.md`: "Send/receive messages with `#h <channel-uuid>` tag" and "kind:9 requires `#h` tag. Messages without a channel-scoped `#h` tag are rejected." This matches NIP-29's group tag convention (not a Buzz-specific tag name).
   - **Agent-identity marker: there is no per-message tag.** Read `crates/buzz-core/src/channel.rs` and `kind.rs` directly: Buzz distinguishes agents from humans via **channel membership role**, not an event tag on the `kind:9` message itself. `MemberRole` (channel.rs) is `Owner | Admin | Member | Guest | Bot` — "Bot is a **separate designation** — it is not part of the linear hierarchy," canonical string `"bot"`, set via NIP-29 admin events (`KIND_NIP29_PUT_USER = 9000` carrying a role tag) and readable from the channel's membership list, not from the message event. Separately, `KIND_AGENT_PROFILE = 10100` ("Agent metadata + owner reference (replaceable, agent-authored)") is a per-pubkey profile event an agent publishes about itself — a second, pubkey-scoped (not message-scoped) signal.
   - **Consequence for `sender_is_agent` (data-dictionary.md, FR-005/FR-009):** accurately populating this field requires either (a) tracking NIP-29 membership-role state per channel (subscribing to/replaying `kind:9000`-series admin events) or (b) checking for a `kind:10100` profile event for the sender pubkey — both are out of scope for the Phase 1 infrastructure tasks as scoped in tasks.md (P1.1–P1.4 build event/verify/subscription/client for `kind:9` messages only; no task covers membership-list or profile-kind subscription/tracking). **Scoping decision for Phase 1 (P1.7):** `sender_is_agent` is populated as a best-effort `false` default with no membership tracking implemented yet — this is an accurate reflection of what Phase 1's scoped infrastructure can determine from the message stream alone, not a fabricated protocol assumption. Real membership-role tracking (or profile-kind lookup) is a documented follow-up for whichever phase actually consumes `sender_is_agent` for a behavioral decision — Phase 2's loop-prevention guard (FR-009, P2.3) is the first consumer and is out of this agent's scope; flagging this explicitly so P2.3's implementer does not assume the boolean is already accurate.
   - Sources: `crates/buzz-core/src/kind.rs`, `crates/buzz-core/src/channel.rs`, `NOSTR.md`, `AGENTS.md` — all fetched directly from `github.com/block/buzz` main branch, 2026-08-02.

6. **[RESOLVED 2026-08-02] Does Buzz-triggered tool execution (FR-010, Phase 3) get genuine RBAC/rate-limit enforcement, or does it inherit an existing gap present for all platforms?**
   Verified against the current codebase during spec review (2026-08-02): `usecase/chat/tool_conversion.go:31` calls `toolExecService.Execute()` — the *unchecked* execution path. The RBAC/rate-limit-checked path, `tool.Service.ExecuteWithUser()` (`internal/usecase/tool/service.go:119`, which calls `checkPermission` + rate limiting + audit), is **not** currently invoked anywhere in the chat-message pipeline (`ChatService.ProcessMessage` → tool conversion), for Telegram, Slack, or CLI-via-skill-handler. `ListTools` (`internal/usecase/tool/service.go:261`) also ignores the `userID` parameter entirely (marked `TODO: Implement user-specific tool filtering`).

   **Decision (product owner, 2026-08-02): option (a) — fix it for all platforms as part of Phase 3.** `ChatService`'s tool-invocation path is rewired to `tool.Service.ExecuteWithUser()`, and `ListTools`' role-based filtering TODO is implemented, for every platform — not scoped to Buzz-originated requests only. This makes the Phase 3 exit criterion and PRD §6.7's security narrative literally true rather than aspirational, at the cost of a real (and out-of-Buzz-scope-on-paper, but necessary) behavior change: existing Telegram/Slack/CLI users may see tool availability change if their role lacks permission for a tool they could previously invoke unchecked. This is a deliberate, accepted side effect — not a bug to work around. See spec.md Phase 3 (FR-010 / new FR-011, FR-012) and tasks.md P3.1–P3.3 for the resulting task breakdown, which now includes existing-platform regression test coverage alongside the Buzz case.

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

Questions 1 and 5 are **resolved** (2026-08-02, P0.1/P0.2 spikes) — see above. Questions 2, 3, and 4 remain implementation-time decisions for Phase 2/3 respectively (out of this agent's Phase 0+1 scope). Question 6 is resolved (2026-08-02) — see above; Phase 3 now includes an all-platforms RBAC-enforcement fix, not just Buzz.

## References

- Source PRD: [nuimanbot-support-buzz-PRD.md](./nuimanbot-support-buzz-PRD.md)
- Nostr protocol: [nostr.com](https://nostr.com/)
- `github.com/btcsuite/btcd/btcec/v2` + `github.com/btcsuite/btcd/btcec/v2/schnorr` (chosen, P0.1)
- `github.com/nbd-wtf/go-nostr` (evaluated, rejected — archived/maintenance-mode)
- `github.com/block/buzz` — `crates/buzz-core/src/kind.rs`, `crates/buzz-core/src/channel.rs`, `NOSTR.md`, `AGENTS.md` (protocol source of truth, P0.2)
