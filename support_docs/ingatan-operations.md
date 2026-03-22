# Ingatan Operations Guide

This guide covers operational tasks for managing NuimanBot's Ingatan memory backend. It focuses on expired memory cell cleanup, which requires manual intervention because Ingatan does not support automatic TTL-based deletion.

---

## Expired Memory Cells

### What Are Expired Cells?

NuimanBot stores memory cells in Ingatan with an optional `expires_at` field. When a cell's `expires_at` timestamp is in the past, NuimanBot considers the cell expired. Expired cells are invisible to the bot during normal operation — the recall layer skips them automatically — but they remain stored in Ingatan and consume storage indefinitely.

Over time, expired cells accumulate. This is expected behavior (see ADR-4 in the improved-memory-system spec), but operators should monitor and clean up expired cells periodically.

### Why Do Expired Cells Accumulate?

Ingatan does not expose a TTL-based delete endpoint or automatic expiry mechanism. NuimanBot's `DeleteExpired` operation is intentionally a no-op for the Ingatan backend. Expired cells must be removed manually via the Ingatan REST API.

---

## Monitoring Expired Cell Accumulation

NuimanBot exposes a Prometheus counter that tracks how many expired cells are encountered during memory recall:

**Metric:** `memory_recall_expired_cells_skipped_total`

A steadily increasing value indicates expired cells are accumulating in Ingatan storage. Access this counter via the metrics endpoint (default: `/metrics`):

```
curl http://localhost:9090/metrics | grep memory_recall_expired_cells_skipped
```

**Interpretation:**

| Behaviour | Meaning |
|-----------|---------|
| Counter stays at 0 | No expired cells encountered during recall |
| Counter increases slowly | Normal expiry rate; schedule periodic cleanup |
| Counter increases rapidly | High expiry rate; run cleanup immediately |

---

## Manual Cleanup Procedure

Use the Ingatan REST API to find and delete expired cells. You will need:

- Access to the Ingatan admin API (URL and credentials from your NuimanBot configuration)
- The store prefix configured in NuimanBot (default: `nuiman`)
- The current time in RFC3339 format (e.g., `2026-03-22T17:00:00Z`)

### Step 1 — List All Stores

Retrieve all NuimanBot memory stores from the Ingatan API:

```
GET /api/v1/stores
```

NuimanBot store names follow the pattern: `{prefix}_{sha256(conversationID)[:16]}`

Example store names with the default `nuiman` prefix:

```
nuiman_a3f2c1d4e5b6a7f8
nuiman_9b8c7d6e5f4a3b2c
```

### Step 2 — Search for Expired Cells in Each Store

For each store, search for cells whose `expires_at` metadata field is in the past:

```
POST /api/v1/stores/{store}/memories/search
Content-Type: application/json

{
  "query": "*",
  "mode": "keyword",
  "top_k": 1000,
  "metadata_filter": {
    "expires_at": { "$lt": "2026-03-22T17:00:00Z" }
  }
}
```

Replace `2026-03-22T17:00:00Z` with the current time in RFC3339 format.

**Note:** The exact filter syntax depends on your Ingatan version. If metadata filtering is not supported, retrieve all memories and filter client-side by checking the `expires_at` field in each memory's `metadata` object.

### Step 3 — Delete Each Expired Cell

For each expired cell returned in the search results, delete it by its Ingatan-internal ID:

```
DELETE /api/v1/stores/{store}/memories/{memoryID}
```

The `memoryID` is the `id` field from the search result (Ingatan's internal UUID, not NuimanBot's `nuiman_cell_id`).

### Step 4 — Verify Cleanup

Repeat Step 2 for each store. The search should return no results when cleanup is complete. The `memory_recall_expired_cells_skipped_total` metric should stop increasing after the next recall cycle.

---

## Recommended Maintenance Schedule

| Environment | Cleanup Frequency |
|-------------|------------------|
| Development | On demand |
| Staging | Weekly |
| Production | Daily (automated) or Weekly (manual) |

### Automating Cleanup

Consider running a scheduled cron job that calls the Ingatan API directly to remove expired cells. The `expires_at` value is stored as an RFC3339 string in each memory's `metadata` object. A cleanup script should:

1. List all stores matching the NuimanBot prefix pattern
2. Search each store for memories where `metadata.expires_at < now`
3. Delete each matching memory

Monitor the `memory_recall_expired_cells_skipped_total` counter to verify the automation is keeping pace with expiry.

---

## Configuration Reference

The following NuimanBot configuration values affect Ingatan store names and memory expiry:

| Setting | Default | Description |
|---------|---------|-------------|
| `ingatan.store_prefix` | `nuiman` | Prefix used in all Ingatan store names |
| `ingatan.url` | (required) | Base URL for the Ingatan REST API |

Store names are computed as: `{store_prefix}_{sha256(conversationID)[:16]}`
