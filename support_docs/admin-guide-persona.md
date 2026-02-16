# Administrator Guide: Persona Customization System

**Version:** 1.0
**Last Updated:** 2026-02-15
**Audience:** System Administrators

---

## Table of Contents

1. [System Overview](#system-overview)
2. [Installation & Setup](#installation--setup)
3. [User Onboarding](#user-onboarding)
4. [Admin Policy Configuration](#admin-policy-configuration)
5. [Monitoring & Maintenance](#monitoring--maintenance)
6. [Security](#security)
7. [Performance Tuning](#performance-tuning)
8. [Troubleshooting](#troubleshooting)
9. [Backup & Recovery](#backup--recovery)
10. [CLI Reference](#cli-reference)

---

## System Overview

### Architecture

The persona customization system consists of four layers:

**Domain Layer:**
- `internal/domain/personafile.go` - PersonaFile entity
- `internal/domain/rulesconfig.go` - RulesConfig value object
- `internal/domain/memoryaction.go` - MemoryAction entity

**Infrastructure Layer:**
- `internal/infrastructure/persona/filerepository.go` - File I/O with caching
- `internal/infrastructure/persona/rulesparser.go` - YAML frontmatter parser
- `internal/infrastructure/persona/security.go` - Path validation and security
- `internal/infrastructure/audit/logger.go` - Audit logging

**Use Case Layer:**
- `internal/usecase/persona/promptcomposer.go` - System prompt composition
- `internal/usecase/persona/rulesenforcer.go` - Rule enforcement
- `internal/usecase/persona/memorywriter.go` - Memory write operations

**Adapter Layer:**
- `internal/adapter/cli/persona.go` - CLI commands
- Integration with ChatService and ToolService

### Data Flow

```
User Message → ChatService → PromptComposer → Persona Files (cached)
                              ↓
                         System Prompt (with persona context)
                              ↓
                         LLM Processing
                              ↓
                         Tool Execution → RulesEnforcer → RULES.md Check
                              ↓
                         Response to User
```

### File Structure

```
~/.nuimanbot/
└── personas/
    ├── user1/
    │   ├── SOUL.md      # AI personality
    │   ├── USER.md      # User context
    │   └── RULES.md     # Hard rules (YAML frontmatter)
    ├── user2/
    │   ├── SOUL.md
    │   ├── USER.md
    │   └── RULES.md
    └── ...
```

---

## Installation & Setup

### Prerequisites

- NuimanBot v1.3+ installed
- Go 1.24+ (for building from source)
- File system access for persona storage

### Enable Persona System

**1. Configuration File (config.yaml):**

```yaml
persona:
  enabled: true
  base_path: "~/.nuimanbot/personas"  # Expands to user's home directory
  cache_ttl: 15m                       # File cache duration
  token_budget:
    max_total: 4000    # Total tokens across all persona files
    max_per_file: 2000 # Maximum tokens per individual file
  templates:
    soul: "templates/SOUL.md"
    user: "templates/USER.md"
    rules: "templates/RULES.md"
```

**2. Environment Variables (Optional):**

```bash
# Override base path
export NUIMANBOT_PERSONA_PATH="/data/nuimanbot/personas"

# Override cache TTL (duration format: 1h, 30m, etc.)
export NUIMANBOT_PERSONA_CACHE_TTL="30m"
```

### Create Template Files

**1. Create templates directory:**

```bash
mkdir -p templates
```

**2. Create SOUL.md template:**

```bash
cat > templates/SOUL.md << 'EOF'
# AI Personality

You are a helpful AI assistant with expertise in various domains.

## Communication Style

- **Tone:** Professional and friendly
- **Verbosity:** Balanced - provide detail when needed
- **Format:** Clear structure with examples

## Expertise

- General knowledge and problem-solving
- Technical assistance and explanations
- Research and information gathering

## Behavior

- Ask clarifying questions when needed
- Provide accurate and helpful responses
- Admit when uncertain
- Suggest alternatives when appropriate
EOF
```

**3. Create USER.md template:**

```bash
cat > templates/USER.md << 'EOF'
# User Profile

**Name:** [Your Name]
**Role:** [Your Role]
**Timezone:** [Your Timezone]

## Preferences

- **Communication:** [Preferred style]
- **Technical Level:** [Beginner/Intermediate/Advanced]
- **Focus Areas:** [Your interests or work domains]

## Current Projects

[Describe what you're working on]

## Notes

[Any additional context the AI should know]
EOF
```

**4. Create RULES.md template:**

```bash
cat > templates/RULES.md << 'EOF'
---
blocked_tools: []
requires_confirmation: []
---

# Custom Rules

## Guidelines

[Your custom rules and preferences]

## Constraints

[Any specific limitations or requirements]
EOF
```

### Verify Installation

```bash
# Build the application
go build -o bin/nuimanbot ./cmd/nuimanbot

# Verify persona command is available
./bin/nuimanbot persona --help

# Initialize test user
./bin/nuimanbot persona init test-user

# Verify files were created
ls -la ~/.nuimanbot/personas/test-user/
```

---

## User Onboarding

### Initialize Persona Files

**Single User:**

```bash
./bin/nuimanbot persona init <user-id>
```

**Multiple Users:**

```bash
# From user list file
while read user_id; do
  ./bin/nuimanbot persona init "$user_id"
done < users.txt
```

**Bulk Initialization (All Users):**

```bash
# This command may be added in Phase 6 deployment
./bin/nuimanbot persona migrate --all
```

### Onboarding Workflow

1. **Create User Account** (if not exists)
   ```bash
   ./bin/nuimanbot user create <username> --role user
   ```

2. **Initialize Persona Files**
   ```bash
   ./bin/nuimanbot persona init <user-id>
   ```

3. **Verify File Creation**
   ```bash
   ls -la ~/.nuimanbot/personas/<user-id>/
   # Should show: SOUL.md, USER.md, RULES.md
   ```

4. **Guide User to Customize**
   - Send user the [User Guide](user-guide-persona.md)
   - Provide examples relevant to their role
   - Explain available tools and rules

5. **Test Persona Integration**
   ```bash
   # User sends a test message
   # Verify persona context is included in system prompt
   # Check logs for persona file reads
   ```

### User Offboarding

**Remove Persona Files:**

```bash
rm -rf ~/.nuimanbot/personas/<user-id>/
```

**Note:** Persona files are separate from user accounts. Deleting persona files does NOT delete the user account.

---

## Admin Policy Configuration

### Overview

Administrators can set **global policies** that override user rules. This ensures organization-wide compliance.

### Setting Admin Policies

**Code Configuration (internal/config/persona.go):**

```go
// Example: Global admin policy
adminPolicy := &domain.RulesConfig{
    BlockedTools: []string{
        "production_deploy",     // Block production deployments
        "database_migration",    // Block database migrations
        "credential_delete",     // Block credential deletion
    },
    RequiresConfirmation: []string{
        "external_api",          // Require confirmation for external APIs
        "filesystem_delete",     // Require confirmation for file deletion
    },
}

// Pass to RulesEnforcer
enforcer := persona.NewRulesEnforcer(repo, parser, adminPolicy)
```

**Configuration File (config.yaml):**

```yaml
persona:
  admin_policy:
    blocked_tools:
      - production_deploy
      - database_migration
      - credential_delete
    requires_confirmation:
      - external_api
      - filesystem_delete
```

### Policy Precedence

**Priority (highest to lowest):**
1. **Admin Policy** - Cannot be overridden by users
2. **User RULES.md** - User-specific rules
3. **Default** - Allow all tools

**Merging Logic:**
- Admin `blocked_tools` + User `blocked_tools` = Combined block list
- Admin `requires_confirmation` + User `requires_confirmation` = Combined confirmation list
- Admin blocks always apply (users cannot unblock admin-blocked tools)

### Common Policy Scenarios

**Scenario 1: Security-Conscious Organization**
```yaml
admin_policy:
  blocked_tools:
    - credential_use
    - production_deploy
    - database_migration
  requires_confirmation:
    - external_api
    - github_push
    - filesystem_delete
```

**Scenario 2: Development Team (Permissive)**
```yaml
admin_policy:
  blocked_tools: []
  requires_confirmation:
    - production_deploy
```

**Scenario 3: Compliance-Driven (Strict)**
```yaml
admin_policy:
  blocked_tools:
    - external_api
    - credential_use
    - filesystem_write
    - network_access
  requires_confirmation:
    - github
    - repo_search
```

---

## Monitoring & Maintenance

### Health Checks

**Verify Persona System:**

```bash
# Check if persona files are being loaded
tail -f /var/log/nuimanbot/app.log | grep "persona"

# Look for:
# - "Loading persona file: <user-id>/SOUL.md"
# - "Cache hit: <user-id>:SOUL"
# - "Composed prompt: <user-id> (tokens: 1234)"
```

### Metrics to Monitor

**Performance Metrics:**
- `persona_file_reads_total` - Total file reads
- `persona_cache_hits_total` - Cache hit rate (target: >90%)
- `persona_cache_misses_total` - Cache miss rate
- `persona_compose_duration_seconds` - PromptComposer latency (target: <100ms)
- `persona_rules_enforcement_duration_seconds` - RulesEnforcer latency (target: <10ms)

**Security Metrics:**
- `persona_path_traversal_attempts_total` - Path traversal attacks
- `persona_symlink_blocks_total` - Symlink attacks blocked
- `persona_rule_violations_total` - Blocked tool usage attempts

**Error Metrics:**
- `persona_file_read_errors_total` - File read failures
- `persona_parse_errors_total` - YAML parsing failures
- `persona_validation_errors_total` - Validation failures

### Log Analysis

**Check for security violations:**

```bash
grep "path traversal" /var/log/nuimanbot/app.log
grep "symlink blocked" /var/log/nuimanbot/app.log
grep "rule violation" /var/log/nuimanbot/app.log
```

**Check for performance issues:**

```bash
grep "persona_compose_duration" /var/log/nuimanbot/app.log | \
  awk '{print $NF}' | \
  sort -n | \
  tail -20  # Show slowest 20 composes
```

**Check cache hit rate:**

```bash
# Over last 1000 requests
grep "persona" /var/log/nuimanbot/app.log | \
  grep -E "(cache hit|cache miss)" | \
  tail -1000 | \
  awk '{print $0}' | \
  sort | uniq -c
```

### Maintenance Tasks

**Weekly:**
- Review security metrics for anomalies
- Check error logs for parsing failures
- Verify cache hit rate is >80%
- Review disk usage for persona directories

**Monthly:**
- Audit admin policy effectiveness
- Review user persona files for compliance
- Analyze performance trends
- Clean up deleted user files

**Quarterly:**
- Update templates based on user feedback
- Review and update documentation
- Performance optimization review
- Security audit

---

## Security

### Threat Model

**Protected Against:**
- ✅ Path traversal attacks (30+ test cases)
- ✅ Symlink attacks (follows symlinks blocked)
- ✅ Cross-user file access
- ✅ Null byte injection
- ✅ Large file DoS (100KB limit)
- ✅ Cache poisoning

**Admin Responsibilities:**
- File system permissions
- Network access controls
- Backup encryption
- User authentication

### Security Best Practices

**1. File System Permissions:**

```bash
# Persona directory: Only NuimanBot can write
chown -R nuimanbot:nuimanbot ~/.nuimanbot/personas
chmod 755 ~/.nuimanbot/personas

# User directories: Readable by NuimanBot and user
chmod 755 ~/.nuimanbot/personas/*/
chmod 644 ~/.nuimanbot/personas/*/*.md
```

**2. Audit Logging:**

All security events are logged to the audit system:
- Path traversal attempts
- Symlink blocks
- Rule violations
- Unauthorized access attempts

**Review audit logs:**
```bash
# View last 100 security events
./bin/nuimanbot audit list --type security --limit 100

# View events for specific user
./bin/nuimanbot audit list --user <user-id> --type security
```

**3. Regular Security Scans:**

```bash
# Check for world-writable files
find ~/.nuimanbot/personas -type f -perm /o+w

# Check for suspicious symlinks
find ~/.nuimanbot/personas -type l

# Check for files outside expected pattern
find ~/.nuimanbot/personas -type f ! -name "*.md"
```

**4. Admin Policy Enforcement:**

Ensure critical tools are always blocked at admin level:
```yaml
admin_policy:
  blocked_tools:
    - production_deploy    # Prevent accidental deployments
    - database_migration   # Require manual migration
    - credential_delete    # Prevent credential loss
```

### Security Incident Response

**If path traversal attempt detected:**

1. Review audit logs for user ID and timestamp
2. Check if attempt was successful (should be blocked)
3. Review user's RULES.md for compliance
4. Contact user to understand intent
5. Consider stricter admin policy if needed

**If symlink attack detected:**

1. Immediately check affected user directory
2. Remove any symlinks: `find ~/.nuimanbot/personas/<user-id>/ -type l -delete`
3. Verify no data exfiltration occurred
4. Review logs for attack pattern
5. Consider file system monitoring

---

## Performance Tuning

### Cache Configuration

**Default Settings:**
```yaml
persona:
  cache_ttl: 15m  # 15 minutes
```

**Tuning Guidelines:**

**High-traffic systems:**
```yaml
cache_ttl: 30m  # Longer cache = less disk I/O
```

**Frequently-updated files:**
```yaml
cache_ttl: 5m   # Shorter cache = faster updates
```

**No cache (development):**
```yaml
cache_ttl: 0s   # Disable cache for testing
```

### Token Budget Optimization

**Default Budgets:**
```yaml
token_budget:
  max_total: 4000    # Total across all files
  max_per_file: 2000 # Per individual file
```

**For Large LLMs (Claude Opus, GPT-4):**
```yaml
token_budget:
  max_total: 8000
  max_per_file: 3000
```

**For Small LLMs (Haiku, GPT-3.5):**
```yaml
token_budget:
  max_total: 2000
  max_per_file: 1000
```

### Benchmarking

**Run performance benchmarks:**

```bash
# Benchmark PromptComposer
go test -bench=BenchmarkPromptComposer ./internal/usecase/persona/

# Benchmark RulesEnforcer
go test -bench=BenchmarkRulesEnforcer ./internal/usecase/persona/

# Benchmark with CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./internal/usecase/persona/
go tool pprof cpu.prof
```

**Target Benchmarks:**
- PromptComposer: <100ms (actual: ~252ns - 400,000x faster)
- RulesEnforcer: <10ms (actual: ~42ns - 238,000x faster)
- Cache hit rate: >90%

---

## Troubleshooting

### Files Not Loading

**Symptom:** User reports AI not using their persona

**Diagnosis:**

```bash
# 1. Verify files exist
ls -la ~/.nuimanbot/personas/<user-id>/

# 2. Check file permissions
ls -l ~/.nuimanbot/personas/<user-id>/*.md

# 3. Check logs for errors
grep "persona.*<user-id>" /var/log/nuimanbot/app.log | tail -20

# 4. Verify persona system is enabled
grep "persona.enabled" config.yaml
```

**Solutions:**

```bash
# Re-initialize files
./bin/nuimanbot persona init <user-id> --force

# Fix permissions
chmod 644 ~/.nuimanbot/personas/<user-id>/*.md

# Restart NuimanBot (clears cache)
systemctl restart nuimanbot
```

### YAML Parsing Errors

**Symptom:** RULES.md not being enforced

**Diagnosis:**

```bash
# Check for YAML syntax errors
grep "YAML parse error" /var/log/nuimanbot/app.log
```

**Common Issues:**

```yaml
# ❌ Wrong: missing hyphen
blocked_tools:
  dangerous_tool

# ✅ Correct
blocked_tools:
  - dangerous_tool

# ❌ Wrong: using tabs instead of spaces
blocked_tools:
→	- tool_name  # Tab character

# ✅ Correct: use spaces
blocked_tools:
  - tool_name  # Two spaces
```

**Solution:**

```bash
# Validate YAML
yamllint ~/.nuimanbot/personas/<user-id>/RULES.md

# Or use online validator
cat ~/.nuimanbot/personas/<user-id>/RULES.md | \
  curl -X POST -d @- https://yaml-online-parser.appspot.com/
```

### Performance Degradation

**Symptom:** Slow response times

**Diagnosis:**

```bash
# Check PromptComposer latency
grep "persona_compose_duration" /var/log/nuimanbot/app.log | \
  tail -100 | \
  awk '{print $NF}' | \
  sort -n

# Check cache hit rate
grep "persona.*cache" /var/log/nuimanbot/app.log | \
  tail -100 | \
  grep -c "hit"
```

**Solutions:**

1. **Increase cache TTL:**
   ```yaml
   cache_ttl: 30m  # From 15m
   ```

2. **Reduce token budgets:**
   ```yaml
   token_budget:
     max_total: 2000   # From 4000
     max_per_file: 1000 # From 2000
   ```

3. **Optimize large files:**
   ```bash
   # Find files >50KB
   find ~/.nuimanbot/personas -name "*.md" -size +50k

   # Ask users to reduce file size
   ```

### Security Alerts

**Path Traversal Detected:**

```bash
# Find offending user
grep "path traversal" /var/log/nuimanbot/app.log | \
  grep -oP 'user_id=\K[^ ]+'

# Review user's recent activity
./bin/nuimanbot audit list --user <user-id> --limit 50
```

**Symlink Attack Blocked:**

```bash
# Locate symlink
find ~/.nuimanbot/personas/<user-id>/ -type l

# Remove symlink
find ~/.nuimanbot/personas/<user-id>/ -type l -delete

# Notify user and investigate intent
```

---

## Backup & Recovery

### Backup Strategy

**What to Back Up:**
- All persona files: `~/.nuimanbot/personas/`
- Templates: `templates/SOUL.md`, `USER.md`, `RULES.md`
- Configuration: `config.yaml`

**Backup Frequency:**
- **Daily:** Automated backup of all persona files
- **Before upgrades:** Manual backup
- **On-demand:** When requested by users

### Backup Commands

**Full backup:**

```bash
# Create timestamped backup
tar -czf persona-backup-$(date +%Y%m%d).tar.gz \
  ~/.nuimanbot/personas/ \
  templates/ \
  config.yaml

# Verify backup
tar -tzf persona-backup-$(date +%Y%m%d).tar.gz | head -20
```

**Incremental backup (rsync):**

```bash
# Sync to backup server
rsync -avz --delete \
  ~/.nuimanbot/personas/ \
  backup-server:/backups/nuimanbot/personas/
```

**Version control (Git):**

```bash
cd ~/.nuimanbot/personas
git init
git add .
git commit -m "Persona backup $(date +%Y-%m-%d)"
git push origin main
```

### Recovery Procedures

**Restore single user:**

```bash
# Extract from backup
tar -xzf persona-backup-20260215.tar.gz \
  .nuimanbot/personas/<user-id>/

# Verify restored files
ls -la ~/.nuimanbot/personas/<user-id>/
```

**Restore all users:**

```bash
# Extract full backup
tar -xzf persona-backup-20260215.tar.gz -C ~/

# Verify restoration
find ~/.nuimanbot/personas -name "*.md" | wc -l
```

**Recover deleted file:**

```bash
# From backup
tar -xzf persona-backup-20260215.tar.gz \
  .nuimanbot/personas/<user-id>/SOUL.md

# From version control
cd ~/.nuimanbot/personas
git checkout <user-id>/SOUL.md
```

### Disaster Recovery

**Complete system loss:**

1. Reinstall NuimanBot
2. Restore configuration: `config.yaml`
3. Restore templates: `templates/`
4. Restore persona files: `~/.nuimanbot/personas/`
5. Verify permissions and ownership
6. Restart NuimanBot
7. Test with sample user

**Corruption detected:**

1. Stop NuimanBot
2. Identify corrupted files
3. Restore from most recent backup
4. Verify file integrity
5. Clear cache (restart NuimanBot)
6. Monitor logs for errors

---

## CLI Reference

### persona init

Initialize persona files for a user from templates.

```bash
./bin/nuimanbot persona init <user-id> [flags]
```

**Flags:**
- `--force` - Overwrite existing files
- `--template-dir <path>` - Use custom template directory

**Examples:**

```bash
# Initialize new user
./bin/nuimanbot persona init alice

# Reinitialize (overwrite existing)
./bin/nuimanbot persona init alice --force

# Use custom templates
./bin/nuimanbot persona init alice --template-dir /custom/templates
```

### persona list

List all users with persona files (future command).

```bash
./bin/nuimanbot persona list
```

**Output:**
```
USER_ID          SOUL    USER    RULES   LAST_MODIFIED
alice            ✓       ✓       ✓       2026-02-15 10:30
bob              ✓       ✓       ✗       2026-02-14 15:20
charlie          ✗       ✗       ✗       -
```

### persona validate

Validate persona file syntax (future command).

```bash
./bin/nuimanbot persona validate <user-id>
```

**Checks:**
- YAML frontmatter syntax
- File size limits
- Required fields
- Tool name validity

---

## Appendix: System Specifications

### File Limits

- **Maximum file size:** 100KB per file
- **Token budget:** 4000 tokens total, 2000 per file (configurable)
- **Cache TTL:** 15 minutes (configurable)
- **File types:** `.md` only

### Security Features

- ✅ Path traversal prevention (30+ attack vectors tested)
- ✅ Symlink attack blocking
- ✅ Cross-user isolation
- ✅ Null byte rejection
- ✅ Admin policy override
- ✅ Comprehensive audit logging

### Performance

- **PromptComposer:** 252ns average (400,000x faster than 100ms target)
- **RulesEnforcer:** 42ns average (238,000x faster than 10ms target)
- **Cache hit rate:** >90% typical
- **File I/O:** <1ms for cached reads

---

## Support

**Documentation:**
- [User Guide](user-guide-persona.md)
- [Product Details](product-details.md)
- [Security Audit Report](../specs/user-customization/security-audit-report.md)

**Issue Reporting:**
- GitHub: https://github.com/anthropics/nuimanbot/issues

**Contact:**
- Email: support@nuimanbot.example.com
- Slack: #nuimanbot-admin

---

**Document Version:** 1.0
**Last Updated:** 2026-02-15
**Maintained By:** NuimanBot Team
