---
name: test-skill
description: A test skill to demonstrate the Agent Skills system
user-invocable: true
allowed-tools:
  - read_file
  - grep
---

# Test Skill

This is a test skill that demonstrates the Agent Skills system.

## Usage

You can invoke this skill with:
```
/test-skill <arguments>
```

## Arguments

This skill accepts the following arguments:
- First argument: $0
- Second argument: $1
- All arguments: $ARGUMENTS

## Example

If you invoke `/test-skill file1.go file2.go`, this skill will process:
- File 1: $0
- File 2: $1
- All files: $ARGUMENTS

## Instructions

When this skill is activated, please:
1. Acknowledge that you received the arguments: $ARGUMENTS
2. List the allowed tools: read_file, grep
3. Provide helpful assistance with the given arguments
