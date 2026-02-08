# NuimanBot Administration Guide

**Version:** 1.0
**Last Updated:** 2026-02-08
**Target Audience:** System Administrators, DevOps Engineers

---

## Table of Contents

1. [Introduction](#introduction)
2. [Installation and Setup](#installation-and-setup)
3. [REST API Administration](#rest-api-administration)
4. [CLI Administration](#cli-administration)
5. [User Management](#user-management)
6. [Bot Management](#bot-management)
7. [Configuration Management](#configuration-management)
8. [Monitoring and Observability](#monitoring-and-observability)
9. [Security and Access Control](#security-and-access-control)
10. [Common Administrative Tasks](#common-administrative-tasks)
11. [Troubleshooting](#troubleshooting)

---

## Introduction

NuimanBot provides comprehensive administrative capabilities through:
- **REST API**: Programmatic administration with full CRUD operations
- **CLI Commands**: Command-line interface for local administration
- **Configuration Files**: YAML-based configuration with hot reload support

This guide covers all administrative functions for managing users, bots, and system configuration.

---

## Installation and Setup

### Prerequisites

- Go 1.24 or later
- SQLite3
- At least one LLM provider (Anthropic, OpenAI, Bedrock, or Ollama)
- Encryption key for secure credential storage

### Initial Setup

1. **Clone and Build:**
   ```bash
   git clone https://github.com/stainedhead/NuimanBot.git
   cd NuimanBot
   go mod download
   go build -o bin/nuimanbot ./cmd/nuimanbot
   ```

2. **Set Encryption Key:**
   ```bash
   # Generate a secure 32-byte key
   export NUIMANBOT_ENCRYPTION_KEY=$(openssl rand -base64 32 | head -c 32)

   # Add to your shell profile for persistence
   echo "export NUIMANBOT_ENCRYPTION_KEY='your-key-here'" >> ~/.bashrc
   ```

3. **Create Configuration:**
   Create `config.yaml` in the project root (see Configuration Reference for details).

4. **Initialize Database:**
   ```bash
   # The database will be created automatically on first run
   ./bin/nuimanbot help
   ```

5. **Create Admin User:**
   ```bash
   # Using CLI
   ./bin/nuimanbot admin profile create \
     --user-id admin-001 \
     --email admin@example.com \
     --role admin \
     --first-name Admin \
     --last-name User
   ```

---

## REST API Administration

### Authentication

All API requests require Bearer token authentication:

```bash
curl -X GET http://localhost:8080/api/v1/admin/profiles \
  -H "Authorization: Bearer your-api-key-here"
```

API keys are auto-generated when creating user profiles and stored in the `apiKey` field.

### Base URL

- **Development:** `http://localhost:8080`
- **Production:** Configure via `NUIMANBOT_SERVER_PORT` environment variable

### Common API Operations

**List Users:**
```bash
curl -X GET "http://localhost:8080/api/v1/admin/profiles?offset=0&limit=50" \
  -H "Authorization: Bearer ${API_KEY}"
```

**Create User:**
```bash
curl -X POST http://localhost:8080/api/v1/admin/profiles \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "userID": "user-123",
    "primaryEmail": "user@example.com",
    "firstName": "John",
    "lastName": "Doe",
    "role": "user",
    "userType": "individual"
  }'
```

**Update User (Partial):**
```bash
curl -X PUT http://localhost:8080/api/v1/admin/profiles/user-123 \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "firstName": "Jane",
    "timezone": "America/New_York"
  }'
```

**Delete User:**
```bash
curl -X DELETE http://localhost:8080/api/v1/admin/profiles/user-123 \
  -H "Authorization: Bearer ${API_KEY}"
```

See [API Reference](api-reference.md) for complete endpoint documentation.

---

## CLI Administration

### User Profile Management

**List Profiles:**
```bash
./bin/nuimanbot admin profile list
./bin/nuimanbot admin profile list --format json
./bin/nuimanbot admin profile list --format csv
```

**View Profile:**
```bash
./bin/nuimanbot admin profile view user-123
```

**Create Profile:**
```bash
./bin/nuimanbot admin profile create \
  --user-id user-123 \
  --email user@example.com \
  --first-name John \
  --last-name Doe \
  --role user \
  --timezone "America/New_York"
```

**Update Profile:**
```bash
./bin/nuimanbot admin profile update user-123 \
  --first-name Jane \
  --timezone "America/Los_Angeles"
```

**Delete Profile:**
```bash
./bin/nuimanbot admin profile delete user-123
```

**Import/Export:**
```bash
# Export to JSON
./bin/nuimanbot admin profile export --file users-backup.json

# Export to CSV
./bin/nuimanbot admin profile export --file users-backup.csv --format csv

# Import from JSON
./bin/nuimanbot admin profile import --file users-backup.json
```

### Bot Management

**Slack Bot Management:**
```bash
# List Slack bots
./bin/nuimanbot admin bot slack list

# View Slack bot
./bin/nuimanbot admin bot slack view bot-123

# Create Slack bot
./bin/nuimanbot admin bot slack create \
  --bot-id bot-123 \
  --bot-name "Customer Support Bot" \
  --bot-type public \
  --slack-bot-token "xoxb-your-token" \
  --slack-app-token "xapp-your-token"

# Update Slack bot
./bin/nuimanbot admin bot slack update bot-123 \
  --bot-name "Updated Bot Name"

# Enable/Disable bot
./bin/nuimanbot admin bot slack enable bot-123
./bin/nuimanbot admin bot slack disable bot-123

# Delete bot
./bin/nuimanbot admin bot slack delete bot-123
```

**Telegram Bot Management:**
```bash
# List Telegram bots
./bin/nuimanbot admin bot telegram list

# Create Telegram bot
./bin/nuimanbot admin bot telegram create \
  --bot-id tg-bot-456 \
  --bot-name "Support Bot" \
  --bot-type private \
  --owner-user-id user-123 \
  --telegram-bot-token "123456:ABC-DEF..."

# Enable/Disable bot
./bin/nuimanbot admin bot telegram enable tg-bot-456
./bin/nuimanbot admin bot telegram disable tg-bot-456
```

---

## User Management

### User Roles

NuimanBot supports three user roles:

1. **Guest** (role: `guest`)
   - Limited access
   - Unauthenticated users
   - Cannot modify system state

2. **User** (role: `user`)
   - Standard access
   - Can use bot features
   - Can manage own profile
   - Can view own bots

3. **Admin** (role: `admin`)
   - Full system access
   - Can manage all users
   - Can manage all bots
   - Can modify configuration
   - Can access system metrics

### User Types

- **Individual**: Personal user account
- **Enterprise**: Organization user account
- **Developer**: Developer account with additional API access
- **Admin**: System administrator account

### Platform Integration

Link user profiles to messaging platforms:

**REST API:**
```bash
# Link Slack ID
curl -X PUT http://localhost:8080/api/v1/admin/profiles/user-123/integrations/slack \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"slackID": "U01ABC123"}'

# Link Telegram ID
curl -X PUT http://localhost:8080/api/v1/admin/profiles/user-123/integrations/telegram \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"telegramID": "123456789"}'
```

### Agent Preferences

Users can customize agent behavior through preferences (stored in profile):
- Preferred LLM model
- Response temperature
- Max tokens
- Conversation context window

---

## Bot Management

### Bot Types

**Public Bots:**
- Shared among multiple users
- Requires `allowedUserIDs` list
- Central administration

**Private Bots:**
- Single-user ownership
- Requires `ownerUserID`
- User self-service management

### Security

- Bot tokens are encrypted at rest using AES-256-GCM
- Tokens are never returned in API responses (masked as `***`)
- Encryption key must be set via `NUIMANBOT_ENCRYPTION_KEY`

### Bot Status Monitoring

Check bot health and connectivity:

```bash
# Via REST API
curl -X GET http://localhost:8080/api/v1/admin/status \
  -H "Authorization: Bearer ${API_KEY}"
```

---

## Configuration Management

### Configuration Files

- **Primary:** `config.yaml` in project root
- **Override:** Environment variables (prefix: `NUIMANBOT_`)

### Hot Reload

Reload configuration without restarting:

**REST API:**
```bash
curl -X POST http://localhost:8080/api/v1/admin/config/reload \
  -H "Authorization: Bearer ${API_KEY}"
```

**STDIO (if running server daemon):**
```
/refresh
```

### Validation

Validate configuration before applying:

```bash
curl -X POST http://localhost:8080/api/v1/admin/config/validate \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d @new-config.json
```

### Paths Configuration

Configure separate paths for config, data, and logs:

```yaml
server:
  paths:
    config: "./config/"
    data: "./data/"
    logs: "./logs/"
```

Override with environment variables:
```bash
export NUIMANBOT_SERVER_PATHS_CONFIG="/etc/nuimanbot/config"
export NUIMANBOT_SERVER_PATHS_DATA="/var/lib/nuimanbot/data"
export NUIMANBOT_SERVER_PATHS_LOGS="/var/log/nuimanbot"
```

---

## Monitoring and Observability

### Server Status

Get current server status:

```bash
curl -X GET http://localhost:8080/api/v1/admin/status \
  -H "Authorization: Bearer ${API_KEY}"
```

**Response includes:**
- Server uptime
- Version information
- Memory usage
- Active connections (Slack, Telegram, CLI)

### Metrics

Access server metrics:

```bash
curl -X GET http://localhost:8080/api/v1/admin/metrics \
  -H "Authorization: Bearer ${API_KEY}"
```

**Metrics include:**
- Requests in last 24 hours
- Error rate
- Average response time
- Active users count
- Active bots count

### Logs

Retrieve recent logs:

```bash
# All logs (last 50)
curl -X GET "http://localhost:8080/api/v1/admin/logs?limit=50" \
  -H "Authorization: Bearer ${API_KEY}"

# Filter by level
curl -X GET "http://localhost:8080/api/v1/admin/logs?level=error&limit=100" \
  -H "Authorization: Bearer ${API_KEY}"
```

### Prometheus Integration

NuimanBot exposes Prometheus metrics at `/metrics` endpoint:

```bash
curl http://localhost:8080/metrics
```

---

## Security and Access Control

### API Key Management

**Generate New API Key:**
API keys are automatically generated when creating user profiles. To regenerate:

1. Update user profile (this will NOT regenerate the key)
2. For key rotation, use the security service endpoints (if implemented)

**Best Practices:**
- Rotate API keys periodically
- Store keys securely (environment variables, secrets manager)
- Never commit keys to version control
- Use different keys for different environments

### Role-Based Access Control (RBAC)

**Endpoint Authorization:**
- `/api/v1/admin/*` - Requires `admin` role
- `/api/v1/profile` - User can access own profile
- `/api/v1/bots/*` - User can access own bots

**Middleware Stack:**
```
Request → Authentication → RBAC → Handler
```

### Audit Logging

All administrative actions are logged with:
- Timestamp
- Admin user ID
- Action performed
- Resource affected
- Changes made

View audit logs through the logging endpoint.

---

## Common Administrative Tasks

### Task 1: Add a New User

```bash
# 1. Create user profile
./bin/nuimanbot admin profile create \
  --user-id user-001 \
  --email user@company.com \
  --first-name John \
  --last-name Doe \
  --role user \
  --user-type individual

# 2. Note the generated API key from output

# 3. Link platform IDs (if needed)
curl -X PUT http://localhost:8080/api/v1/admin/profiles/user-001/integrations/slack \
  -H "Authorization: Bearer ${ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"slackID": "U01ABC123"}'

# 4. Provide API key to user
```

### Task 2: Add a Slack Bot

```bash
# 1. Obtain Slack tokens from https://api.slack.com/apps
# 2. Create bot configuration
./bin/nuimanbot admin bot slack create \
  --bot-id slack-001 \
  --bot-name "Support Bot" \
  --bot-type public \
  --slack-bot-token "xoxb-your-bot-token" \
  --slack-app-token "xapp-your-app-token" \
  --slack-signing-secret "your-signing-secret"

# 3. Enable the bot
./bin/nuimanbot admin bot slack enable slack-001

# 4. Verify bot is active
./bin/nuimanbot admin bot slack view slack-001
```

### Task 3: Bulk User Import

```bash
# 1. Prepare JSON file (users.json)
cat > users.json <<EOF
[
  {
    "userID": "user-001",
    "primaryEmail": "user1@example.com",
    "firstName": "Alice",
    "role": "user"
  },
  {
    "userID": "user-002",
    "primaryEmail": "user2@example.com",
    "firstName": "Bob",
    "role": "user"
  }
]
EOF

# 2. Import users
curl -X POST http://localhost:8080/api/v1/admin/profiles/import \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d @users.json

# 3. Verify import
./bin/nuimanbot admin profile list
```

### Task 4: Update Configuration

```bash
# 1. Edit config.yaml
vim config.yaml

# 2. Validate configuration
curl -X POST http://localhost:8080/api/v1/admin/config/validate \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d @config.json

# 3. Reload configuration
curl -X POST http://localhost:8080/api/v1/admin/config/reload \
  -H "Authorization: Bearer ${API_KEY}"

# 4. Verify changes
curl -X GET http://localhost:8080/api/v1/admin/config \
  -H "Authorization: Bearer ${API_KEY}"
```

### Task 5: Backup and Restore

**Backup:**
```bash
# 1. Export user profiles
./bin/nuimanbot admin profile export --file users-backup.json

# 2. Backup bot configurations (stored in data/bots.json)
cp data/bots.json backups/bots-$(date +%Y%m%d).json

# 3. Backup database
cp data/nuimanbot.db backups/nuimanbot-$(date +%Y%m%d).db

# 4. Backup configuration
cp config.yaml backups/config-$(date +%Y%m%d).yaml
```

**Restore:**
```bash
# 1. Restore database
cp backups/nuimanbot-20260208.db data/nuimanbot.db

# 2. Restore configuration
cp backups/config-20260208.yaml config.yaml

# 3. Import user profiles
./bin/nuimanbot admin profile import --file backups/users-backup.json

# 4. Restore bot configurations
cp backups/bots-20260208.json data/bots.json

# 5. Restart service
```

---

## Troubleshooting

### Common Issues

#### Issue: "Encryption key not set"

**Symptom:**
```
Failed to load configuration: NUIMANBOT_ENCRYPTION_KEY is not set in environment
```

**Solution:**
```bash
export NUIMANBOT_ENCRYPTION_KEY=$(openssl rand -base64 32 | head -c 32)
# Add to ~/.bashrc for persistence
```

#### Issue: "Unauthorized" API responses

**Symptom:**
```
HTTP 401 Unauthorized
```

**Solution:**
- Verify API key is correct
- Check Authorization header format: `Bearer <token>`
- Ensure user account is enabled
- Check user has appropriate role

#### Issue: Bot not connecting

**Symptom:**
Bot appears in list but doesn't respond to messages

**Solution:**
```bash
# 1. Check bot is enabled
./bin/nuimanbot admin bot slack view bot-id

# 2. Verify tokens are correct
# Re-create bot with correct tokens if needed

# 3. Check bot status
curl -X GET http://localhost:8080/api/v1/admin/status \
  -H "Authorization: Bearer ${API_KEY}"

# 4. Review logs
curl -X GET "http://localhost:8080/api/v1/admin/logs?level=error" \
  -H "Authorization: Bearer ${API_KEY}"
```

#### Issue: Configuration reload fails

**Symptom:**
```
Failed to reload configuration: invalid config
```

**Solution:**
```bash
# 1. Validate configuration first
curl -X POST http://localhost:8080/api/v1/admin/config/validate \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d @config.json

# 2. Check validation errors
# Fix issues in config.yaml

# 3. Retry reload
curl -X POST http://localhost:8080/api/v1/admin/config/reload \
  -H "Authorization: Bearer ${API_KEY}"
```

#### Issue: Database locked errors

**Symptom:**
```
database is locked
```

**Solution:**
- Ensure only one instance of NuimanBot is running
- Check for zombie processes: `ps aux | grep nuimanbot`
- Remove stale lock files if present
- Restart the application

### Debugging Tips

**Enable Debug Logging:**
```yaml
# config.yaml
server:
  log_level: debug
  debug: true
```

**Check File Permissions:**
```bash
# Data directory should be writable
ls -la data/
chmod 755 data/
```

**Verify Database Integrity:**
```bash
sqlite3 data/nuimanbot.db "PRAGMA integrity_check;"
```

**Test API Connectivity:**
```bash
# Health check
curl http://localhost:8080/health

# Status check with auth
curl -X GET http://localhost:8080/api/v1/admin/status \
  -H "Authorization: Bearer ${API_KEY}"
```

### Getting Help

- **Documentation**: See [API Reference](api-reference.md) and [Configuration Reference](configuration-reference.md)
- **Issues**: Report bugs at https://github.com/stainedhead/NuimanBot/issues
- **Logs**: Check application logs for detailed error messages

---

## Appendix

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `NUIMANBOT_ENCRYPTION_KEY` | 32-byte encryption key (required) | None |
| `NUIMANBOT_SERVER_PORT` | HTTP server port | 8080 |
| `NUIMANBOT_SERVER_LOG_LEVEL` | Log level (debug, info, warn, error) | info |
| `NUIMANBOT_SERVER_PATHS_CONFIG` | Config directory path | ./config/ |
| `NUIMANBOT_SERVER_PATHS_DATA` | Data directory path | ./data/ |
| `NUIMANBOT_SERVER_PATHS_LOGS` | Logs directory path | ./logs/ |

### File Locations

| File | Location | Purpose |
|------|----------|---------|
| `config.yaml` | Project root | Main configuration |
| `users.json` | data/users.json | User profile registry |
| `bots.json` | data/bots.json | Bot configurations (encrypted) |
| `nuimanbot.db` | data/nuimanbot.db | SQLite database |
| `audit.log` | logs/audit.log | Audit trail |

### Quick Reference

**User Management:**
```bash
# List
./bin/nuimanbot admin profile list

# Create
./bin/nuimanbot admin profile create --user-id <id> --email <email> --role <role>

# Update
./bin/nuimanbot admin profile update <id> --first-name <name>

# Delete
./bin/nuimanbot admin profile delete <id>
```

**Bot Management:**
```bash
# List
./bin/nuimanbot admin bot slack list

# Create
./bin/nuimanbot admin bot slack create --bot-id <id> --bot-name <name> --bot-type <type>

# Enable/Disable
./bin/nuimanbot admin bot slack enable <id>
./bin/nuimanbot admin bot slack disable <id>
```

**API Quick Commands:**
```bash
# List users
curl -X GET http://localhost:8080/api/v1/admin/profiles \
  -H "Authorization: Bearer ${API_KEY}"

# Server status
curl -X GET http://localhost:8080/api/v1/admin/status \
  -H "Authorization: Bearer ${API_KEY}"

# Reload config
curl -X POST http://localhost:8080/api/v1/admin/config/reload \
  -H "Authorization: Bearer ${API_KEY}"
```

---

**Document Version:** 1.0
**Last Updated:** 2026-02-08
**Maintainer:** NuimanBot Development Team
