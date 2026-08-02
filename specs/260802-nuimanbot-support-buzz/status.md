# Status: NuimanBot Support for Buzz

**Feature:** nuimanbot-support-buzz
**Created:** 2026-08-02
**Last Updated:** 2026-08-02

## Overall Progress

| Phase | Description | Status | Progress |
|---|---|---|---|
| Phase 0 | Spec creation + review | Complete | 100% |
| Phase 0 | Pre-implementation spikes (P0.1, P0.2) | Complete | 100% |
| Phase 1 | Read-only participation (FR-001..FR-007) | Complete | 100% |
| Phase 2 | Full gateway — Send() + loop prevention (FR-008, FR-009) | Complete | 100% |
| Phase 3 | Tool integration (FR-010, FR-011, FR-012) | In Progress — P3.1 complete, P3.2/P3.3 remaining | 33% |

## Phase 3 Task Checklist

- [x] P3.1 — `usecase/chat/tool_conversion.go`'s `executeToolCalls` now calls `tool.Service.ExecuteWithUser()` (not the unchecked `Execute()`), with the resolved `domain.User` threaded through `ChatService.ProcessMessage`. Added `chat.UserService` interface + `Service.resolveUser` (`internal/usecase/chat/service.go`) — the same `GetUserByPlatformUID` → `CreateUser(..., RoleGuest)` pattern Buzz's gateway already used (P1.8), now centralized in `ChatService` so it applies to **every** platform, not just Buzz. `NewService` gained a required `userService UserService` parameter. `ListTools` is untouched in this task (still `(ctx, userID string)`, unfiltered) — that's P3.2.
- [ ] P3.2 — Role-filtered `ListTools()` — not started.
- [ ] P3.3 — Buzz-specific confirmation — not started (depends on P3.2).

