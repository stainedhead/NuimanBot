# NuimanBot Migration Guide

**Version:** 1.0
**Last Updated:** 2026-02-08
**Target Audience:** System Administrators, DevOps Engineers

---

## Table of Contents

1. [Overview](#overview)
2. [Migration Scope](#migration-scope)
3. [Pre-Migration Checklist](#pre-migration-checklist)
4. [Migration Steps](#migration-steps)
5. [Post-Migration Verification](#post-migration-verification)
6. [Rollback Procedures](#rollback-procedures)
7. [Common Migration Issues](#common-migration-issues)

---

## Overview

This guide covers migration to the improved admin features architecture, which includes:

- **Enhanced User Management**: File-based user profile storage (`users.json`)
- **Bot Configuration Management**: Database-driven bot configs (`bots.json`)
- **REST API**: Complete administrative REST API
- **Improved Configuration**: Flexible paths, hot reload, provider inheritance

### What's New

**Architecture Changes:**
- User profiles now stored in JSON format with rich metadata
- Bot configurations moved from YAML to encrypted JSON storage
- Separate paths for config, data, and logs
- Hot configuration reload without restart

**New Features:**
- Comprehensive REST API for administration
- Multi-platform user identities (Slack, Telegram, CLI)
- Agent personalization per user
- Public/private bot management
- API key-based authentication
- Role-based access control

### Migration Impact

**Breaking Changes:**
- User storage format changed (SQLite users table → users.json)
- Bot configuration format changed (config.yaml → bots.json)
- New encryption key requirement for bot tokens

**Backward Compatibility:**
- Existing conversations preserved (SQLite database unchanged)
- Existing tool configurations compatible
- LLM provider configurations compatible (with minor updates)

---

## Migration Scope

### What Gets Migrated

✅ **User Data:**
- User IDs and usernames
- Email addresses
- User roles (admin, user, guest)
- Created/updated timestamps

✅ **Bot Configurations:**
- Slack bot tokens and configuration
- Telegram bot tokens and configuration
- Bot names and descriptions
- Enable/disable status

✅ **System Configuration:**
- LLM provider settings
- Gateway settings
- Logging configuration

### What Doesn't Migrate Automatically

⚠️ **Manual Migration Required:**
- User preferences (need manual mapping)
- Platform IDs (Slack/Telegram user IDs)
- Agent personalization settings (new feature)
- Custom user metadata

❌ **Not Migrated:**
- Conversation history (already in SQLite, no migration needed)
- Tool execution history (already in SQLite, no migration needed)

---

## Pre-Migration Checklist

### 1. Backup Everything

**Critical: Create backups before starting migration!**

```bash
# Create backup directory
mkdir -p backups/$(date +%Y%m%d)

# Backup database
cp data/nuimanbot.db backups/$(date +%Y%m%d)/nuimanbot.db

# Backup config
cp config.yaml backups/$(date +%Y%m%d)/config.yaml

# Backup entire data directory
tar -czf backups/$(date +%Y%m%d)/data-backup.tar.gz data/
```

### 2. Document Current State

```bash
# Export current users (if using old system)
sqlite3 data/nuimanbot.db "SELECT * FROM users;" > backups/$(date +%Y%m%d)/users-export.csv

# List current bot configurations
grep -A 10 "slack:" config.yaml > backups/$(date +%Y%m%d)/old-bot-config.txt
grep -A 10 "telegram:" config.yaml >> backups/$(date +%Y%m%d)/old-bot-config.txt

# Document API keys
# Manually note down any existing API keys or credentials
```

### 3. Set Up Encryption Key

```bash
# Generate secure 32-byte encryption key
export NUIMANBOT_ENCRYPTION_KEY=$(openssl rand -base64 32 | head -c 32)

# Save it securely (password manager, secrets vault, etc.)
echo "NUIMANBOT_ENCRYPTION_KEY=${NUIMANBOT_ENCRYPTION_KEY}" > .env.secret
chmod 600 .env.secret

# Add to shell profile for persistence
echo "export NUIMANBOT_ENCRYPTION_KEY='${NUIMANBOT_ENCRYPTION_KEY}'" >> ~/.bashrc
```

### 4. Review System Requirements

- [ ] Go 1.24+ installed
- [ ] Sufficient disk space (estimate: current DB size + 50%)
- [ ] Encryption key generated and saved
- [ ] Admin access to all systems
- [ ] Scheduled maintenance window
- [ ] Stakeholders notified

### 5. Test Migration in Dev Environment

**Highly Recommended:** Test the entire migration process in a development or staging environment first.

```bash
# Create test environment
mkdir -p ~/nuimanbot-migration-test
cd ~/nuimanbot-migration-test

# Copy production data
cp -r /path/to/production/data ./data-test
cp /path/to/production/config.yaml ./config-test.yaml

# Run migration on test data
# (follow migration steps below)
```

---

## Migration Steps

### Step 1: Stop the Running Service

```bash
# If running as systemd service
sudo systemctl stop nuimanbot

# If running in screen/tmux
# Ctrl+C to stop the process

# Verify no processes running
ps aux | grep nuimanbot
```

### Step 2: Update to New Version

```bash
# Pull latest code
cd /path/to/NuimanBot
git fetch origin
git checkout main
git pull origin main

# Rebuild application
go mod download
go build -o bin/nuimanbot ./cmd/nuimanbot
```

### Step 3: Update Configuration File

**Old config.yaml format:**
```yaml
llm:
  anthropic:
    api_key: "..."

gateways:
  slack:
    bot_token: "xoxb-..."
    app_token: "xapp-..."
```

**New config.yaml format:**
```yaml
server:
  paths:
    config: "./config/"
    data: "./data/"
    logs: "./logs/"

llm:
  anthropic:
    api_key: "..."

gateways:
  cli:
    enabled: true
  slack:
    enabled: true
  telegram:
    enabled: false

# Bot configurations now managed via bots.json
# Remove bot token details from config.yaml
```

**Migration Script for Config:**
```bash
# Backup old config
cp config.yaml config.yaml.old

# Update config structure
# (Manual edit or use sed/awk for automation)
# Add server.paths section
# Add enabled flags to gateways
# Remove bot token details (will be in bots.json)
```

### Step 4: Migrate User Data

**Create users.json from existing SQLite data:**

```bash
# Run migration script (if available)
./bin/nuimanbot admin migrate users --from sqlite --to json

# OR manually create users.json
cat > data/users.json <<'EOF'
{
  "users": [
    {
      "userID": "admin-001",
      "moniker": "admin",
      "firstName": "Admin",
      "lastName": "User",
      "primaryEmail": "admin@example.com",
      "primaryLanguage": "en",
      "timezone": "UTC",
      "role": "admin",
      "userType": "admin",
      "platformIDs": {
        "cli": "admin",
        "slack": "",
        "telegram": ""
      },
      "profileInfo": "",
      "apiKey": "generated-api-key-here",
      "enabled": true,
      "dataDirectory": "./data/users/admin-001",
      "createdAt": "2026-02-08T00:00:00Z",
      "updatedAt": "2026-02-08T00:00:00Z",
      "lastVerified": "2026-02-08T00:00:00Z"
    }
  ],
  "byUsername": {
    "admin": "admin-001"
  },
  "byEmail": {
    "admin@example.com": "admin-001"
  },
  "byPlatform": {
    "cli": {
      "admin": "admin-001"
    },
    "slack": {},
    "telegram": {}
  }
}
EOF

# Create user directories
mkdir -p data/users/admin-001
```

**User Migration Checklist:**
- [ ] All users exported from SQLite
- [ ] users.json created with correct structure
- [ ] API keys generated for all users
- [ ] User directories created
- [ ] Platform IDs mapped (if applicable)
- [ ] Roles preserved

### Step 5: Migrate Bot Configurations

**Create bots.json from config.yaml:**

```bash
# Extract bot configs from old config.yaml
# Encrypt tokens before storing

cat > data/bots.json <<'EOF'
{
  "slackBots": [
    {
      "botID": "slack-001",
      "botName": "Main Slack Bot",
      "botType": "public",
      "slackBotToken": "ENCRYPTED_TOKEN_HERE",
      "slackAppToken": "ENCRYPTED_TOKEN_HERE",
      "slackSigningSecret": "ENCRYPTED_SECRET_HERE",
      "allowedUserIDs": ["user-001", "user-002"],
      "ownerUserID": "",
      "enabled": true,
      "createdAt": "2026-02-08T00:00:00Z",
      "updatedAt": "2026-02-08T00:00:00Z"
    }
  ],
  "telegramBots": [],
  "byName": {
    "Main Slack Bot": "slack-001"
  },
  "byPlatformBotID": {
    "slack": {},
    "telegram": {}
  }
}
EOF

# Note: Actual encryption happens automatically when using the admin CLI/API
```

**Bot Migration Checklist:**
- [ ] All bot tokens extracted from config.yaml
- [ ] bots.json created with correct structure
- [ ] Bot tokens encrypted
- [ ] Bot ownership assigned
- [ ] Allowed users configured
- [ ] Enable/disable status preserved

### Step 6: Verify File Structure

```bash
# Expected directory structure
tree -L 2 data/
# data/
# ├── users.json
# ├── bots.json
# ├── nuimanbot.db
# └── users/
#     └── admin-001/
```

### Step 7: Start Service with New Configuration

```bash
# Set encryption key
export NUIMANBOT_ENCRYPTION_KEY='your-32-byte-key'

# Start application
./bin/nuimanbot help

# Or start as systemd service
sudo systemctl start nuimanbot
```

### Step 8: Verify Migration Success

```bash
# Check user profiles
./bin/nuimanbot admin profile list

# Check bot configurations
./bin/nuimanbot admin bot slack list
./bin/nuimanbot admin bot telegram list

# Test REST API
curl -X GET http://localhost:8080/api/v1/admin/profiles \
  -H "Authorization: Bearer YOUR_API_KEY"

# Check logs
tail -f logs/nuimanbot.log
```

---

## Post-Migration Verification

### Verification Checklist

**User Management:**
- [ ] All users visible in `nuimanbot admin profile list`
- [ ] User count matches pre-migration count
- [ ] Admin users have correct roles
- [ ] API keys work for REST API access
- [ ] User directories exist for all users

**Bot Management:**
- [ ] All bots visible in bot list commands
- [ ] Bots can connect to platforms
- [ ] Messages received and processed correctly
- [ ] Bot enable/disable works
- [ ] Token encryption verified (no plaintext tokens in bots.json)

**REST API:**
- [ ] Authentication works with API keys
- [ ] All CRUD operations functional
- [ ] Role-based access control works
- [ ] Pagination works on list endpoints
- [ ] Error responses appropriate

**System:**
- [ ] Configuration hot reload works
- [ ] Logs written correctly
- [ ] No error messages in logs
- [ ] Performance acceptable
- [ ] All gateways functioning

### Testing Commands

```bash
# Test user operations
./bin/nuimanbot admin profile list
./bin/nuimanbot admin profile view admin-001

# Test bot operations
./bin/nuimanbot admin bot slack list
./bin/nuimanbot admin bot slack view slack-001

# Test REST API
curl -X GET http://localhost:8080/api/v1/admin/status \
  -H "Authorization: Bearer YOUR_API_KEY"

# Test gateway connectivity
# Send test message via Slack
# Send test message via Telegram
# Send message via CLI
```

---

## Rollback Procedures

### When to Rollback

Rollback if you encounter:
- Data corruption or loss
- Service failures or crashes
- Critical features not working
- Performance degradation
- Unable to complete migration

### Rollback Steps

```bash
# 1. Stop new service
sudo systemctl stop nuimanbot
# or kill the process

# 2. Restore old version
cd /path/to/NuimanBot
git checkout [previous-version-tag]
go build -o bin/nuimanbot ./cmd/nuimanbot

# 3. Restore configuration
cp backups/$(date +%Y%m%d)/config.yaml ./config.yaml

# 4. Restore data (if modified)
cp backups/$(date +%Y%m%d)/nuimanbot.db ./data/nuimanbot.db

# 5. Remove new files
rm data/users.json
rm data/bots.json
rm -rf data/users/

# 6. Restart old service
./bin/nuimanbot
# or sudo systemctl start nuimanbot

# 7. Verify service operational
./bin/nuimanbot help
```

### Post-Rollback Actions

- Document issues encountered
- Review logs for errors
- Plan mitigation strategy
- Schedule retry with fixes

---

## Common Migration Issues

### Issue 1: Encryption Key Not Set

**Symptom:**
```
Failed to load configuration: NUIMANBOT_ENCRYPTION_KEY is not set
```

**Solution:**
```bash
export NUIMANBOT_ENCRYPTION_KEY='your-32-byte-key'
# Add to ~/.bashrc for persistence
```

### Issue 2: User Profile Not Found

**Symptom:**
```
User profile not found for user ID: xxx
```

**Solution:**
- Check users.json exists: `ls -la data/users.json`
- Verify JSON format: `cat data/users.json | jq .`
- Ensure user exists in users array
- Check file permissions: `chmod 644 data/users.json`

### Issue 3: Bot Token Decryption Failed

**Symptom:**
```
Failed to decrypt bot token
```

**Solution:**
- Verify encryption key is correct
- Check bots.json is not corrupted
- Re-create bot with fresh tokens
- Ensure tokens were encrypted with same key

### Issue 4: API Authentication Fails

**Symptom:**
```
HTTP 401 Unauthorized
```

**Solution:**
- Verify API key in users.json
- Check Authorization header format: `Bearer <token>`
- Ensure user is enabled: `"enabled": true`
- Verify user has appropriate role

### Issue 5: Configuration Reload Fails

**Symptom:**
```
Failed to reload configuration
```

**Solution:**
- Validate config.yaml syntax: `yq eval config.yaml`
- Check file permissions
- Review logs for specific errors
- Restore from backup if corrupted

### Issue 6: Missing User Directories

**Symptom:**
```
Failed to create user data directory
```

**Solution:**
```bash
# Create missing directories
mkdir -p data/users/[userID]
chown nuimanbot:nuimanbot data/users/[userID]
chmod 755 data/users/[userID]
```

### Issue 7: Database Lock

**Symptom:**
```
database is locked
```

**Solution:**
- Ensure only one instance running
- Kill any zombie processes
- Remove stale lock files
- Check file permissions on SQLite database

---

## Best Practices

### Before Migration

1. **Test in non-production first**
2. **Schedule during maintenance window**
3. **Notify all stakeholders**
4. **Document custom configurations**
5. **Have rollback plan ready**

### During Migration

1. **Follow steps exactly**
2. **Verify each step before proceeding**
3. **Document any deviations**
4. **Keep backups accessible**
5. **Monitor logs continuously**

### After Migration

1. **Run comprehensive tests**
2. **Monitor for 24-48 hours**
3. **Keep backups for 30 days**
4. **Document lessons learned**
5. **Update documentation**

---

## Support and Resources

### Documentation

- [Admin Guide](admin-guide.md) - Administrative procedures
- [API Reference](api-reference.md) - REST API documentation
- [Configuration Reference](configuration-reference.md) - Config file reference

### Getting Help

- **Issues:** https://github.com/stainedhead/NuimanBot/issues
- **Logs:** Check `logs/nuimanbot.log` for detailed errors
- **Community:** [Project discussions]

---

## Migration Timeline

**Estimated Duration:** 2-4 hours for typical installation

| Phase | Duration | Description |
|-------|----------|-------------|
| Pre-Migration | 30 min | Backups, checklist, planning |
| Configuration Update | 30 min | Update config.yaml |
| User Migration | 30-60 min | Create users.json |
| Bot Migration | 30-60 min | Create bots.json |
| Testing | 30-60 min | Verification and smoke tests |
| Rollback Buffer | N/A | Time reserved for issues |

---

**Document Version:** 1.0
**Last Updated:** 2026-02-08
**Maintainer:** NuimanBot Development Team
