# Subagents Guide

## Overview

Subagents are autonomous execution contexts that allow skills to run independently in the background, making multi-step decisions and using tools without blocking the main conversation flow.

## What Are Subagents?

A **subagent** is a forked copy of your conversation context that executes autonomously to complete a specific task. Think of it as spawning a separate agent that:

- Has its own conversation history (copied from the parent)
- Can make multiple tool calls independently
- Runs in the background without blocking your main session
- Has strict resource limits (time, tokens, tool calls)
- Reports results back when complete

## When to Use Subagents

Use subagents (`context: fork`) for tasks that:

1. **Take Multiple Steps**: Require several tool calls and decisions
2. **Are Self-Contained**: Don't need to interact with the user mid-task
3. **Can Run Independently**: Don't require real-time feedback
4. **Have Clear Objectives**: Can be completed autonomously

### Good Use Cases

- **Code Analysis**: "Analyze this bug and trace the root cause"
- **Research Tasks**: "Find all usages of this API and document them"
- **Refactoring**: "Identify code duplication and suggest consolidation"
- **Testing**: "Generate comprehensive test cases for this module"
- **Documentation**: "Audit code comments and suggest improvements"

### Bad Use Cases

- **Interactive Tasks**: Requiring user input or clarification
- **Simple Queries**: Single tool call tasks
- **Real-Time Feedback**: User wants to see each step
- **Exploratory Work**: Direction changes based on findings

## Creating a Subagent Skill

Add `context: fork` to your skill frontmatter:

```yaml
---
name: my-autonomous-skill
description: Does something autonomously
user-invocable: true
context: fork
allowed-tools:
  - read
  - grep
  - glob
---

# Your skill prompt here

Work autonomously to complete this task...
```

### Key Frontmatter Fields

- **`context: fork`** (required): Marks skill as subagent
- **`allowed-tools`** (recommended): Restrict which tools the subagent can use
- **`user-invocable: true`**: Allow users to invoke with `/skill-name`

## Resource Limits

Subagents have strict resource limits to prevent runaway execution:

| Limit | Default Value | Purpose |
|-------|---------------|---------|
| **Timeout** | 5 minutes | Maximum execution time |
| **Max Tokens** | 100,000 | Token budget (input + output) |
| **Max Tool Calls** | 50 | Maximum number of tool invocations |

These limits ensure subagents:
- Don't consume excessive resources
- Complete in a reasonable time
- Can't run indefinitely

## Invoking a Subagent Skill

From the CLI:

```bash
./bin/nuimanbot skill execute debug-issue "why is the login failing?"
```

From chat:

```
/debug-issue why is the login failing?
```

### What Happens

1. **Skill Executed**: System detects `context: fork`
2. **Subagent Created**: Forked context with conversation history
3. **Background Execution**: Runs asynchronously
4. **Notification**: User receives subagent ID
5. **Autonomous Work**: Subagent makes tool calls independently
6. **Completion**: Results available via status check

### Sample Output

```
Started subagent: debug-issue (ID: subagent-debug-issue-1770501588117047000)
Use /subagent-status subagent-debug-issue-1770501588117047000 to check progress
```

## Monitoring Subagents

### Check Status

```bash
./bin/nuimanbot subagent status <subagent-id>
```

**Output:**
```
Subagent: subagent-debug-issue-1770501588117047000
Status: complete
Execution Time: 2m 34s
Tokens Used: 15,432 / 100,000
Tool Calls: 12 / 50

=== Output ===
# Investigation Report
...
```

### List Running Subagents

```bash
./bin/nuimanbot subagent list
```

**Output:**
```
Running subagents (2):
  subagent-debug-issue-1770501588117047000 - running
  subagent-analyze-api-1770501699234567000 - complete
```

### Cancel a Subagent

```bash
./bin/nuimanbot subagent cancel <subagent-id>
```

## Subagent Lifecycle

