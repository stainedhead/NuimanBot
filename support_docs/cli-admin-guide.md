# CLI Administration Guide

## Overview

NuimanBot provides a set of administrative commands accessible through the CLI gateway. These commands allow administrators to manage users, assign roles, configure tool permissions, and maintain the system.

**Requirements:**
- CLI gateway must be running
- User must have `admin` role to execute admin commands
- Commands are prefixed with `/admin`

---

## Quick Reference

| Command | Description |
|---------|-------------|
| `/admin help` | Show all available admin commands |
| `/admin user create` | Create a new user |
| `/admin user list` | List all users |
| `/admin user get` | Get detailed user information |
| `/admin user update` | Update user role or tool permissions |
| `/admin user delete` | Delete a user |

---

## User Management

### Creating Users

Create a new user with a specific role:

```bash
/admin user create <platform> <platform_uid> <role>
```

**Parameters:**
- `platform`: Platform identifier (`cli`, `telegram`, `slack`)
- `platform_uid`: Platform-specific user ID (e.g., username for CLI, Telegram ID)
- `role`: User role (`guest`, `user`, `admin`)

**Examples:**

```bash
# Create a CLI user with user role
> /admin user create cli alice user

# Create a Telegram user with admin role
> /admin user create telegram 123456789 admin

# Create a Slack user with guest role
> /admin user create slack U01234567 guest
```

**Response:**
```
User created successfully
ID: abc123def456
Platform: cli
Platform UID: alice
Role: user
```

---

### Listing Users

List all users in the system:

```bash
/admin user list
```

**Example:**

```bash
> /admin user list
```

**Response:**
```
Users (3):
1. ID: abc123def456
   Platform: cli | UID: alice | Role: user
   Allowed Skills: calculator, datetime, weather

2. ID: def456abc789
   Platform: telegram | UID: 123456789 | Role: admin
   Allowed Skills: (all)

3. ID: ghi789jkl012
   Platform: slack | UID: U01234567 | Role: guest
   Allowed Skills: (none)
```

---

### Getting User Details

Get detailed information about a specific user:

```bash
/admin user get <user_id>
```

**Parameters:**
- `user_id`: The unique user ID (shown in user list)

**Example:**

```bash
> /admin user get abc123def456
```

**Response:**
```
User Details:
ID: abc123def456
Platform: cli
Platform UID: alice
Role: user
Allowed Skills: calculator, datetime, weather
Created: 2026-02-07 10:30:00
Last Updated: 2026-02-07 10:30:00
```

---

### Updating Users

Update a user's role or tool permissions.

#### Update User Role

```bash
/admin user update <user_id> --role <role>
```

**Parameters:**
- `user_id`: The unique user ID
- `role`: New role (`guest`, `user`, `admin`)

**Examples:**

```bash
# Promote user to admin
> /admin user update abc123def456 --role admin

# Demote admin to user
> /admin user update def456abc789 --role user
```

**Response:**
```
User updated successfully
New role: admin
```

**Important Notes:**
- Cannot demote the last admin user (system requires at least one admin)
- Role changes take effect immediately

#### Update User Tool Permissions

```bash
/admin user update <user_id> --skills <skill1,skill2,...>
```

**Parameters:**
- `user_id`: The unique user ID
- `skills`: Comma-separated list of allowed tool names (no spaces)

**Available Tools:**
- Core tools: `calculator`, `datetime`, `weather`, `websearch`, `notes`
- Developer tools: `github`, `repo_search`, `doc_summarize`, `summarize`, `coding_agent`

**Examples:**

```bash
# Grant basic tools only
> /admin user update abc123def456 --skills calculator,datetime

# Grant web and search tools
> /admin user update abc123def456 --skills websearch,repo_search,github

# Grant all core tools
> /admin user update abc123def456 --skills calculator,datetime,weather,websearch,notes
```

**Response:**
```
User updated successfully
Allowed Skills: calculator, datetime
```

**Important Notes:**
- Tool names are case-sensitive
- Invalid tool names are ignored (not an error)
- Admin users have access to all tools by default
- Empty skills list (`--skills ""`) removes all tool permissions

---

### Deleting Users

Delete a user from the system:

```bash
/admin user delete <user_id>
```

**Parameters:**
- `user_id`: The unique user ID

**Example:**

```bash
> /admin user delete abc123def456
```

**Response:**
```
User deleted successfully
```

