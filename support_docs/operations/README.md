# NuimanBot Operational Scripts

**Status**: Optional helpers for operations and deployment

These scripts are **NOT required** for running NuimanBot. The application handles core functionality automatically:
- ✅ Directory structure creation (auto-created on startup)
- ✅ Default admin user setup (auto-created on first run)
- ✅ Health monitoring (available via `/health` and `/metrics` HTTP endpoints)

## Purpose

These scripts provide **optional operational helpers** for deployment and maintenance scenarios:
- Backup and restore procedures
- External monitoring integration
- Manual initialization (for advanced deployment scenarios)

## Scripts

### init-file-storage.sh (OPTIONAL)

Manual initialization script for advanced deployment scenarios.

**When NOT needed (99% of cases):**
- Normal application startup (app auto-creates directories)
- Docker/Kubernetes deployments (use init containers if needed)
- Development environments (app handles it)

**When potentially useful:**
- Pre-creating data directories before app deployment
- Setting up specific file permissions in restricted environments
- Testing deployment procedures

```bash
./support_docs/operations/init-file-storage.sh [data-dir]
```

### backup-file-storage.sh

Creates compressed backups with metadata and retention management.

```bash
# Create backup
./support_docs/operations/backup-file-storage.sh [data-dir] [backup-dir]

# Set retention period (default: 7 days)
BACKUP_RETENTION_DAYS=30 ./support_docs/operations/backup-file-storage.sh
```

### restore-file-storage.sh

Restores from a backup archive.

```bash
./support_docs/operations/restore-file-storage.sh <backup-file.tar.gz> [data-dir]
```

### monitor-file-storage.sh

External monitoring script for integration with monitoring systems.

**Note**: The application exposes this data via `/health` and `/metrics` endpoints. Use those instead of this script unless you need external shell-based monitoring.

```bash
# Human-readable output
./support_docs/operations/monitor-file-storage.sh [data-dir]

# JSON output (for external monitoring)
./support_docs/operations/monitor-file-storage.sh [data-dir] --json
```

### crontab.example

Example cron configuration for automated backups.

```bash
# Install example cron jobs
crontab support_docs/operations/crontab.example

# Or edit your crontab manually
crontab -e
```

## Recommended Deployment Approach

**Instead of using these scripts:**

1. **Let the app auto-initialize**: Just start NuimanBot, it creates required directories and default admin user
2. **Use HTTP endpoints for monitoring**: Query `/health` and `/metrics` endpoints
3. **Use infrastructure tools for backups**:
   - AWS S3 sync
   - rsync to backup server
   - Velero for Kubernetes
   - Volume snapshots

**Use these scripts only if:**
- You need shell-based automation in your deployment pipeline
- Your backup strategy requires tar/gzip archives
- Your monitoring system can't query HTTP endpoints

## Application-Native Features

The following features are built into NuimanBot (no scripts needed):

- ✅ **Auto-initialization** (`internal/infrastructure/storage/init.go`)
  - Creates `data/users/` and `data/system/` directories
  - Creates default admin user on first run
  - Sets appropriate file permissions

- ✅ **Health Monitoring** (`/health` endpoint)
  - Storage health checks
  - Disk usage metrics
  - User and data counts
  - Available in JSON format for monitoring systems

- ✅ **Metrics Export** (`/metrics` endpoint)
  - Prometheus-compatible metrics
  - Storage metrics, user counts, file counts
  - Ready for Grafana dashboards

## See Also

- [Installation & Setup Guide](../install-and-setup.md) - Application setup
- [Admin Guide](../admin-guide.md) - Administration via REST API
- [Technical Details](../../documentation/technical-details.md) - Architecture

---

**Bottom line**: These scripts are convenience tools. The application is self-sufficient and doesn't require manual initialization or external monitoring scripts.
