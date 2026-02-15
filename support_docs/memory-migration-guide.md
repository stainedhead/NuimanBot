# Self-Organizing Memory v2 - Migration Guide

**Version:** 1.0
**Last Updated:** 2026-02-15
**Target Audience:** Developers, System Administrators

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Migration Steps](#migration-steps)
4. [Configuration Reference](#configuration-reference)
5. [Feature Flag Rollout Strategy](#feature-flag-rollout-strategy)
6. [Testing Checklist](#testing-checklist)
7. [Monitoring After Enabling](#monitoring-after-enabling)
8. [Rollback Procedure](#rollback-procedure)
9. [FAQ](#faq)

---

## Overview

Self-Organizing Memory v2 adds an LLM-powered long-term memory system to NuimanBot. It extracts knowledge from conversations as typed "memory cells," organizes them into topical "scenes," and automatically recalls relevant memories for future conversations.

**What's new:**
- Automatic memory extraction after every interaction
- Full-text search (FTS5) for fast memory retrieval
- Salience-based fallback when text search yields no results
- Scene consolidation (topic-level summaries)
- CLI commands for browsing, searching, and managing memories
- Admin commands for export, import, stats, and FTS index management

**What's unchanged:**
- Existing conversation memory (message history) is unaffected
- Core chat functionality works identically with or without memory v2
- No changes to gateway configurations (Telegram, Slack, CLI)

---

## Prerequisites

Before enabling self-organizing memory, ensure:

### 1. Software Requirements

- **Go 1.22+** (for building from source)
- **SQLite with FTS5 support** (provided automatically by `modernc.org/sqlite` dependency)
- **LLM provider configured** (Anthropic recommended for extraction; any provider works for chat)

### 2. LLM Provider

Memory extraction and scene consolidation use LLM calls. The default configuration uses `claude-3-haiku-20240307` for cost efficiency. Verify your LLM provider is configured:

```bash
# Build and test LLM connectivity
go build -o bin/nuimanbot ./cmd/nuimanbot
./bin/nuimanbot --help
```

If NuimanBot starts without LLM errors, your provider is configured correctly.

### 3. Disk Space

Memory v2 uses very little storage. Each cell is a short text (max 2000 characters). Typical usage:

| Scale | Cells | Estimated Size |
|-------|-------|----------------|
| Light | 100 | < 1 MB |
| Moderate | 1,000 | 1-3 MB |
| Heavy | 10,000 | 5-15 MB |

### 4. Data Directory

Ensure the data directory exists and is writable:

```bash
mkdir -p ./data
ls -la ./data/
```

The memory database file will be created automatically at:
- Default: `./data/nuimanbot-memory.db`
- Custom: derived from `storage.dsn` (replaces `.db` with `-memory.db`)

---

## Migration Steps

### Step 1: Update Dependencies

Pull the latest code and update Go module dependencies:

```bash
git pull origin main
go mod tidy
```

### Step 2: Build

Build the updated binary:

```bash
go build -o bin/nuimanbot ./cmd/nuimanbot
```

### Step 3: Start NuimanBot

Start the application normally. Memory v2 initializes automatically on startup:

```bash
./bin/nuimanbot
```

**What happens at startup:**

1. NuimanBot opens (or creates) `nuimanbot-memory.db` in the data directory
2. The schema is initialized: `memory_cells`, `memory_scenes`, `memory_cells_fts` tables
3. FTS5 triggers are created for automatic search index synchronization
4. Curator and recall services are wired into the chat pipeline
5. Startup log confirms: `"Memory v2 (self-organizing memory) initialized"`

### Step 4: Verify Initialization

Check the startup logs for these messages:

```
Memory v2 (self-organizing memory) initialized
  db_path=./data/nuimanbot-memory.db
  curator_enabled=true
  recall_fts_limit=20
  recall_token_budget=2000
```

If you see a warning instead, memory failed to initialize but chat continues working:

```
Failed to open memory v2 database (continuing without memory)
Failed to initialize memory v2 schema (continuing without memory)
```

### Step 5: Verify CLI Commands

Test that memory commands are available:

```bash
# In the NuimanBot CLI, type:
/memory help
/memory stats
/memory list
```

### Step 6: Test Memory Extraction

Have a conversation with NuimanBot and verify extraction works:

```bash
# 1. Chat normally - mention some facts or preferences
> I prefer using Go with table-driven tests

# 2. After the response, check for new cells
/memory list

# 3. Search for what you mentioned
/memory search "table-driven tests"
```

---

## Configuration Reference

Memory v2 is configured in `cmd/nuimanbot/main.go` at initialization time. Here are the key configuration values and their defaults:

### Database Path

| Setting | Default | Description |
|---------|---------|-------------|
| Database path | `./data/nuimanbot-memory.db` | Separate SQLite file for memory data |
| Max open connections | 10 | Connection pool size |
| Max idle connections | 3 | Idle connection pool size |
| Connection max lifetime | 5 minutes | Connection recycling interval |

The database path is derived from `storage.dsn` in your config:
- If `storage.dsn` is empty: uses `./data/nuimanbot-memory.db`
- If `storage.dsn` is `./data/nuimanbot.db`: uses `./data/nuimanbot-memory.db`
- If `storage.dsn` is `./data/mybot.db`: uses `./data/mybot-memory.db`

### Curator (Extraction) Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `Enabled` | `true` | Enable/disable memory extraction |
| `ExtractionModel` | `claude-3-haiku-20240307` | LLM model for extraction |
| `ConsolidationModel` | `claude-3-haiku-20240307` | LLM model for scene consolidation |
| `MaxCellsPerExtraction` | `5` | Maximum cells extracted per interaction |
| `RetryOnInvalidJSON` | `false` | Retry LLM call if JSON is invalid |
| `SceneSummaryMaxTokens` | `500` | Max tokens for scene summaries |

### Recall Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `FTSResultLimit` | `20` | Max results from FTS search |
| `SalienceThreshold` | `0.5` | Minimum salience for fallback recall |
| `FallbackCellLimit` | `10` | Max cells in salience fallback |
| `MaxScenes` | `10` | Max scene summaries to include |
| `TokenBudget` | `2000` | Token budget for recalled memory |

### Changing Configuration

To modify these values, edit the initialization block in `cmd/nuimanbot/main.go` (section `10.5`):

```go
// Curator config
curatorConfig := memoryv2uc.CuratorConfig{
    Enabled:               true,  // Set to false to disable extraction
    ExtractionModel:       "claude-3-haiku-20240307",
    ConsolidationModel:    "claude-3-haiku-20240307",
    MaxCellsPerExtraction: 5,
    RetryOnInvalidJSON:    false,
    SceneSummaryMaxTokens: 500,
}

// Recall config
recallConfig := memoryv2uc.RecallConfig{
    FTSResultLimit:    20,
    SalienceThreshold: 0.5,
    FallbackCellLimit: 10,
    MaxScenes:         10,
    TokenBudget:       2000,
}
```

---

## Feature Flag Rollout Strategy

Memory v2 supports a phased rollout using the `CuratorConfig.Enabled` flag:

### Phase 1: Observe-Only (Recall Disabled, Extraction Disabled)

Start with memory completely disabled to verify the application runs normally with the new code:

```go
curatorConfig := memoryv2uc.CuratorConfig{
    Enabled: false,  // No extraction
}
// Recall still initializes but returns empty (no cells exist)
```

**Duration:** 1-2 days. Verify no regressions in chat functionality.

### Phase 2: Extraction Only (Extraction Enabled, Monitor)

Enable extraction to start building the memory corpus:

```go
curatorConfig := memoryv2uc.CuratorConfig{
    Enabled: true,
}
```

**Duration:** 3-7 days. Monitor:
- `memory_extraction_total{status="success"}` increasing
- `memory_extraction_total{status="error"}` staying low
- `memory_cells_created_total` growing steadily
- No impact on chat response times

### Phase 3: Full Rollout (Extraction + Recall Active)

Once the memory corpus has sufficient data, recall is automatically active. No additional configuration change is needed - recall starts returning results as soon as cells exist in the database.

**Verify:**
- `/memory stats` shows cell and scene counts
- `/memory search <topic>` returns relevant results
- Chat responses reference recalled context appropriately
- `memory_recall_total{status="success"}` increasing

---

## Testing Checklist

Run through this checklist before deploying to production:

### Build Verification

- [ ] `go build -o bin/nuimanbot ./cmd/nuimanbot` succeeds
- [ ] `./bin/nuimanbot --help` runs without errors
- [ ] All tests pass: `go test ./...`

### Memory Initialization

- [ ] Application starts and logs "Memory v2 (self-organizing memory) initialized"
- [ ] Memory database file created at expected path
- [ ] `/memory help` displays available commands
- [ ] `/memory stats` returns zero counts (fresh database)

### Extraction

- [ ] After a conversation, `/memory list` shows new cells
- [ ] Cells have appropriate types (fact, decision, preference, etc.)
- [ ] Cells have reasonable salience scores (0.0 - 1.0)
- [ ] Scene names are descriptive and topic-based
- [ ] `/memory scenes` shows consolidated summaries

### Recall

- [ ] `/memory search <keyword>` returns matching cells
- [ ] Memory context appears in LLM responses (check logs for recall completion)
- [ ] Fallback to salience-based recall works when FTS has no matches

### CLI Commands

- [ ] `/memory list` - displays cells in table format
- [ ] `/memory list --format json` - outputs valid JSON
- [ ] `/memory list --scene <name>` - filters by scene
- [ ] `/memory list --type <type>` - filters by cell type
- [ ] `/memory get <id>` - displays full cell details
- [ ] `/memory search <query>` - full-text search works
- [ ] `/memory delete <id>` - deletes a cell
- [ ] `/memory scenes` - lists scene summaries
- [ ] `/memory prune` - removes expired cells (or reports none)

### Admin Commands

- [ ] `/memory stats` - shows cell count, scene count, DB size
- [ ] `/memory export --conversation <id>` - exports to JSON
- [ ] `/memory import < file.json` - imports from JSON
- [ ] `/memory rebuild-fts` - rebuilds search index
- [ ] `/memory clear-user --conversation <id>` - dry-run shows count
- [ ] `/memory clear-user --conversation <id> --confirm` - deletes cells

### Graceful Degradation

- [ ] If memory DB file is deleted, NuimanBot starts without memory (no crash)
- [ ] If memory DB is read-only, extraction errors are logged but chat continues
- [ ] LLM extraction failures do not block chat responses

### Performance

- [ ] Chat response times are not noticeably impacted
- [ ] FTS search completes in < 50ms (check metrics)
- [ ] Recall completes in < 100ms (check metrics)

---

## Monitoring After Enabling

### Key Metrics to Watch

**First 24 hours - check frequently:**

```bash
# Check metrics endpoint
curl -s localhost:8080/metrics | grep memory_
```

| Metric | Expected | Action if Abnormal |
|--------|----------|-------------------|
| `memory_extraction_total{status="success"}` | Increasing | Check LLM connectivity |
| `memory_extraction_total{status="error"}` | Near zero | Review error logs |
| `memory_cells_created_total` | Growing steadily | Check extraction config |
| `memory_recall_duration_seconds` | < 100ms p95 | Rebuild FTS index |
| `memory_fts_query_duration_seconds` | < 50ms p95 | Rebuild FTS index |

**First week - daily check:**

| Check | Command | Expected |
|-------|---------|----------|
| Cell count | `/memory stats` | Growing proportionally to usage |
| Scene count | `/memory scenes` | Reasonable number of topics |
| Database size | `/memory stats` | Under 10MB for typical usage |
| Error rate | Check logs | < 5% extraction errors |

### Log Messages to Monitor

Watch for these in production logs:

**Normal (INFO):**
- `Starting memory extraction` - extraction triggered
- `Memory extraction complete` - extraction succeeded
- `Memory recall complete` - recall served

**Investigate (WARN):**
- `Slow FTS query` - FTS taking > 50ms
- `FTS returned no results, using salience fallback` - frequent fallback may indicate indexing issues

**Alert (ERROR):**
- `LLM extraction failed` - extraction LLM call failed
- `LLM consolidation failed` - consolidation LLM call failed
- `FTS search failed` - database issue

See [Admin Guide](../documentation/admin-guide-memory.md) for complete logging and alerting reference.

---

## Rollback Procedure

Memory v2 is designed for safe rollback. Since it uses a separate database and integrates via optional setter injection, rolling back is straightforward.

### Quick Rollback: Disable Extraction

The fastest way to disable memory is to set `Enabled: false` in the curator config:

```go
curatorConfig := memoryv2uc.CuratorConfig{
    Enabled: false,  // Disables extraction; recall returns empty if no cells exist
}
```

Rebuild and restart. This stops new memory creation while preserving existing data.

### Full Rollback: Remove Memory Integration

To completely remove memory v2 from the chat pipeline:

1. **Comment out the memory v2 initialization block** in `cmd/nuimanbot/main.go` (section `10.5`, approximately lines 206-275)

2. **Rebuild:**
   ```bash
   go build -o bin/nuimanbot ./cmd/nuimanbot
   ```

3. **Restart NuimanBot.** The application runs without memory, just as before. The memory database file remains on disk but is not opened.

### Data Cleanup (Optional)

If you want to remove all memory data after rollback:

```bash
# Option 1: Delete the memory database file
rm ./data/nuimanbot-memory.db

# Option 2: Keep the file but clear all data
sqlite3 ./data/nuimanbot-memory.db "DELETE FROM memory_cells; DELETE FROM memory_scenes; VACUUM;"
```

### Re-enabling After Rollback

To re-enable memory after a rollback:

1. Uncomment the memory v2 initialization block (or set `Enabled: true`)
2. Rebuild and restart
3. Existing memory data (if not deleted) is immediately available
4. If you deleted the database, a fresh one is created automatically

---

## FAQ

**Q: Does memory v2 replace the existing conversation memory?**
A: No. Memory v2 is an additional system that extracts long-term knowledge. The existing message history (conversations table) is unchanged and continues to work independently.

**Q: Will memory v2 slow down my chat responses?**
A: Minimally. Recall typically completes in < 1ms. Extraction happens after the response is sent, so it doesn't add latency to the user-visible response. The only added latency is the recall query before the LLM request.

**Q: Why does memory v2 use a separate database file?**
A: Memory v2 requires FTS5 (Full-Text Search 5), which is provided by the `modernc.org/sqlite` pure-Go driver. The main database uses `mattn/go-sqlite3`. Using separate files avoids driver conflicts.

**Q: What LLM costs should I expect?**
A: Extraction and consolidation use `claude-3-haiku-20240307` by default, which is the most cost-efficient model. Typical costs are fractions of a cent per interaction. You can switch to any supported model by changing `ExtractionModel` and `ConsolidationModel`.

**Q: Can I migrate memory data between environments?**
A: Yes. Use the export/import commands:
```bash
# Export from source
/memory export --conversation conv-123 > memories.json

# Import to destination
/memory import < memories.json
```

**Q: What happens if I run multiple NuimanBot instances?**
A: SQLite does not support concurrent writes from multiple processes well. Run only one instance per memory database file. For multi-instance deployments, use separate data directories.

**Q: Is memory data encrypted?**
A: The memory database is a standard SQLite file stored on disk. It is not encrypted at rest. For sensitive deployments, use filesystem-level encryption (e.g., LUKS, FileVault, BitLocker).

---

**Related Documentation:**
- [User Guide](self-organizing-memory-guide.md) - End-user guide for memory features
- [Admin Guide](../documentation/admin-guide-memory.md) - Administration and monitoring
- [Technical Details](../documentation/technical-details.md) - Architecture and system design
- [Architecture Diagrams](../documentation/diagrams/) - Visual architecture documentation
