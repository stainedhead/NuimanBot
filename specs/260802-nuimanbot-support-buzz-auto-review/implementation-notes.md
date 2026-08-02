# Implementation Notes: NuimanBot Support for Buzz — Auto-Review Remediation

**Created:** 2026-08-02

## Purpose

This document records decisions, edge cases, and lessons learned during implementation of the 14 tasked FRs (see tasks.md). Update it as each task completes — especially FR-009's TTL/capacity choice and FR-010's high-water-mark design, which were explicitly left to the implementer by the product owner and must be documented here per that decision.

## Technical Decisions

*(To be filled in during implementation. Expected entries include:)*

- FR-009: chosen TTL/capacity bound for `agentCache` eviction, and rationale. **Done — see below.**
- FR-010: per-relay vs. gateway-wide high-water-mark choice for `Since`, and rationale. **Done — see below.**
- FR-004: ticker vs. callback-driven gauge update mechanism, and rationale.
- FR-007: implement-env-var vs. document-only resolution, and rationale.
- FR-016: remove vs. document-only resolution, and rationale.

### FR-009 — `agentCache` TTL + capacity bound (Cluster D)

**Files:** `internal/adapter/gateway/buzz/agent_cache.go`, `agent_cache_test.go`

No existing cache-eviction pattern elsewhere in this codebase to match (confirmed by search), so this was a fresh design call per the product owner's Open Question 4 resolution.

**Chosen bound: sliding 24h TTL + hard 5,000-entry capacity cap (oldest-touched evicted first).**

- **TTL = 24h, sliding (refreshed on both `Set` and `IsAgent`), not fixed-at-write.** `agentCache` is also read by `loopGuard` (via `Gateway.processEvent`) on every channel message to decide whether the sender counts toward the runaway-agent-chain heuristic. A short or write-only TTL would let a long-lived agent that's simply quiet for a while (no new kind:9000/10100 identity event, but still occasionally posting) silently fall back to "unknown" (`IsAgent` defaults `false` for unknown pubkeys), which **weakens loop-guard protection** rather than just wasting memory. Sliding the TTL on every successful `IsAgent` lookup keeps an actively-participating agent's identity alive indefinitely without needing repeated kind:9000/10100 events, while a pubkey that genuinely stops appearing for 24h is reclaimed. 24h was picked as comfortably outliving any single Buzz conversation/session without being so long it defeats the point of bounding growth.
- **Capacity = 5,000 entries, hard cap regardless of TTL.** Backstops the TTL against a burst scenario (a busy/public relay observing many thousands of distinct pubkeys within one 24h window) — without a cap, TTL alone doesn't bound peak memory, only steady-state. On overflow, the single least-recently-touched entry is evicted (via a linear scan under the write lock) to admit the new pubkey; this is O(n) but only triggered once already at capacity, and n is capped at 5,000, so it's cheap in absolute terms. This isn't a true LRU (no access-reordering data structure) — a simple "evict min lastSeen" scan was chosen over `container/list`-backed LRU for simplicity, since `agentCache.Set` (the only path that can add new keys) fires only on kind:9000/10100 events, not on every chat message, so write throughput is low and the O(n) scan cost is a non-issue in practice.
- **Concurrency note:** `IsAgent`'s TTL-sliding write happens *after* releasing `agentCache.mu`'s read lock (to avoid holding a lock across the whole call), so `cacheEntry.isAgent`/`lastSeen` are both `atomic.Bool`/`atomic.Int64` rather than plain fields guarded solely by the map-level `RWMutex` — a plain-field version failed `go test -race` (mutation in `Set` under `Lock()` racing with the unguarded post-`RUnlock()` read in `IsAgent`).
- **`loopGuard.channels` intentionally left untouched.** Re-confirmed (not assumed) that it doesn't need the same treatment: `loopGuard.Allow` already ages out a channel's streak lazily whenever `receivedAt.Sub(s.firstAt) > window` (30s), and per FR-009's own text, the map is keyed by Buzz *channel* ID, not by pubkey — channel count is bounded by the operator's static config (`BuzzConfig.ChannelIDs`), not by relay traffic volume, so it cannot grow unbounded the way a pubkey-keyed map can.

### FR-010 — `Filter.Since` reconnect backfill (Cluster D)

**Files:** `internal/infrastructure/nostr/client.go`, `client_test.go` (also read, unchanged: `subscription.go`)

**Chosen design: per-relay high-water mark**, tracked as `map[relayURL]lastEventCreatedAt` on `nostr.Client`, updated whenever an event is successfully parsed off that relay's connection, and re-applied to the shared `Filter` set (via a fresh `since` pointer) each time that specific relay's read loop reconnects.

- **Why per-relay, not gateway-wide:** research.md Q1 flagged that relays may deliver events out of order relative to each other. A single gateway-wide high-water mark taken from whichever relay happens to deliver the newest event would risk skipping events on a *different*, slower/lagging relay if that relay reconnects and inherits the faster relay's higher `since` value — a real loss, not just a duplicate (which `seen`/`seenMu` in `gateway.go` already absorbs harmlessly). Per-relay tracking means each relay's `since` is only ever derived from what that same relay has itself already delivered, so a reconnect can never skip past events that specific relay hasn't sent yet, regardless of clock skew or delivery-order differences between relays.
- **`since` value = the last processed event's `created_at` exactly, not `+1`.** NIP-01 timestamps are second-resolution, so multiple distinct events can legitimately share one `created_at`. Using `last + 1` risks silently skipping same-second siblings of the last-seen event. Using the exact value means the relay will re-send that last event once more on reconnect (inclusive `since` semantics), but that's a harmless duplicate: `gateway.go`'s existing `seen`/`seenMu` dedupe-by-event-ID already exists specifically to absorb this kind of at-least-once redelivery.
- **Only reconnects get `since` populated — the first connection to each relay does not.** This preserves existing behavior (full backlog on process startup) and matches the acceptance criteria's framing ("populate on reconnect"); `Client.Start`'s initial dial still uses the caller-supplied filters unmodified.
- **Scope boundary:** "last successfully processed event" is interpreted at the transport layer as "last event this `Client` successfully parsed and handed off on `Events()`," not "last event `gateway.go`'s business logic (signature verification, dedupe, RBAC) accepted." Threading a processing-outcome acknowledgment back down into `nostr.Client` would cross the adapter/infrastructure boundary this branch's other findings (e.g. FR-011) are actively trying to clean up, and `gateway.go` was off-limits for this cluster. A forged/invalid event that reaches `Client.Events()` still advances `since` — acceptable because `gateway.go` would reject it identically on redelivery post-reconnect (no correctness gap, just an unnecessary but harmless resend).

## Edge Cases & Solutions

*(To be filled in during implementation.)*

## Deviations from Plan

*(To be filled in during implementation, if any cluster's actual work diverges from tasks.md's breakdown.)*

## Lessons Learned

*(To be filled in at spec completion, before archiving.)*
