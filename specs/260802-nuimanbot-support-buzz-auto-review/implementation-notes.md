# Implementation Notes: NuimanBot Support for Buzz — Auto-Review Remediation

**Created:** 2026-08-02

## Purpose

This document records decisions, edge cases, and lessons learned during implementation of the 14 tasked FRs (see tasks.md). Update it as each task completes — especially FR-009's TTL/capacity choice and FR-010's high-water-mark design, which were explicitly left to the implementer by the product owner and must be documented here per that decision.

## Technical Decisions

*(To be filled in during implementation. Expected entries include:)*

- FR-009: chosen TTL/capacity bound for `agentCache` eviction, and rationale.
- FR-010: per-relay vs. gateway-wide high-water-mark choice for `Since`, and rationale.
- FR-004: ticker vs. callback-driven gauge update mechanism, and rationale.
- FR-007: implement-env-var vs. document-only resolution, and rationale.
- FR-016: remove vs. document-only resolution, and rationale.

## Edge Cases & Solutions

*(To be filled in during implementation.)*

## Deviations from Plan

*(To be filled in during implementation, if any cluster's actual work diverges from tasks.md's breakdown.)*

## Lessons Learned

*(To be filled in at spec completion, before archiving.)*
