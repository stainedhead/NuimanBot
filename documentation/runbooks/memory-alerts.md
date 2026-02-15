# Memory System Alert Runbook

Alert reference for the NuimanBot memory v2 system.

## Alert Thresholds

| Alert | Threshold | Severity | Component |
|-------|-----------|----------|-----------|
| Slow FTS Query | >50ms | Warning (log only) | memory_recall |
| Slow Memory Recall | >100ms | Warning | memory_recall |
| Slow Extraction | >5s | Error | memory_curator |
| Slow Consolidation | >5s | Warning | memory_curator |
| Extraction Failed | On error | Error | memory_curator |
| Consolidation Failed | On error | Warning | memory_curator |
| Recall Failed | On error | Error | memory_recall |

Threshold constants are defined in `internal/usecase/memoryv2/memory_alerts.go`.

## Alerts

### Memory Extraction Failed

**Severity:** Error
**Title:** `Memory extraction failed`
**Triggers:** LLM call failure, invalid JSON response, or zero cells created with errors.

**Investigation:**
1. Check the `conversation_id` in alert details
2. Review LLM provider status and API key validity
3. Check `memory_extraction_total{status="error"}` metric for error rate
4. Review extraction duration via `memory_extraction_duration_seconds` histogram

**Resolution:**
- If LLM provider is down: wait for recovery; extractions are non-blocking
- If JSON parse errors: check LLM model version for schema compliance changes
- If zero cells created: review interaction content quality (may be trivial chit-chat)

### Scene Consolidation Failed

**Severity:** Warning
**Title:** `Scene consolidation failed`
**Triggers:** LLM consolidation call failure, invalid JSON, or scene upsert failure.

**Investigation:**
1. Check the `scene` name in alert details
2. Review `memory_consolidation_total{status="error"}` metric
3. Check if the scene has an unusually large number of cells (query `GetByScene`)
4. Review consolidation duration via `memory_consolidation_duration_seconds`

**Resolution:**
- If LLM failure: same as extraction (provider issue)
- If scene upsert fails: check SQLite database health and disk space
- If slow: consider reducing `SceneSummaryMaxTokens` in CuratorConfig

### Memory Recall Failed

**Severity:** Error
**Title:** `Memory recall failed`
**Triggers:** FTS search failure or salience fallback failure.

**Investigation:**
1. Check `conversation_id` and `query` in alert details
2. Review `memory_recall_total{status="error"}` metric
3. Check FTS index health via admin CLI: `nuimanbot memory rebuild-fts`
4. Review `memory_fts_query_duration_seconds` for latency trends

**Resolution:**
- If FTS index corrupted: run `nuimanbot memory rebuild-fts`
- If SQLite locked: check for concurrent write contention
- If salience fallback fails: check cell repository connectivity

### Slow Memory Recall

**Severity:** Warning
**Title:** `Slow memory recall`
**Triggers:** Total recall duration exceeds 100ms.

**Investigation:**
1. Check `duration_ms` and `threshold_ms` in alert details
2. Review `memory_recall_duration_seconds` histogram for p95/p99 trends
3. Check `memory_fts_query_duration_seconds` to isolate FTS vs scene lookup time
4. Review cell count and scene count in response

**Resolution:**
- If FTS slow: optimize FTS index, reduce `FTSResultLimit`
- If scene lookup slow: reduce `MaxScenes` in RecallConfig
- If token budget calculation slow: reduce result set size
- Consider database VACUUM if SQLite file has grown significantly

### Slow FTS Query (Log Only)

**Threshold:** >50ms
**Triggers:** Individual FTS query exceeds threshold. Logged as warning, no alert sent.

**Investigation:**
1. Check query complexity in log entry
2. Review `memory_fts_query_duration_seconds` histogram
3. Check FTS table size: `nuimanbot memory stats`

**Resolution:**
- Rebuild FTS index: `nuimanbot memory rebuild-fts`
- If persistent: review SQLite PRAGMA settings (journal_mode, synchronous)
- Consider adding query term limits for very long queries

## Metrics Reference

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `memory_extraction_total` | Counter | status | Extraction outcomes (success/error/skipped) |
| `memory_extraction_duration_seconds` | Histogram | - | Extraction latency |
| `memory_cells_created_total` | Counter | - | Total cells created |
| `memory_consolidation_total` | Counter | status | Consolidation outcomes |
| `memory_consolidation_duration_seconds` | Histogram | - | Consolidation latency |
| `memory_recall_total` | Counter | status, query_type | Recall outcomes (fts/fallback) |
| `memory_recall_duration_seconds` | Histogram | - | Recall latency |
| `memory_recall_cells_total` | Counter | - | Total cells recalled |
| `memory_fts_query_duration_seconds` | Histogram | - | FTS query latency |

## Structured Log Fields

All memory logs include a `component` field for filtering:
- `memory_curator` - extraction and consolidation operations
- `memory_recall` - recall and FTS operations

Common fields: `conversation_id`, `duration_ms`, `error`.
