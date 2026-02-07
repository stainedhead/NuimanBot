# Skill Memory Guide

## Overview

Skill memory allows skills to persist data between invocations, enabling stateful workflows.

## Memory Scopes

### Skill Scope

Memory specific to a single skill:

```go
// Store
api.Remember("my-skill", "last-run", time.Now(), domain.MemoryScopeSkill)

// Retrieve
var lastRun time.Time
api.Recall("my-skill", "last-run", domain.MemoryScopeSkill, &lastRun)
```

### User Scope

Memory per user (future):

```go
api.Remember("my-skill", "preferences", userPrefs, domain.MemoryScopeUser)
```

### Global Scope

Shared across all invocations:

```go
api.Remember("my-skill", "total-runs", 42, domain.MemoryScopeGlobal)
```

### Session Scope

Temporary session memory:

```go
api.Remember("my-skill", "temp-data", data, domain.MemoryScopeSession)
```

## Usage in Skills

Skills can access memory during execution:

```markdown
---
name: stateful-skill
description: A skill with memory
---

# Stateful Skill

Last run: [Retrieved from memory]

Current count: [Incremented from memory]
```

## Memory API

### Remember

```go
err := memoryAPI.Remember(
    "skill-name",
    "key",
    value,          // Any JSON-serializable value
    domain.MemoryScopeSkill,
)
```

### Recall

```go
var value MyType
err := memoryAPI.Recall(
    "skill-name",
    "key",
    domain.MemoryScopeSkill,
    &value,
)
```

### Forget

```go
err := memoryAPI.Forget(
    "skill-name",
    "key",
    domain.MemoryScopeSkill,
)
```

## Expiration

Memory can have TTL:

```go
memory := &domain.SkillMemory{
    SkillName: "my-skill",
    Key:       "temp-key",
    Value:     data,
    Scope:     domain.MemoryScopeSkill,
    ExpiresAt: &expirationTime,
}
storage.Set(memory)
```

## Storage

Memory is stored in SQLite:

```
~/.nuimanbot/skill_memory.db
```

## Cleanup

Expired memory is automatically cleaned up periodically.

## Best Practices

1. **Use appropriate scopes** - Skill scope for skill-specific data
2. **Set expiration** for temporary data
3. **Keep values small** - Avoid storing large objects
4. **Handle missing keys** gracefully
