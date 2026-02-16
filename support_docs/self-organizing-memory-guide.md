# Self-Organizing Memory User Guide

## Overview

NuimanBot includes a **self-organizing long-term memory** system that automatically learns from your conversations. As you chat, the bot extracts important facts, decisions, preferences, and plans, organizing them into topic-based "scenes." On future conversations, relevant memories are automatically recalled and used to provide more personalized, context-aware responses.

**Key benefits:**
- The bot remembers what matters across conversations
- No manual setup required - memory works automatically
- You can browse, search, and manage your memories via CLI commands
- Expired or irrelevant memories can be pruned

## How Memory Works

### The Memory Lifecycle

```
1. You chat with NuimanBot
2. After each interaction, the bot extracts key information
3. Knowledge is stored as "memory cells" organized into "scenes"
4. Scene summaries are automatically consolidated
5. On your next conversation, relevant memories are recalled
6. Recalled memories help the bot give better, more informed responses
```

### Memory Cells

A **memory cell** is a single unit of knowledge extracted from a conversation. Each cell has:

| Property | Description |
|----------|-------------|
| **Type** | What kind of knowledge (see types below) |
| **Scene** | The topic it belongs to (e.g., "authentication", "project-setup") |
| **Salience** | How important it is (0.0 = trivial, 1.0 = critical) |
| **Content** | The actual knowledge, written clearly and specifically |

### Cell Types

| Type | What it captures | Example |
|------|-----------------|---------|
| **fact** | Objective information | "The project uses Go 1.22 with file-based storage" |
| **decision** | Choices that were made | "Decided to use JWT tokens with 24-hour expiry" |
| **task** | Action items or goals | "Need to implement rate limiting before launch" |
| **preference** | Your likes/dislikes/patterns | "User prefers TDD workflow with table-driven tests" |
| **plan** | Future strategies | "Will migrate to PostgreSQL in Q3 2026" |
| **risk** | Warnings or concerns | "API rate limit may be exceeded at current growth rate" |

### Scenes

A **scene** is a topic bucket that groups related memory cells. For example, all cells about authentication would be in the "authentication" scene. Each scene has a consolidated summary that captures the essence of all cells in that topic.

Scenes are created automatically based on the content of your conversations. Common scenes include:
- `project-setup` - Project configuration and setup decisions
- `user-preferences` - Your workflow and tool preferences
- `authentication` - Authentication-related facts and decisions
- `debugging` - Issues encountered and solutions found

### Memory Recall

When you start a new conversation, NuimanBot automatically searches its memory for relevant information:

1. **Full-text search** - Your message is used to find matching memories (ranked by relevance)
2. **Fallback** - If no text matches, the bot falls back to your highest-importance memories
3. **Token budget** - Only the most relevant memories are included (to keep responses focused)
4. **Context injection** - Recalled memories appear in the bot's context as structured information

You don't need to do anything for recall to work - it happens automatically.

## CLI Commands

You can manage your memories using `/memory` commands in the CLI interface.

### List Memory Cells

View stored memory cells with optional filters:

```
/memory list
/memory list --scene authentication
/memory list --type decision
/memory list --conversation conv-123
/memory list --limit 10
/memory list --scene project-setup --type fact --format json
```

**Flags:**
- `--scene <name>` - Filter by scene/topic
- `--type <type>` - Filter by cell type (fact, decision, task, preference, plan, risk)
- `--conversation <id>` - Filter by conversation ID
- `--limit <n>` - Maximum results to show
- `--format json|table` - Output format (default: table)

**Example output (table):**
```
ID        SCENE                 TYPE          SALIENCE  CONTENT
--------------------------------------------------------------------------------
a1b2c3d4  authentication        decision      0.90      Decided to use JWT tokens...
e5f6g7h8  project-setup         fact          0.85      Project uses Go 1.22 with...
i9j0k1l2  user-preferences      preference    0.80      User prefers TDD workflow...

3 cell(s) found.
```

### Get Cell Details

View full details of a specific memory cell:

```
/memory get a1b2c3d4-e5f6-7890-abcd-ef1234567890
/memory get a1b2c3d4-e5f6-7890-abcd-ef1234567890 --format json
```

**Example output:**
```
ID:              a1b2c3d4-e5f6-7890-abcd-ef1234567890
Scene:           authentication
Type:            decision
Salience:        0.90
Content:         Decided to use JWT tokens with 24-hour expiry for the API
Conversation ID: conv-123
Source:          ["msg-45", "msg-46"]
Created At:      2026-02-15T10:30:00Z
Updated At:      2026-02-15T10:30:00Z
```

### Search Memories

Full-text search across all memory cells:

```
/memory search authentication
/memory search "JWT tokens"
/memory search OAuth setup --limit 5
/memory search database migration --format json
```

**Flags:**
- `--limit <n>` - Maximum results (default: 20)
- `--format json|table` - Output format