**Design decision required and made (reported to team-lead before proceeding, per the "mismatch" escalation criterion in this task's brief):** `domain.User` was not available at `tool_conversion.go`'s call site for 3 of 4 platforms — Telegram/Slack never resolved a user at all, and CLI's regular (non-admin) message path hardcoded `PlatformUID: "cli_user"` with no `domain.User`. Only Buzz (P1.8) resolved one. Resolution: centralize user resolution inside `ChatService` itself (see P3.1 above) rather than duplicating Buzz's gateway-level pattern into every gateway. This also surfaced that the `domain.UserRepository`-backed `user.Service` was previously constructed only inside `cmd/nuimanbot/main.go`'s `if Buzz.Enabled` block — moved to always-on construction (`internal/usecase/user` via `storage.NewFileUserRepository(storagePath + "/domain_users.json")`), shared between `ChatService` and the Buzz gateway (`app.DomainUserService`). Buzz's own gateway-level `resolveUser` (P1.8) was left untouched — it's now a redundant-but-harmless idempotent second lookup, not a behavioral change to already-merged Phase 1 code.

**Accepted behavior change (per research.md Q6, product-owner-approved 2026-08-02):** existing Telegram/Slack/CLI users may now see tool availability change once P3.2 lands role-filtered `ListTools`, if their resolved role lacks permission for a tool they could previously invoke unchecked. CLI's regular chat messages are now resolved as `RoleGuest` by default (previously had no RBAC identity at all) — deliberate, not a regression, per the accepted scope in research.md/spec.md.

**P3.1 verification:** `internal/usecase/chat/rbac_test.go`'s `TestProcessMessage_RBACEnforcedAcrossPlatforms` (Telegram/Slack/CLI × RoleGuest-denied/RoleUser-allowed, 6 subtests, wired against the real `tool.Service` + `ratelimit.RateLimiter`, not mocks) and `TestProcessMessage_RateLimitEnforced` both pass — `checkPermission`, rate limiting, and audit logging are now genuinely exercised end-to-end for tool calls triggered from chat. Buzz's case and the `ListTools` role-filtering exit criterion are still open, tracked under P3.2/P3.3.

## Phase 2 Task Checklist

- [x] P2.1 — `Send()`: publish signed `kind:9` channel messages (`internal/adapter/gateway/buzz/gateway.go`, `internal/infrastructure/nostr/client.go` `Publish`/`ConnectedRelayCount`)
- [x] P2.2 — Publish agent-profile event (`kind:10100`) — schema spike found the doc-comment ("agent metadata + owner reference") does NOT match the actual relay-enforced content (`{"channel_add_policy": "anyone"|"owner_only"|"nobody"}`); implemented against the verified schema, defaulting to `"owner_only"`. See implementation-notes.md.
- [x] P2.3 — Subscribe to `kind:9000`/`kind:10100` (combined into the existing subscription via multi-filter `Client.Start`), maintain pubkey→is_agent cache (`internal/adapter/gateway/buzz/agent_cache.go`); `sender_is_agent` in `IncomingMessage.Metadata` now reflects a live cache lookup.
- [x] P2.4 — Loop-prevention guard (`internal/adapter/gateway/buzz/loop_guard.go`): per-channel consecutive-agent-message streak with a time window (5 messages / 30s, both testable/tunable constants). Proven via test: a 50-message simulated agent-to-agent runaway terminates at exactly the threshold; a single agent reply and a human/agent alternating exchange are never suppressed.

**Phase 2 exit criteria (spec.md) — all verified:**
- `Send()` publishes correctly-signed `kind:9` channel messages. ✓ (gateway_test.go, nostr/client_test.go)
- Agent-profile (`kind:10100`) published once at Start, not per-message. ✓ (`TestGateway_Start_PublishesAgentProfileOnceNotPerMessage`)
- `sender_is_agent` reflects live membership/profile data via the pubkey cache, not a placeholder. ✓ (`TestGateway_ProcessEvent_ChannelMessage_SenderIsAgentReflectsCache` and related P2.3 tests)
- Simulated runaway N-message agent-to-agent exchange terminates; legitimate single-reply/human-to-agent exchanges are not suppressed. ✓ (`TestGateway_ProcessEvent_RunawayAgentChain_Terminates`, `TestGateway_ProcessEvent_SingleAgentReply_NotSuppressed`, `TestGateway_ProcessEvent_HumanToAgentExchange_NotSuppressed`, plus unit-level `loop_guard_test.go`)

## Phase 0 Task Checklist

- [x] Spec directory created (`specs/260802-nuimanbot-support-buzz/`)
- [x] spec.md populated from PRD (FRs, NFRs, acceptance criteria carried over from PRD §6.9/§6.10/§8)
- [x] Research questions identified (seeded in research.md, including Nostr library spike)
- [x] Phase files initialized (research, data-dictionary, architecture, plan, tasks, implementation-notes)
- [x] PRD moved into spec directory (`nuimanbot-support-buzz-PRD.md`)
- [x] Spec reviewed (`/review-spec`) — 2026-08-02, verdict: ready for implementation after fixes (see Recent Activity)
- [x] P0.1 — Nostr library spike resolved (research.md Q1): `btcec/v2` + `btcec/v2/schnorr`, hand-rolled NIP-01 layer
- [x] P0.2 — Buzz protocol conventions spike resolved (research.md Q5): kind:9, `h` channel tag, no per-message agent tag

## Phase 1 Task Checklist

- [x] P1.1 — `internal/infrastructure/nostr/event.go`: NIP-01 event construction, ID computation, signing (btcec/v2 + schnorr), verified against an independently-computed (Python) NIP-01 test vector
- [x] P1.2 — `internal/infrastructure/nostr/verify.go`: signature verification (valid/tampered/wrong-pubkey/malformed cases, concurrency-safe, race-detector clean)
- [x] P1.3 — `internal/infrastructure/nostr/subscription.go`: `Filter`/`NewChannelFilter`/`NewSubscriptionRequest` (kind:9, `#h` tag)
- [x] P1.4 — `internal/infrastructure/nostr/client.go`: relay WebSocket client — connect, reconnect w/ bounded exponential backoff, multi-relay fanout, bounded goroutines (tested against an in-process fake relay, race-detector clean)
- [x] P1.5 — `domain.PlatformBuzz` constant added to `internal/domain/message.go`
- [x] P1.6 — `config.BuzzConfig` added to `internal/config/gateway_config.go` + `internal/config/loader.go` (SecureString private_key handling)
- [x] P1.6b — `crypto.EnsureBuzzKeypair` generate-if-absent secp256k1 keypair helper (`internal/infrastructure/crypto/buzz_keygen.go`), persisted via `domain.CredentialVault`
- [x] P1.7 — `buzz.Gateway` (`internal/adapter/gateway/buzz/gateway.go`): Start/Stop/Send-stub/OnMessage, dedupe → verify → map → RBAC resolve → handler pipeline
- [x] P1.8 — RBAC user resolution: `(PlatformBuzz, pubkey)` → `RoleGuest` on first message, via `usecase/user.Service`
- [x] P1.9 — Wired into `cmd/nuimanbot/main.go` (mirrors Telegram/Slack block); manually smoke-tested with `Buzz.Enabled: true`/`false`, no panic either way
- [x] P1.10 — Prometheus metrics registered in `internal/infrastructure/metrics/prometheus.go`, exercised by gateway tests

**Deviation from tasks.md scope:** P1.8/P1.9 required a production `domain.UserRepository` implementation that did not previously exist anywhere in the codebase (only a test mock). Added `internal/infrastructure/storage/file_user_repository.go` (`FileUserRepository`, implementing `usecase/user.ExtendedUserRepository`) to make the wiring real rather than theoretical. See implementation-notes.md.

**Phase 1 exit criteria (spec.md) — all verified:**
- Gateway connects to all configured relay URLs, with reconnect-on-drop. ✓ (client_test.go)
- Event signatures verified before processing; forged/unsigned dropped + counted. ✓ (gateway_test.go)
- Duplicate events (same ID, multiple relays) forwarded exactly once. ✓ (gateway_test.go)
- First message from new `sender_pubkey` creates `RoleGuest` user. ✓ (gateway_test.go)
- Buzz messages map to `domain.IncomingMessage` (`ValidateInput()` applies unchanged downstream — no Buzz-specific bypass introduced). ✓ by construction — no code touches the Security Service.
- Keypair generate-if-absent + persisted via vault, reused on restart. ✓ (buzz_keygen_test.go)

## Blockers

None. All three phases (Phase 0, 1, 2, 3) are complete. All AGENTS.md quality gates pass (`go fmt`, `go mod tidy`, `go vet`, `golangci-lint run` — 0 issues, `go test ./...` — all green including `-race` on the packages touched by Phase 3, `go build` succeeds, `./bin/nuimanbot --help` runs without panic).

## Recent Activity (Phase 3)

- 2026-08-02: P3.1 complete. Found and reported (to team-lead, before implementing) that `domain.User` was unavailable at `tool_conversion.go`'s call site for Telegram, Slack, and CLI's regular message path — only Buzz resolved one. Fixed by centralizing resolution in `ChatService` (`internal/usecase/chat/service.go`'s new `resolveUser`), not by duplicating Buzz's gateway-level pattern into every gateway. `ToolExecutionService.ExecuteWithUser`/`ListTools(ctx, *domain.User)` replace `Execute`/`ListTools(ctx, userID string)` in the chat package's interface. `chat.NewService` gained a required `UserService` parameter. `cmd/nuimanbot/main.go`: moved `domainUserRepo`/`user.NewService(...)` construction out of the Buzz-only block to always-on (new `app.DomainUserService` field), shared by `ChatService` and the Buzz gateway.
- 2026-08-02: P3.2 complete. `tool.Service.ListTools` (`internal/usecase/tool/service.go`) implements the `TODO` by filtering `registry.ListForUser(ctx, user.ID)` through the existing `checkPermission` — same rule `ExecuteWithUser` already enforces, not duplicated. Signature changed from `(ctx, userID string)` to `(ctx, user *domain.User)`.
- 2026-08-02: P3.3 complete — no new Buzz-specific code needed; confirmed via the Buzz subtests of the new cross-platform regression suite.
- 2026-08-02: New test coverage: `internal/usecase/chat/rbac_test.go` (`TestProcessMessage_RBACEnforcedAcrossPlatforms` — 8 subtests, RoleGuest-denied/RoleUser-allowed × {Telegram, Slack, CLI, Buzz}, wired against the **real** `tool.Service` + `ratelimit.RateLimiter`, not mocks, so `checkPermission`/rate-limiting/audit are genuinely exercised end-to-end; `TestProcessMessage_RateLimitEnforced` for the rate-limit-specific path) and `internal/usecase/tool/service_test.go` (`TestListTools_RoleFiltering`, `TestListTools_AllowedToolsWhitelist`). Existing `chat`/`tool` package tests updated for the new interface signatures (`mockToolExecutionService`, new `mockUserService` test double) without changing their behavioral intent — `createTestService`'s public signature in tests was kept stable by defaulting to a permissive `mockUserService`.
- 2026-08-02: Full quality gate green: `go fmt`, `go mod tidy`, `go vet`, `golangci-lint run` (0 issues), `go test ./...` (all packages, including `-race` on `chat`/`tool`), `go build -o bin/nuimanbot ./cmd/nuimanbot`, `./bin/nuimanbot --help` (starts and shuts down cleanly, no panic).

## Recent Activity (Phase 2)

- 2026-08-02: P2.1 complete — `Client.Publish` (new: relay connection tracking via `relayConn`/`connsMu`, `ConnectedRelayCount`) added to `internal/infrastructure/nostr/client.go`; `Event` given NIP-01 wire JSON tags + a `Tag()` helper in `event.go`; `Gateway.Send()` implemented in `internal/adapter/gateway/buzz/gateway.go`, publishing correctly-signed, `#h`-tagged `kind:9` events and incrementing `buzz_events_published_total{status}`. All new code covered by tests (client_test.go, event_test.go, gateway_test.go), race-detector clean, full quality gate green.
- 2026-08-02: P2.2 complete — pre-requisite schema spike run directly against `github.com/block/buzz` (`crates/buzz-relay/src/handlers/side_effects.rs`'s `handle_agent_profile`, `crates/buzz-cli/src/commands/channels.rs`'s `cmd_set_add_policy`, and `NOSTR.md`), found the doc comment on `KIND_AGENT_PROFILE` in `kind.rs` ("Agent metadata + owner reference") does not match what any current relay code path reads or requires: the only content field the relay parses/enforces is `channel_add_policy` (`"anyone"|"owner_only"|"nobody"`), a per-pubkey channel-invite-permission setting, not an identity declaration. Implemented `Gateway.publishAgentProfile`/`publishAgentProfileBestEffort` against this verified schema (constant `buzzChannelAddPolicy = "owner_only"`), published once at `Start()` via a bounded-retry goroutine (relay connection is dialed async and may not be up on the first attempt), not on every message. Full finding recorded in implementation-notes.md. All new code covered by tests, race-detector clean, full quality gate green.
- 2026-08-02: P2.3+P2.4 complete. P2.3: added `nostr.KindChannelMembership`/`PubkeyTagName`/`RoleTagName`/`RoleBot` constants and `NewMembershipFilter`/`NewAgentProfileFilter` (verified against `side_effects.rs`'s `handle_put_user` for the kind:9000 `p`/`role` tag shape); made `Client.Start`/`NewSubscriptionRequest` variadic over `Filter` so channel-message + membership + profile filters combine into one NIP-01 REQ (multiple filters OR'd, since kind:10100 carries no `#h` tag and can't share a single filter object with kind:9000's `#h` requirement). Added `agentCache` (`internal/adapter/gateway/buzz/agent_cache.go`), a concurrency-safe pubkey→is_agent map. `Gateway.processEvent` now dedupes/verifies every event kind generically (hardening: an unverified kind:9000/10100 event must not be able to forge cache state) before dispatching to `handleAgentStatusEvent` (kind:9000/10100) or the renamed `processChannelMessage` (kind:9), which now reads `sender_is_agent` from the cache instead of a hardcoded `false`. P2.4: added `loopGuard` (`internal/adapter/gateway/buzz/loop_guard.go`) — chose a per-channel consecutive-agent-message-count + time-window heuristic (5 messages / 30s) over a reply-chain/event-tag approach, because Buzz's kind:9 messages carry no reliable in-reply-to tag to follow (research.md Q2 left this open). Wired into `processChannelMessage` right before handler dispatch. Proven via test at both the `loopGuard` unit level and the `Gateway.processEvent` integration level: a 50-message simulated runaway agent-to-agent chain (same channel, rapid succession) terminates at exactly the 5-message threshold; a single agent reply and an alternating human/agent exchange are never suppressed. All new code covered by tests, race-detector clean, full quality gate green.

## Recent Activity (Phase 0-1)

- 2026-08-02: Spec directory created from `nuimanbot-support-buzz-PRD.md`. All phase files initialized. PRD content (FRs, NFRs, rollout exit criteria) carried into spec.md without re-derivation, per explicit instruction — this PRD was already reviewed/hardened for this purpose.
- 2026-08-02: Spec review completed (`/review-spec`). Verified claims against the actual codebase and fixed several gaps directly: (1) added tasks.md P0.2 spike for research.md Q5 (was previously unscoped, silently deferred into P1.3's refactor step); (2) added tasks.md P1.6b — FR-007's generate-if-absent keypair helper had zero task coverage; added corresponding Phase 1 exit criterion to spec.md; (3) corrected the `"buzz:<pubkey>"` user-key convention across spec.md/data-dictionary.md/architecture.md/tasks.md — `domain.User.ID` is a UUID, lookup is by `(Platform, PlatformUID)` tuple via the existing `usecase/user.Service`, and no gateway currently auto-creates users on first message (that claim was inaccurate — CreateUser today is CLI-admin-only); (4) clarified `BuzzConfig.DMPolicy` is reserved/unused — no FR in this spec covers Buzz DMs; (5) flagged (not resolved) research.md Q6: Phase 3's "no bypass" RBAC claim can't be verified as-is because the chat-triggered tool path bypasses RBAC/rate-limit checks for all platforms today, not just Buzz — needs a design decision before P3.1 starts.
