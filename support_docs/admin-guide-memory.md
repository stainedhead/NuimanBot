# Memory System Administration Guide

**Version:** 1.0
**Last Updated:** 2026-02-15
**Target Audience:** System Administrators, DevOps Engineers

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [CLI Commands Reference](#cli-commands-reference)
4. [Metrics and Monitoring](#metrics-and-monitoring)
5. [Logging](#logging)
6. [Alerting](#alerting)
7. [Performance Tuning](#performance-tuning)
8. [Database Maintenance](#database-maintenance)
9. [Backup and Recovery](#backup-and-recovery)
10. [Troubleshooting](#troubleshooting)

---

## Overview

NuimanBot's memory system provides persistent, structured long-term memory across conversations. It uses an LLM-based extraction pipeline to identify important information from interactions and stores it as typed "memory cells" organized into topical "scenes."

**Key components:**
- **Memory Curator**: Extracts memory cells from completed interactions using LLM analysis
- **Memory Recall**: Retrieves relevant memories using FTS5 full-text search with salience-based fallback
- **Scene Consolidation**: Generates and maintains topic-level summaries for efficient context injection

**Storage:** File-based JSON storage with memory indexing

---

## Architecture

The memory system follows Clean Architecture with four layers:

```
Domain Layer       → MemoryCell, MemoryScene entities, repository interfaces
Use Case Layer     → MemoryCuratorService, MemoryRecallService
Adapter Layer      → CLI commands, ChatService integration
Infrastructure     → SQLite repositories, LLM client, metrics, tracing, alerting
```

**Data flow - Extraction:**
1. User sends a message → ChatService processes it → LLM responds
2. After response, MemoryCuratorService extracts memory cells via LLM
3. Cells are validated and persisted to SQLite
4. Touched scenes are consolidated (summaries regenerated)

**Data flow - Recall:**
1. Before LLM request, MemoryRecallService queries FTS5 index
2. Falls back to high-salience cells if FTS yields no results
3. Applies token budget, includes scene summaries
4. Injected into context window as structured memory block

For detailed diagrams, see `documentation/diagrams/`.

---

## CLI Commands Reference

### Basic Memory Commands

**List memory cells:**
```bash
# List all cells
./bin/nuimanbot memory list

# Filter by scene
./bin/nuimanbot memory list --scene project-setup

# Filter by conversation
./bin/nuimanbot memory list --conversation conv-123

# JSON output
./bin/nuimanbot memory list --format json
```

**Get a specific cell:**
```bash
./bin/nuimanbot memory get <cell-id>
./bin/nuimanbot memory get <cell-id> --format json
```

**Search memory (FTS):**
```bash
# Full-text search
./bin/nuimanbot memory search "authentication OAuth2"

# Limit results
./bin/nuimanbot memory search "project setup" --limit 5

# JSON output
./bin/nuimanbot memory search "TDD workflow" --format json
```

**Delete a cell:**
```bash
./bin/nuimanbot memory delete <cell-id>
```

**List scenes:**
```bash
./bin/nuimanbot memory scenes
./bin/nuimanbot memory scenes --format json
```

**Prune expired cells:**
```bash
./bin/nuimanbot memory prune
```

### Admin Commands

**View memory statistics:**
```bash
./bin/nuimanbot memory stats
# Output:
# Memory Statistics
# ------------------------------
# Cells:         142
# Scenes:        8
# Database Size: 2.3 MB
```

**Clear user data (with dry-run):**
```bash
# Dry run - show what would be deleted
./bin/nuimanbot memory clear-user --conversation conv-123

# Actually delete
./bin/nuimanbot memory clear-user --conversation conv-123 --confirm
```

**Export memories:**
```bash
# Export all memories for a conversation to JSON
./bin/nuimanbot memory export --conversation conv-123 > backup.json
```

**Import memories:**
```bash
# Import from JSON (skips duplicates)
./bin/nuimanbot memory import < backup.json
```

**Rebuild FTS index:**
```bash
# Rebuild full-text search index (use after database corruption or migration)
./bin/nuimanbot memory rebuild-fts
```

---

## Metrics and Monitoring

### Prometheus Metrics

All memory metrics are exposed at the `/metrics` endpoint.

#### Extraction Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `memory_extraction_total` | Counter | `status` (success, error, skipped) | Total extraction attempts |
| `memory_extraction_duration_seconds` | Histogram | - | Extraction latency |
| `memory_cells_created_total` | Counter | - | Total cells created across all extractions |

#### Consolidation Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `memory_consolidation_total` | Counter | `status` (success, error) | Total scene consolidation attempts |
| `memory_consolidation_duration_seconds` | Histogram | - | Consolidation latency |

#### Recall Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `memory_recall_total` | Counter | `status`, `query_type` (fts, fallback) | Total recall attempts |
| `memory_recall_duration_seconds` | Histogram | - | Recall latency (end-to-end) |
| `memory_recall_cells_total` | Counter | - | Total cells recalled |
| `memory_fts_query_duration_seconds` | Histogram | - | FTS query latency |

### Key Health Indicators

**Extraction success rate:**
```promql
rate(memory_extraction_total{status="success"}[5m])
/ rate(memory_extraction_total{status=~"success|error"}[5m])
```
Target: > 95%

**Recall latency (p95):**
```promql
histogram_quantile(0.95, rate(memory_recall_duration_seconds_bucket[5m]))
```
Target: < 100ms

**FTS query latency (p95):**
```promql
histogram_quantile(0.95, rate(memory_fts_query_duration_seconds_bucket[5m]))
```
Target: < 50ms

**Fallback rate:**
```promql
rate(memory_recall_total{query_type="fallback"}[5m])
/ rate(memory_recall_total{status="success"}[5m])
```
High fallback rate may indicate FTS index issues or insufficient cell content.

### Grafana Dashboard Suggestions

Create panels for:
1. **Extraction rate** - `memory_extraction_total` by status (stacked area)
2. **Extraction latency** - `memory_extraction_duration_seconds` (heatmap)
3. **Cells created** - `memory_cells_created_total` (rate graph)
4. **Recall latency** - `memory_recall_duration_seconds` (p50, p95, p99 lines)
5. **FTS vs fallback** - `memory_recall_total` by query_type (pie chart)
6. **FTS query latency** - `memory_fts_query_duration_seconds` (heatmap)

---

## Logging

The memory system uses structured `log/slog` logging with consistent key-value attributes.

### Log Messages Reference

#### Extraction

| Level | Message | Key Attributes |
|-------|---------|---------------|
| DEBUG | `Memory extraction skipped: disabled` | `conversation_id` |
| INFO | `Starting memory extraction` | `conversation_id` |
| INFO | `Memory extraction complete` | `conversation_id`, `cells_created`, `scenes_updated`, `errors`, `duration_ms` |
| WARN | `Invalid extracted cell` | `conversation_id`, `scene`, `cell_type`, `error` |
| ERROR | `LLM extraction failed` | `conversation_id`, `error` |
| ERROR | `Invalid JSON from LLM extraction` | `conversation_id`, `error` |
| ERROR | `Failed to persist memory cell` | `conversation_id`, `cell_id`, `scene`, `error` |
| ERROR | `Extraction produced no cells` | `conversation_id`, `error_count` |

#### Consolidation

| Level | Message | Key Attributes |
|-------|---------|---------------|
| INFO | `Scene consolidation completed` | `scene`, `cell_count`, `token_count`, `duration_ms` |
| ERROR | `Failed to get cells for scene consolidation` | `scene`, `error` |
| ERROR | `LLM consolidation failed` | `scene`, `cell_count`, `error` |
| ERROR | `Invalid JSON from LLM consolidation` | `scene`, `error` |
| ERROR | `Failed to upsert scene` | `scene`, `error` |

#### Recall

| Level | Message | Key Attributes |
|-------|---------|---------------|
| INFO | `Starting memory recall` | `conversation_id`, `query`, `max_tokens` |
| INFO | `FTS returned no results, using salience fallback` | `conversation_id`, `query` |
| INFO | `Memory recall complete` | `conversation_id`, `cell_count`, `scene_count`, `total_tokens`, `fts_match_count`, `fallback_used`, `duration_ms` |
| WARN | `Slow FTS query` | `query`, `duration_ms`, `threshold_ms` |
| WARN | `Slow memory recall` | `conversation_id`, `duration_ms`, `threshold_ms` |
| ERROR | `FTS search failed` | `conversation_id`, `query`, `error` |
| ERROR | `Salience fallback failed` | `conversation_id`, `error` |

### Filtering Memory Logs

```bash
# Filter memory-related logs (structured JSON output)
./bin/nuimanbot ... 2>&1 | jq 'select(.component == "memory_curator" or .component == "memory_recall")'

# Filter errors only
./bin/nuimanbot ... 2>&1 | jq 'select(.level == "ERROR" and (.component | startswith("memory")))'
```

---

## Alerting

The memory system integrates with the `alerting` package for critical conditions.

### Alert Conditions

| Alert | Severity | Trigger |
|-------|----------|---------|
| Memory extraction LLM failure | ERROR | LLM call fails during cell extraction |
| Scene consolidation LLM failure | ERROR | LLM call fails during scene consolidation |
| Memory FTS search failure | ERROR | FTS5 query fails |
| Memory extraction produced no cells | WARNING | Extraction completes but creates 0 cells with errors |
| Slow memory recall | WARNING | Recall exceeds 100ms threshold |
| Slow extraction | WARNING | Extraction exceeds 5s threshold |

### Performance Thresholds

Defined in `internal/usecase/memoryv2/memory_alerts.go`:

| Threshold | Value | Description |
|-----------|-------|-------------|
| `FTSSlowQueryThreshold` | 50ms | Logs warning for slow FTS queries |
| `RecallSlowThreshold` | 100ms | Triggers alert for slow recall |
| `ExtractionSlowThreshold` | 5s | Triggers alert for slow extraction |
| `ConsolidationSlowThreshold` | 5s | Triggers alert for slow consolidation |

### Configuring Alert Channels

Alert channels are configured via the alerting infrastructure:

```go
alerting.Initialize(alerting.Config{
    Enabled:        true,
    ServiceName:    "nuimanbot",
    ThrottleWindow: 300, // 5 minutes between duplicate alerts
    Channels: []alerting.ChannelConfig{
        {Type: alerting.ChannelTypeLog, Enabled: true},
        {Type: alerting.ChannelTypeSlack, Enabled: true, Config: map[string]string{
            "webhook_url": "https://hooks.slack.com/services/...",
        }},
    },
})
```

---

## Performance Tuning

### FTS5 Search Performance

The FTS5 index provides sub-millisecond search at typical cell counts:
- 1,000 cells: ~0.17ms per query
- 5,000 cells: ~0.57ms per query

**Optimization tips:**
- Keep cell content concise (improves index efficiency)
- Use `rebuild-fts` after bulk imports or migrations
- Monitor `memory_fts_query_duration_seconds` for degradation

### Token Budget

Memory recall enforces a token budget to prevent context bloat:
- Default budget: 2,000 tokens
- Max percentage: 25% of available context window
- Scene summaries count against the budget

**Tuning:**
- Increase budget for conversations that benefit from more context
- Decrease for latency-sensitive scenarios
- Monitor `total_tokens` in recall completion logs

### Cell Volume Management

High cell counts may degrade performance over time:
- Run `memory prune` periodically to remove expired cells
- Use `memory clear-user` for inactive conversations
- Monitor `memory_cells_created_total` growth rate

---

## Database Maintenance

### Storage Location

Memory uses file-based JSON storage: `memory/` directory (in the data directory).

### File Structure

**Directories:**
- `memory/cells/` - Individual memory cell JSON files
- `memory/scenes/` - Scene summary JSON files
- `memory/index/` - Search index for memory lookup

### Integrity Check

Use the built-in memory stats command to verify storage integrity:

```bash
./bin/nuimanbot memory stats
```

### Index Rebuild

If search returns incorrect results or after storage corruption:

```bash
./bin/nuimanbot memory rebuild-index
```

This recreates the search index from existing memory cell files.

---

## Backup and Recovery

### Backup

```bash
# 1. Export conversation memories (JSON, portable)
./bin/nuimanbot memory export --conversation conv-123 > memory-conv-123.json

# 2. Full directory backup (file-based storage)
cp -r data/memory/ backups/memory-$(date +%Y%m%d)/

# 3. Verify backup
./bin/nuimanbot memory stats --data-dir backups/memory-$(date +%Y%m%d)/
```

### Recovery

```bash
# Option A: Restore from directory backup
cp -r backups/memory-20260215/ data/memory/
./bin/nuimanbot memory rebuild-index  # Rebuild search index after restore

# Option B: Import from JSON export
./bin/nuimanbot memory import < memory-conv-123.json
```

### Export Format

The JSON export format (version 1):

```json
{
  "version": 1,
  "conversation_id": "conv-123",
  "exported_at": "2026-02-15T12:00:00Z",
  "cell_count": 42,
  "scene_count": 5,
  "cells": [...],
  "scenes": [...]
}
```

Import is idempotent - duplicate cells are skipped, scenes are upserted.

---

## Troubleshooting

### Issue: Memory extraction not running

**Symptom:** No new cells created after conversations.

**Diagnosis:**
```bash
# Check if extraction is enabled
# Look for "Memory extraction skipped: disabled" in logs

# Check extraction metrics
curl -s localhost:8080/metrics | grep memory_extraction_total
```

**Solution:**
- Verify `CuratorConfig.Enabled` is `true` in configuration
- Check that the LLM client is properly configured
- Review logs for extraction errors

### Issue: Search returns no results

**Symptom:** `memory search` returns empty despite cells existing.

**Diagnosis:**
```bash
# Verify cells exist
./bin/nuimanbot memory list

# Check index status
./bin/nuimanbot memory stats
```

**Solution:**
```bash
# Rebuild search index
./bin/nuimanbot memory rebuild-index
```

### Issue: High extraction error rate

**Symptom:** `memory_extraction_total{status="error"}` increasing.

**Diagnosis:**
```bash
# Check logs for specific errors
# Common causes:
# - LLM API timeouts
# - Invalid JSON responses
# - Repository connection failures
```

**Solution:**
- Check LLM API health and rate limits
- Enable `RetryOnInvalidJSON` in curator config
- Monitor `memory_extraction_duration_seconds` for timeouts
- Check database connectivity

### Issue: Slow recall performance

**Symptom:** `memory_recall_duration_seconds` > 100ms.

**Diagnosis:**
```bash
# Check FTS query latency
curl -s localhost:8080/metrics | grep memory_fts_query_duration

# Check cell count
./bin/nuimanbot memory stats
```

**Solution:**
- Rebuild search index: `./bin/nuimanbot memory rebuild-index`
- Prune expired cells: `./bin/nuimanbot memory prune`
- Reduce `SearchResultLimit` in recall config
- Clean up old index files

### Issue: File locking errors

**Symptom:** File access errors during memory operations.

**Solution:**
- Ensure only one NuimanBot instance accesses the memory directory
- Check for zombie processes: `ps aux | grep nuimanbot`
- Verify file permissions on memory/ directory
- Check disk space availability

### Issue: Memory bloat

**Symptom:** Memory directory growing rapidly, cell count very high.

**Solution:**
```bash
# Check stats
./bin/nuimanbot memory stats

# Prune expired
./bin/nuimanbot memory prune

# Clear inactive conversations
./bin/nuimanbot memory clear-user --conversation <old-conv-id> --confirm

# Check directory size
du -sh data/memory/
```

Consider lowering extraction salience thresholds to reduce low-value cells.

---

**Document Version:** 1.0
**Last Updated:** 2026-02-15
**Maintainer:** NuimanBot Development Team
