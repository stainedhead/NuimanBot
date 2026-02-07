# Migration Guide: Skills → Tools Terminology

## Overview

As of version 1.1.0, NuimanBot has renamed "Skills" to "Tools" throughout the codebase to align with industry standards used by OpenAI, Anthropic, Google, LangChain, and LlamaIndex.

**Why this change?**
- **Industry Alignment**: LLM APIs universally use "tools" for executable functions
- **Avoid Confusion**: Anthropic uses "Skills" for markdown-based saved prompts (a different concept)
- **Consistency**: Our terminology now matches what developers expect from LLM documentation

## What Changed?

### Terminology Mapping

| Old Term (Deprecated) | New Term | Description |
|----------------------|----------|-------------|
| Skill | Tool | Executable function the LLM can call |
| Skills System | Tools System | Framework for tool execution |
| Skill Registry | Tool Registry | Catalog of available tools |
| Built-in Skills | Built-in Tools | Core tools (calculator, datetime, etc.) |

### Code Changes

**No action required for most users** - these are internal changes:
- Domain types: `Skill` → `Tool`, `SkillResult` → `ExecutionResult`, `SkillConfig` → `ToolConfig`
- Package paths: `internal/skills/` → `internal/tools/`, `internal/usecase/skill/` → `internal/usecase/tool/`
- All built-in tools updated (calculator, datetime, weather, websearch, notes, github, repo_search, doc_summarize, summarize, coding_agent)

## Action Required

### 1. Update Configuration File

**Change your `config.yaml` from:**
```yaml
skills:
  entries:
    calculator:
      enabled: true
    datetime:
      enabled: true
```

**To:**
```yaml
tools:
  entries:
    calculator:
      enabled: true
    datetime:
      enabled: true
```

### 2. Update Environment Variables (if used)

**Old format (deprecated):**
```bash
NUIMANBOT_SKILLS_ENTRIES_CALCULATOR_APIKEY=xxx
```

**New format:**
```bash
NUIMANBOT_TOOLS_ENTRIES_CALCULATOR_APIKEY=xxx
```

### 3. No Code Changes Needed

All tool functionality remains identical. The rename is purely cosmetic for consistency.

## Backward Compatibility

### Version 1.1.0
- **Both config keys supported**: You can use either `skills` or `tools` in your config
- **Deprecation warning**: Using `skills` will log a warning message
- **Recommendation**: Update to `tools` to avoid future issues

### Future Versions (2.0.0+)
- **Breaking change**: The `skills` config key will be removed
- **Action required**: Must use `tools` in configuration

## Migration Steps

### Quick Migration (< 1 minute)

```bash
# 1. Update config.yaml
sed -i 's/^skills:/tools:/g' config.yaml

# 2. Update environment variables (if using .env)
sed -i 's/NUIMANBOT_SKILLS_/NUIMANBOT_TOOLS_/g' .env

# 3. Restart NuimanBot
./bin/nuimanbot
```

### Verify Migration

After updating your configuration:

1. Start NuimanBot
2. Check logs - you should NOT see any deprecation warnings
3. Test tool execution - all tools should work identically

## Custom Tool Development

If you've developed custom tools, update your code:

**Before:**
```go
import "nuimanbot/internal/domain"

type MySkill struct {
    config domain.SkillConfig
}

func (s *MySkill) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
    return &domain.SkillResult{
        Output: "result",
    }, nil
}
```

**After:**
```go
import "nuimanbot/internal/domain"

type MyTool struct {
    config domain.ToolConfig
}

func (t *MyTool) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
    return &domain.ExecutionResult{
        Output: "result",
    }, nil
}
```

## Need Help?

- **Documentation**: See updated docs in `documentation/` directory
- **Issues**: Report problems at https://github.com/anthropics/nuimanbot/issues
- **Questions**: Check AGENTS.md for development guidelines

## Summary

✅ **Simple change**: Just update `skills:` to `tools:` in your config
✅ **Backward compatible**: Both keys work in v1.1.0
✅ **No functional changes**: All tools work exactly the same
⚠️ **Action needed**: Update before v2.0.0 to avoid breaking changes
