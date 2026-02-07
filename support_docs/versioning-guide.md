# Skill Versioning Guide

## Overview

Skill versioning allows multiple versions of skills to coexist and ensures compatibility.

## Semantic Versioning

Skills use semantic versioning (semver): `MAJOR.MINOR.PATCH`

### Version Format

```
1.2.3
│ │ │
│ │ └─ Patch: Bug fixes
│ └─── Minor: New features (backward compatible)
└───── Major: Breaking changes
```

### Version in Frontmatter

```yaml
---
name: my-skill
version: 1.2.3
description: My skill
---
```

## Version Constraints

### Exact Version

```yaml
dependencies:
  other-skill: 1.0.0
```

### Caret (^) - Compatible With

```yaml
dependencies:
  other-skill: ^1.0.0  # Matches 1.x.x (same major)
```

### Tilde (~) - Approximately

```yaml
dependencies:
  other-skill: ~1.2.0  # Matches 1.2.x (same major.minor)
```

## Dependency Management

### Declaring Dependencies

```yaml
---
name: my-skill
version: 2.0.0
dependencies:
  acme/utils: ^1.0.0
  acme/core: ~2.1.0
---
```

### Resolution

The system automatically resolves to the latest compatible version.

## Best Practices

1. **Follow semver strictly**
2. **Document breaking changes** in major version bumps
3. **Test compatibility** with dependent skills
4. **Use constraints wisely** - prefer caret (^) for flexibility