**Example output:**
```
Search results for "JWT tokens" (2 matches):

ID        SCENE                 TYPE          SALIENCE  CONTENT
--------------------------------------------------------------------------------
a1b2c3d4  authentication        decision      0.90      Decided to use JWT tokens...
b2c3d4e5  authentication        fact          0.85      JWT refresh tokens stored...

2 cell(s) found.
```

### Delete a Memory Cell

Remove a specific memory cell by its ID:

```
/memory delete a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

**Output:**
```
Memory cell a1b2c3d4-e5f6-7890-abcd-ef1234567890 deleted successfully.
```

### List Scenes

View all scene summaries:

```
/memory scenes
/memory scenes --format json
```

**Example output:**
```
SCENE                      TOKENS   UPDATED               SUMMARY
------------------------------------------------------------------------------------------
authentication             12       2026-02-15 10:30:00    User configured OAuth2 auth...
project-setup              15       2026-02-15 09:15:00    Go 1.22 project with file-based storage...
user-preferences           8        2026-02-14 16:45:00    Prefers TDD, dark mode, vim...

3 scene(s) found.
```

### Prune Expired Cells

Remove all expired memory cells:

```
/memory prune
```

**Output:**
```
Pruned 5 expired memory cell(s).
```

Or if nothing is expired:
```
No expired cells found.
```

### Help

Show all available memory commands:

```
/memory help
```

## Understanding Salience

Salience is a score from 0.0 to 1.0 that indicates how important a memory cell is:

| Range | Meaning | Example |
|-------|---------|---------|
| **0.9 - 1.0** | Critical | Core architecture decisions, security requirements |
| **0.7 - 0.9** | Important | Key preferences, active tasks, project constraints |
| **0.5 - 0.7** | Moderate | Useful facts, secondary preferences |
| **0.3 - 0.5** | Low | Background information, minor observations |
| **0.0 - 0.3** | Trivial | Transient information, chit-chat details |

Higher-salience cells are prioritized during recall when the token budget is limited.

## Privacy and Data Storage

### Where Data is Stored

Memory data is stored locally in file-based JSON format:
```
./data/memory/
```

This directory contains:
- All memory cells (facts, decisions, preferences, etc.) as JSON files
- Scene summaries as JSON files
- Search index for memory lookup

### What Gets Remembered

The bot extracts **only important, actionable, or memorable information**. It skips:
- Trivial chit-chat or small talk
- Transient or one-off information
- Information that is not useful for future conversations

### Managing Your Data

You have full control over your memory data:
- **Browse**: Use `/memory list` and `/memory scenes` to see what's stored
- **Search**: Use `/memory search` to find specific memories
- **Delete**: Use `/memory delete <id>` to remove individual cells
- **Prune**: Use `/memory prune` to clean up expired cells

### Data Isolation

Memory cells are associated with conversation IDs. Each user's memories are separate and not shared with other users.

## Tips and Best Practices

1. **Be specific in conversations** - The more specific and clear your statements, the better the extracted memories will be. Saying "I prefer Go interfaces to be small, ideally 1-2 methods" produces better memories than "I like small interfaces."

2. **Review your memories periodically** - Use `/memory list` or `/memory scenes` to see what the bot has learned about you. Delete anything incorrect.

3. **Use search to verify** - Before asking the bot about a previous decision, check with `/memory search` to see if it's stored correctly.

4. **Prune regularly** - Run `/memory prune` occasionally to clean up expired cells and keep the database lean.

5. **Trust the automatic recall** - You don't need to remind the bot about previous conversations. If the information was important enough, it was extracted and will be recalled automatically.

## Frequently Asked Questions

**Q: Do I need to set up memory manually?**
A: No. Memory works automatically once NuimanBot is running. The bot extracts and recalls memories without any configuration.

**Q: Can I disable memory extraction?**
A: Yes. Memory extraction can be disabled in the application configuration. When disabled, no new memories are created, but existing ones can still be browsed and searched.

**Q: How much storage does memory use?**
A: Very little. Each memory cell is a short text (max 2000 characters). Thousands of cells typically use less than a few megabytes.

**Q: Will the bot forget things over time?**
A: Only if cells have an expiration date set. Most extracted cells are permanent unless you manually delete them.

**Q: What if the bot extracts something wrong?**
A: Use `/memory search` to find the incorrect cell, then `/memory delete <id>` to remove it. The bot will not use deleted cells in future recall.

**Q: Does memory slow down responses?**
A: Minimally. Memory recall typically takes a few milliseconds. The full-text search engine (FTS5) is optimized for fast retrieval.

**Q: Can I export my memory data?**
A: Use `/memory list --format json` to export all cells as JSON. For scenes, use `/memory scenes --format json`.

**Q: What happens if the memory database is corrupted?**
A: NuimanBot gracefully degrades - it continues working without memory. You won't get memory-augmented responses, but chat still functions normally.
