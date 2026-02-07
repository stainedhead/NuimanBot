# NuimanBot Onboarding Guide

Complete installation and configuration guide for getting NuimanBot up and running.

## Table of Contents

- [System Requirements](#system-requirements)
- [Installation](#installation)
- [Initial Configuration](#initial-configuration)
- [Running the Application](#running-the-application)
- [Verification](#verification)
- [Production Deployment](#production-deployment)
- [Troubleshooting](#troubleshooting)

---

## System Requirements

### Minimum Requirements

- **Operating System**: Linux, macOS, or Windows
- **Go**: Version 1.21 or later
- **SQLite**: Version 3 (usually bundled with OS)
- **Memory**: 512 MB RAM minimum (2 GB recommended for production)
- **Disk Space**: 100 MB for application + storage for database

### Required External Services

Choose **at least one** LLM provider:

| Provider | Cost | Best For | API Key Required |
|----------|------|----------|------------------|
| **Anthropic Claude** | Paid (recommended) | Production, advanced reasoning | ✅ Yes |
| **OpenAI GPT** | Paid | Compatibility, wide model selection | ✅ Yes |
| **Ollama** | Free | Local/offline, privacy, development | ❌ No |

### Optional Services

- **OpenWeatherMap API** - For weather skill (free tier available)
- **Telegram Bot** - For Telegram gateway integration
- **Slack App** - For Slack workspace integration

---

## Installation

### Step 1: Install Go

**macOS (Homebrew):**
```bash
brew install go
go version  # Should show 1.21+
```

**Linux (Ubuntu/Debian):**
```bash
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
go version
```

**Windows:**
1. Download installer from https://go.dev/dl/
2. Run installer
3. Verify in PowerShell: `go version`

### Step 2: Clone and Build

```bash
# Clone the repository
git clone https://github.com/stainedhead/NuimanBot.git
cd NuimanBot

# Download dependencies
go mod download

# Build the application
go build -o bin/nuimanbot ./cmd/nuimanbot

# Verify build
ls -lh bin/nuimanbot  # Should show ~30MB executable
```

---

## Initial Configuration

### Step 1: Generate Encryption Key

NuimanBot requires a 32-byte encryption key to secure stored credentials:

```bash
# Generate a random 32-byte key (macOS/Linux)
openssl rand -hex 16

# OR use this command
head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n'

# Windows (PowerShell)
-join ((48..57) + (97..102) | Get-Random -Count 32 | ForEach-Object {[char]$_})
```

Save this key securely - **you'll need it every time you run NuimanBot**.

### Step 2: Set Up Environment Variables

Create a `.env` file (not committed to git) or export directly:

```bash
# Required: Encryption key
export NUIMANBOT_ENCRYPTION_KEY="your-32-byte-key-from-step-1"

# Choose your LLM provider (pick one to start)

# Option A: Anthropic Claude (recommended)
export NUIMANBOT_LLM_ANTHROPIC_APIKEY="sk-ant-api03-your-key-here"

# Option B: OpenAI GPT
export NUIMANBOT_LLM_OPENAI_APIKEY="sk-your-openai-key-here"
export NUIMANBOT_LLM_OPENAI_BASEURL="https://api.openai.com/v1"

# Option C: Ollama (local, no API key needed)
export NUIMANBOT_LLM_OLLAMA_BASEURL="http://localhost:11434"

# Optional: Weather skill
export OPENWEATHERMAP_API_KEY="your-weather-api-key"

# Optional: Configuration overrides
export NUIMANBOT_SERVER_LOGLEVEL="info"  # debug, info, warn, error
export NUIMANBOT_SECURITY_INPUTMAXLENGTH="4096"
```

### Step 3: Create Configuration File (Optional)

You can use a `config.yaml` file instead of environment variables:

```yaml
# config.yaml
server:
  log_level: info
  debug: false
  environment: development  # development, staging, production

security:
  input_max_length: 4096
  vault_path: "./data/vault.enc"

storage:
  type: sqlite
  dsn: "./data/nuimanbot.db"

llm:
  anthropic:
    api_key: "sk-ant-your-key-here"
  # OR openai:
  #   api_key: "sk-your-openai-key"
  #   base_url: "https://api.openai.com/v1"
  # OR ollama:
  #   base_url: "http://localhost:11434"

gateways:
  cli:
    debug_mode: false

skills:
  entries:
    calculator:
      enabled: true
    datetime:
      enabled: true
    weather:
      enabled: true
    websearch:
      enabled: true
    notes:
      enabled: true
```

**Configuration Precedence** (highest to lowest):
1. Environment variables (e.g., `NUIMANBOT_LLM_ANTHROPIC_APIKEY`)
2. `config.yaml` file in current directory
3. Default values

### Step 4: Initialize Data Directory

```bash
# Create data directory for database and vault
mkdir -p data

# The application will automatically create:
# - data/nuimanbot.db (SQLite database)
# - data/vault.enc (encrypted credential vault)
```

---

## Running the Application

### First Run

```bash
# Set encryption key
export NUIMANBOT_ENCRYPTION_KEY="your-32-byte-key"

# Set at least one LLM provider
export NUIMANBOT_LLM_ANTHROPIC_APIKEY="sk-ant-your-key"

# Run the application
./bin/nuimanbot
```

**Expected Output:**
```
NuimanBot starting...
Config file used: ./config.yaml
2026/02/06 12:00:00 INFO Database schema initialized successfully
2026/02/06 12:00:00 INFO Calculator skill registered
2026/02/06 12:00:00 INFO DateTime skill registered
2026/02/06 12:00:00 INFO Weather skill registered
2026/02/06 12:00:00 INFO WebSearch skill registered
2026/02/06 12:00:00 INFO Notes skill registered
2026/02/06 12:00:00 INFO Database connection pool configured max_open=25 max_idle=5
2026/02/06 12:00:00 INFO LLM response cache configured max_size=1000 ttl=1h
2026/02/06 12:00:00 INFO Starting health check server port=:8080
2026/02/06 12:00:00 INFO NuimanBot initialized with:
2026/02/06 12:00:00 INFO   Log Level: info
2026/02/06 12:00:00 INFO   Debug Mode: false
2026/02/06 12:00:00 INFO   LLM Provider: anthropic
2026/02/06 12:00:00 INFO   Skills Registered: 5

Starting CLI Gateway...
Type your messages below. Commands:
  - Type 'exit' or 'quit' to stop
  - Type 'help' for available skills

>
```

### Interactive Usage

Try these commands to verify functionality:

```
> Hello!
Bot: Hi! I'm NuimanBot. How can I help you today?

> What's 25 * 4?
Bot: The result is 100.

> What time is it?
Bot: The current time is 2026-02-06T12:00:00Z

> What's the weather in London?
Bot: London: Clear sky, 15°C, humidity 60%

> Create a note titled "Meeting" with content "Q1 planning session"
Bot: Note created successfully with ID: note-123

> exit
NuimanBot stopped gracefully.
```

---

## Verification

### Health Checks

NuimanBot exposes health check endpoints for monitoring:

```bash
# Liveness check (is the server running?)
curl http://localhost:8080/health

# Readiness check (are all dependencies healthy?)
curl http://localhost:8080/health/ready

# Version information
curl http://localhost:8080/health/version

# Prometheus metrics
curl http://localhost:8080/metrics
```

**Expected Responses:**

**Liveness:**
```json
{"status":"ok"}
```

**Readiness:**
```json
{
  "status": "ready",
  "checks": {
    "database": true,
    "llm": true,
    "vault": true
  }
}
```

**Metrics:**
```
# HELP llm_requests_total Total number of LLM requests
# TYPE llm_requests_total counter
llm_requests_total{provider="anthropic",model="claude-3-sonnet",status="success"} 42

# HELP cache_hits_total Total number of cache hits
# TYPE cache_hits_total counter
cache_hits_total{cache_type="llm"} 15
...
```

### Run Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Expected output:
# ok      nuimanbot/internal/usecase/chat    0.280s  coverage: 85.2% of statements
# ok      nuimanbot/internal/infrastructure/cache    0.532s  coverage: 100.0% of statements
# ...
```

### Database Verification

```bash
# Check database was created
ls -lh data/nuimanbot.db

# Inspect schema (optional - requires sqlite3 CLI)
sqlite3 data/nuimanbot.db ".schema"
```

---

## Production Deployment

### Environment-Specific Configuration

Set the environment in your configuration:

```yaml
server:
  environment: production  # or staging
  log_level: warn  # Less verbose in production
```

**Environment-Aware Validation:**
- **Development**: Relaxed validation, allows empty config
- **Staging**: Moderate validation, warns on missing optional config
- **Production**: Strict validation, requires all production settings

### Security Best Practices

1. **Never commit secrets to Git:**
   ```bash
   # Add to .gitignore (already included)
   .env
   config.yaml  # if it contains secrets
   data/
   ```

2. **Use environment variables for secrets:**
   ```bash
   # Use a secrets management system
   export NUIMANBOT_ENCRYPTION_KEY="$(aws secretsmanager get-secret-value ...)"
   ```

3. **Rotate encryption key periodically:**
   ```bash
   # NuimanBot supports multi-version vault for zero-downtime rotation
   # See documentation/secret-rotation.md for details
   ```

4. **Enable audit logging:**
   ```yaml
   security:
     audit_log_path: "/var/log/nuimanbot/audit.log"
   ```

### Running as a Service

**systemd (Linux):**

Create `/etc/systemd/system/nuimanbot.service`:

```ini
[Unit]
Description=NuimanBot AI Agent
After=network.target

[Service]
Type=simple
User=nuimanbot
WorkingDirectory=/opt/nuimanbot
Environment="NUIMANBOT_ENCRYPTION_KEY=your-key-here"
Environment="NUIMANBOT_LLM_ANTHROPIC_APIKEY=your-api-key"
ExecStart=/opt/nuimanbot/bin/nuimanbot
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable nuimanbot
sudo systemctl start nuimanbot
sudo systemctl status nuimanbot
```

**Docker (Optional):**

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o bin/nuimanbot ./cmd/nuimanbot

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite
WORKDIR /root/
COPY --from=builder /app/bin/nuimanbot .
COPY --from=builder /app/config.yaml .
EXPOSE 8080
CMD ["./nuimanbot"]
```

### Monitoring and Observability

**Prometheus Integration:**

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'nuimanbot'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:8080']
```

**Key Metrics to Monitor:**
- `llm_requests_total` - LLM API usage
- `cache_hits_total` / `cache_misses_total` - Cache efficiency
- `skill_executions_total` - Skill usage patterns
- `db_connections_open` - Database pool health
- `rate_limit_exceeded_total` - Rate limiting events

### Performance Tuning

**Configuration Options:**

```yaml
server:
  environment: production

# Database connection pooling
storage:
  max_open_connections: 25  # Increase for high load
  max_idle_connections: 5
  connection_max_lifetime: 5m

# LLM response caching
cache:
  llm_cache_size: 1000  # Number of responses to cache
  llm_cache_ttl: 1h     # Time-to-live for cached responses

# Rate limiting (per-user, per-skill)
rate_limits:
  default:
    requests: 10
    window: 1m
  websearch:
    requests: 5
    window: 1m
```

---

## Troubleshooting

### Common Issues

**Problem: `NUIMANBOT_ENCRYPTION_KEY is not set in environment`**

```bash
# Solution: Export the encryption key
export NUIMANBOT_ENCRYPTION_KEY="your-32-byte-key-here"
```

**Problem: `failed to connect to Anthropic API`**

```bash
# Check API key is valid
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $NUIMANBOT_LLM_ANTHROPIC_APIKEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-3-sonnet-20240229","max_tokens":10,"messages":[{"role":"user","content":"test"}]}'
```

**Problem: Database errors on startup**

```bash
# Solution: Remove and recreate database
rm data/nuimanbot.db
./bin/nuimanbot  # Will auto-create schema
```

**Problem: Port 8080 already in use**

```bash
# Solution: Change health check port
export NUIMANBOT_HEALTH_PORT=":9090"
```

### Debug Mode

Enable debug logging for troubleshooting:

```bash
export NUIMANBOT_SERVER_LOGLEVEL="debug"
./bin/nuimanbot
```

### Logs Location

- **Stdout**: Default log output
- **Audit logs**: Configured via `security.audit_log_path`
- **Health check logs**: Included in main application logs

### Getting Help

- **GitHub Issues**: https://github.com/stainedhead/NuimanBot/issues
- **Documentation**: See `README.md`, `AGENTS.md`, `PRODUCT_REQUIREMENT_DOC.md`
- **Examples**: Check `e2e/` directory for usage examples

---

## Next Steps

After successful installation:

1. **Explore Skills**: Try all built-in skills (calculator, datetime, weather, websearch, notes)
2. **Configure Gateways**: Set up Telegram or Slack integration (see `README.md`)
3. **Monitor Performance**: Set up Prometheus and Grafana dashboards
4. **Create Custom Skills**: Follow the skill development guide in `README.md`
5. **Review Security**: Read security best practices in `PRODUCT_REQUIREMENT_DOC.md`

---

**Congratulations!** You've successfully installed and configured NuimanBot. 🎉

For advanced configuration and customization, see the full documentation in the `documentation/` directory.
