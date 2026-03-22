# Spec: Improved Memory System

**Feature:** improved-memory-system
**PRD:** `improved-memory-system-prd.md`
**Status:** Planning
**Date:** 2026-03-22

---

## Overview

Three-part upgrade to NuimanBot's memory architecture:

- **Part A — Ingatan Bridge**: Replace file-based FTS with Ingatan's hybrid HNSW+BM25 search engine via a new repository adapter. Per-user memory isolation via Ingatan stores. URL/file ingest.
- **Part B — TLS + Security Hardening**: Auto-generate self-signed certs; harden all HTTP surfaces; fix identified web admin security gaps; add REST API security controls.
- **Part C — MCP Client**: Implement JSON-RPC MCP transport, `mcp.json` loader, domain tool bridge. Ingatan MCP server as the first validated target.

---

## Goals

1. Improve memory recall quality through hybrid semantic + keyword search.
2. Enforce per-user memory isolation using Ingatan's per-store RBAC.
3. Enable manual memory ingest from URLs and local files.
4. Harden all HTTP surfaces to TLS with auto-generated certs.
5. Close identified web admin UI and REST API security gaps.
6. Implement a working MCP client that bridges discovered tools into the domain tool registry.

---

## Acceptance Criteria

### Part A — Ingatan Bridge
- `memory.backend: ingatan` routes all cell reads/writes through Ingatan REST API
- Hybrid search returns semantically relevant results when query tokens don't match stored content exactly
- Each user's memory isolated in `{store_prefix}_{user_id_hash}` store; no cross-user data accessible
- Graceful fallback to built-in backend when Ingatan is unreachable
- All quality gates pass

### Part B — TLS + Security Hardening
- Bot starts HTTPS on web admin port when `tls.enabled: true`
- Self-signed cert auto-generated to `data/certs/` if absent; reused on restart
- `Secure: true` on session cookie when TLS is active
- All admin routes return 403 for non-admin authenticated user
- Login rate limit returns HTTP 429 after 5 failed attempts within 1 minute
- `POST /api/v1/auth/token` issues JWT; subsequent calls with that JWT succeed; calls without token return 401
- POST requests with body > 1 MiB return 413
- First login with default `admin/admin` redirects to password-change page

### Part C — MCP Client
- `mcp.json` with at least one `http` transport loads without error
- Discovered tools appear in `/help` under `mcp:<server>:` prefix
- Chat message triggering MCP tool call returns response from MCP server output
- Misconfigured MCP server in `mcp.json` skipped with logged error; bot starts normally
- Ingatan's `memory_search` tool callable from chat when Ingatan MCP is configured

---

## Out of Scope

- Vector search in the built-in backend
- Multi-tenancy in the built-in backend
- Docker/Kubernetes deployment
- MCP server implementation for NuimanBot
- Comprehensive linting cleanup