```
┌─────────────────┐
│   User Invokes  │
│   Fork Skill    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Context Forked │ ◄─── Deep copy of conversation
│  Subagent Created│      history and settings
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Background      │ ◄─── Runs in goroutine
│ Execution Starts│      Non-blocking
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Autonomous     │ ◄─── Makes tool calls
│  Multi-Step Loop│      autonomously
└────────┬────────┘
         │
         ├─────► Timeout ───┐
         ├─────► Cancelled ─┤
         ├─────► Error ─────┤
         │                  │
         ▼                  ▼
    ┌─────────┐      ┌──────────┐
    │Complete │      │ Terminated│
    └─────────┘      └──────────┘
```

## Best Practices

### Writing Effective Subagent Skills

1. **Clear Objectives**: Provide a specific, achievable goal
2. **Structured Output**: Ask for formatted results (markdown, JSON)
3. **Step-by-Step Guidance**: Outline the investigation approach
4. **Tool Restrictions**: Limit `allowed-tools` to minimum necessary
5. **Success Criteria**: Define what "done" looks like

### Example: Good vs Bad Prompts

❌ **Bad** (too vague):
```
Fix any bugs you find.
```

✅ **Good** (specific and structured):
```
Investigate the authentication timeout issue:
1. Find all auth-related code
2. Check timeout configurations
3. Identify where delays occur
4. Provide fix recommendations
```

## Tool Restrictions

Use `allowed-tools` to limit what subagents can do:

```yaml
allowed-tools:
  - read      # Read files
  - grep      # Search code
  - glob      # Find files
  # Excluded: write, bash (dangerous for autonomous use)
```

**Security Note**: Avoid allowing `write`, `edit`, or unrestricted `bash` commands in subagent skills unless absolutely necessary.

## Error Handling

Subagents can terminate with different statuses:

| Status | Meaning | Next Steps |
|--------|---------|------------|
| **complete** | Successfully finished | Review output |
| **timeout** | Exceeded 5-minute limit | Check partial output, consider simplifying task |
| **error** | Execution error | Check error message, fix skill or inputs |
| **cancelled** | User cancelled | No output available |
| **resource_limit** | Hit token/tool limit | Increase limits or simplify task |

## Advanced: Customizing Resource Limits

Currently, resource limits are set to defaults. Future versions will support:

```yaml
resource-limits:
  timeout: 10m
  max-tokens: 200000
  max-tool-calls: 100
```

## Troubleshooting

### Subagent Times Out

**Problem**: Execution exceeds 5 minutes

**Solutions**:
- Simplify the task scope
- Break into multiple smaller skills
- Optimize prompts to reduce tool calls

### Subagent Uses Too Many Tool Calls

**Problem**: Hits 50 tool call limit

**Solutions**:
- Provide more focused guidance
- Use grep/glob patterns to narrow searches
- Combine related operations

### Subagent Returns Incomplete Results

**Problem**: Partial output, unclear conclusion

**Solutions**:
- Add structured output requirements
- Provide clearer success criteria
- Request step-by-step documentation

## Examples

### Example 1: Debug Skill

See `data/skills/examples/debug-issue/SKILL.md` for a complete autonomous debugging skill.

### Example 2: Code Analysis

```yaml
---
name: analyze-dependencies
description: Find and document all dependencies for a module
context: fork
allowed-tools: [read, grep, glob]
---

Analyze dependencies for: $0

1. Find all import statements
2. Build dependency graph
3. Identify circular dependencies
4. List external vs internal deps
5. Suggest optimization opportunities
```

### Example 3: Test Generation

```yaml
---
name: generate-tests
description: Generate comprehensive test cases
context: fork
allowed-tools: [read, grep, write]
---

Generate tests for: $0

1. Read source code
2. Identify public methods
3. Determine edge cases
4. Write test functions
5. Include success/error scenarios
```

## Future Enhancements

Planned features for subagents:

- **Custom Resource Limits**: Override defaults per skill
- **Progress Callbacks**: Real-time status updates
- **Nested Subagents**: Subagents that spawn subagents
- **Result Streaming**: See partial results as they're generated
- **Persistent State**: Save/resume subagent work

---

**Learn More**:
- [Skills System Overview](./skills-guide.md)
- [Creating Custom Skills](./custom-skills.md)
- [Tool Permissions](./tool-permissions.md)