**Important Notes:**
- Cannot delete the last admin user (system requires at least one admin)
- Deletion is permanent and cannot be undone
- All user data (conversations, notes) is deleted

---

## User Roles

NuimanBot has three built-in roles with different permission levels:

### Guest Role
**Permissions:** None
- Cannot use any tools
- Can only chat with the LLM (no tool calling)
- Useful for read-only or evaluation access

**Example Use Case:**
```bash
# Create a guest user for demonstration
> /admin user create cli demo guest
```

### User Role
**Permissions:** Configurable tool access
- Access to tools specified in `allowed_skills` list
- Default role for most users
- Supports custom tool allowlists per user

**Example Use Case:**
```bash
# Create user with calculator and datetime access
> /admin user create cli alice user
> /admin user update <alice_id> --skills calculator,datetime
```

### Admin Role
**Permissions:** Full system access
- Access to all tools (ignores `allowed_skills` list)
- Can execute admin commands
- Can manage other users
- Can execute shell tools (e.g., `coding_agent`)

**Example Use Case:**
```bash
# Create an admin user
> /admin user create cli bob admin
```

---

## Tool Permissions

### Permission Model

Tool permissions follow a hierarchical model:

```
Admin Role
  └─> All tools available (unrestricted)

User Role
  └─> Only tools in allowed_skills list
      ├─> calculator (if in list)
      ├─> datetime (if in list)
      └─> ...

Guest Role
  └─> No tools available
```

### Tool Categories

**Core Tools (Infrastructure Layer):**
- `calculator` - Basic arithmetic operations
- `datetime` - Current time, formatting, timezones
- `weather` - Current weather and forecasts (requires API key)
- `websearch` - DuckDuckGo web search
- `notes` - CRUD operations for personal notes

**Developer Productivity Tools (Use Case Layer):**
- `github` - GitHub operations via `gh` CLI (issues, PRs, workflows)
- `repo_search` - Fast codebase search using ripgrep
- `doc_summarize` - LLM-powered document summarization
- `summarize` - Web page and YouTube video summarization
- `coding_agent` - Orchestrate external coding CLIs (admin-only)

### Example Permission Configurations

**Data Analyst:**
```bash
/admin user update <user_id> --skills calculator,websearch,notes
```

**Developer:**
```bash
/admin user update <user_id> --skills calculator,datetime,github,repo_search,doc_summarize
```

**Content Writer:**
```bash
/admin user update <user_id> --skills websearch,summarize,notes
```

**Full Access (Non-Admin):**
```bash
/admin user update <user_id> --skills calculator,datetime,weather,websearch,notes,github,repo_search,doc_summarize,summarize
```

---

## Best Practices

### User Management

1. **Principle of Least Privilege**
   - Grant users only the tools they need
   - Start with minimal permissions and add as needed
   - Review user permissions regularly

2. **Admin Account Management**
   - Maintain at least 2 admin accounts (redundancy)
   - Use descriptive platform UIDs (e.g., `admin_alice`, not `user123`)
   - Audit admin actions regularly

3. **Platform Mapping**
   - Use consistent naming across platforms
   - Document platform UID to user mappings
   - Consider creating a user directory (external to system)

### Security Considerations

1. **Role Assignments**
   - New users should default to `user` role
   - Grant `admin` role only when necessary
   - Never create public-facing guest accounts with escalation paths

2. **Tool Permissions**
   - Restrict `coding_agent` to admins only (shell access)
   - Be cautious with `github` tool (can create PRs, modify repos)
   - `websearch` and `summarize` tools access external data

3. **Audit Trail**
   - All admin commands are logged
   - Review logs for unauthorized access attempts
   - Monitor tool execution patterns for anomalies

---

## Common Workflows

### Onboarding a New Team Member

```bash
# 1. Create user account
> /admin user create cli alice user

# 2. Assign appropriate tools
> /admin user update <alice_id> --skills calculator,datetime,github,repo_search

# 3. Verify setup
> /admin user get <alice_id>

# 4. Map Telegram ID (if using Telegram gateway)
> /admin user create telegram 987654321 user
> /admin user update <telegram_user_id> --skills calculator,datetime,github,repo_search
```

### Rotating Admin Access

```bash
# 1. Ensure at least 2 admins exist
> /admin user list

# 2. Promote new admin
> /admin user create cli bob admin

# 3. Verify promotion
> /admin user get <bob_id>

# 4. Demote old admin (optional)
> /admin user update <old_admin_id> --role user
```

