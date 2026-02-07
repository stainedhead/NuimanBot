# Plugins Guide

## Overview

Plugins extend NuimanBot with third-party skills packaged and distributed independently.

## Plugin Structure

```
my-plugin/
├── plugin.yaml          # Manifest
└── skills/
    ├── skill1/
    │   └── SKILL.md
    └── skill2/
        └── SKILL.md
```

## Creating a Plugin

### plugin.yaml

```yaml
namespace: myorg/my-plugin
version: 1.0.0
description: My custom plugin
author: Your Name
license: MIT
skills:
  - skill1
  - skill2
dependencies:
  other-org/utils: ^1.0.0
```

### Namespace Format

- Format: `org/plugin-name`
- Lowercase alphanumeric with dashes/underscores
- Example: `acme/hello-world`

## Installing Plugins

```bash
# Install from local path
./bin/nuimanbot plugin install acme/hello /path/to/plugin

# List installed
./bin/nuimanbot plugin list

# Uninstall
./bin/nuimanbot plugin uninstall acme/hello
```

## Security

- Plugins run in sandboxed environment
- Resource limits enforced
- Permission model (future)

## Best Practices

1. Use semantic versioning
2. Document dependencies
3. Test thoroughly
4. Follow naming conventions