### Restricting User Access (Downgrade)

```bash
# 1. Check current permissions
> /admin user get <user_id>

# 2. Remove sensitive tools
> /admin user update <user_id> --skills calculator,datetime

# 3. Or downgrade to guest (no tools)
> /admin user update <user_id> --role guest
```

### Removing a User

```bash
# 1. Verify user details
> /admin user get <user_id>

# 2. Ensure not last admin (if admin role)
> /admin user list

# 3. Delete user
> /admin user delete <user_id>
```

---

## Troubleshooting

### "Permission denied" when running admin commands

**Problem:** User lacks admin role

**Solution:**
```bash
# Verify current user role
> /admin user list

# If you're locked out, you'll need database access to fix this
# Option 1: Direct database update (requires SQLite access)
# Option 2: Restart with default admin user
```

### "Cannot delete last admin user"

**Problem:** System requires at least one admin

**Solution:**
```bash
# Create a new admin first
> /admin user create cli new_admin admin

# Then delete the old admin
> /admin user delete <old_admin_id>
```

### "Cannot demote last admin"

**Problem:** System requires at least one admin

**Solution:**
```bash
# Promote another user to admin first
> /admin user update <user_id> --role admin

# Then demote the admin
> /admin user update <admin_id> --role user
```

### User created but can't access Telegram/Slack

**Problem:** Platform UID mapping incorrect

**Solution:**
```bash
# Verify platform UID is correct
> /admin user get <user_id>

# For Telegram: Use numeric Telegram ID (not username)
# For Slack: Use Slack User ID (starts with U, e.g., U01234567)

# Delete incorrect user and recreate
> /admin user delete <user_id>
> /admin user create telegram 123456789 user
```

### Tool permissions not taking effect

**Problem:** User might be admin (admins bypass tool restrictions)

**Solution:**
```bash
# Check user role
> /admin user get <user_id>

# If admin, tool restrictions are ignored by design
# Demote to user role for tool restrictions to apply
> /admin user update <user_id> --role user
```

---

## Integration with Other Features

### Agent Skills

Users can only invoke skills that use tools they have permission for:

```bash
# User has: calculator, datetime
# Can invoke: /code-review (if uses allowed tools only)
# Cannot invoke: /github-pr-review (requires github tool)
```

**Admin Configuration:**
```bash
# Grant tools needed for specific skills
> /admin user update <user_id> --skills github,repo_search  # For code-review skill
```

### Multi-Platform Users

Map the same logical user across multiple platforms:

```bash
# Create CLI user
> /admin user create cli alice user
> /admin user update <cli_id> --skills calculator,datetime

# Create Telegram user (same person)
> /admin user create telegram 123456789 user
> /admin user update <telegram_id> --skills calculator,datetime

# Create Slack user (same person)
> /admin user create slack U01234567 user
> /admin user update <slack_id> --skills calculator,datetime
```

**Note:** Currently, conversation history is platform-specific. Cross-platform conversation merging is a future enhancement.

---

## Getting Help

### Within the CLI

```bash
# Show all admin commands
> /admin help

# Show general help (includes skills)
> /help
```

### Documentation

- **[Agent Skills User Guide](skills-guide.md)** - Creating and using skills
- **[Installation & Setup Guide](install-and-setup.md)** - System installation and configuration
- **[User Onboarding Guide](user-onboarding.md)** - How to use NuimanBot
- **[Product Details](../documentation/product-details.md)** - Full system documentation

### Support

- **Issues**: https://github.com/stainedhead/NuimanBot/issues
- **Configuration**: Check `config.yaml` for gateway settings

---

## Reference

### Command Syntax Summary

```bash
# User Management
/admin user create <platform> <platform_uid> <role>
/admin user list
/admin user get <user_id>
/admin user update <user_id> --role <role>
/admin user update <user_id> --skills <skill1,skill2,...>
/admin user delete <user_id>

# Help
/admin help
```

### Valid Roles

- `guest` - No tool access, LLM chat only
- `user` - Configurable tool access via allowed_skills
- `admin` - Full access, all tools, admin commands

### Valid Platforms

- `cli` - Command-line interface
- `telegram` - Telegram messenger
- `slack` - Slack workspace

### Available Tools

**Core (5):** calculator, datetime, weather, websearch, notes
**Developer (7):** github, repo_search, doc_summarize, summarize, coding_agent, executor, common

---

**Last Updated:** 2026-02-07
**Version:** 1.0
