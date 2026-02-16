#!/bin/bash
#
# backup-file-storage.sh
# Creates a timestamped backup of the NuimanBot file-based storage
#
# Usage: ./scripts/backup-file-storage.sh [data-dir] [backup-dir]
#

set -e  # Exit on error
set -u  # Exit on undefined variable

# Default directories
DATA_DIR="${1:-./data}"
BACKUP_DIR="${2:-./backups}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Timestamp for backup
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_NAME="nuimanbot_backup_$TIMESTAMP"
BACKUP_PATH="$BACKUP_DIR/$BACKUP_NAME"

echo "================================================"
echo "NuimanBot File-Based Storage Backup"
echo "================================================"
echo ""
echo "Source: $DATA_DIR"
echo "Destination: $BACKUP_PATH"
echo "Timestamp: $TIMESTAMP"
echo ""

# Verify source directory exists
if [ ! -d "$DATA_DIR" ]; then
    echo -e "${RED}Error: Data directory does not exist: $DATA_DIR${NC}"
    exit 1
fi

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

# Calculate source size
echo "Calculating backup size..."
SOURCE_SIZE=$(du -sh "$DATA_DIR" | cut -f1)
echo "Data directory size: $SOURCE_SIZE"
echo ""

# Check available disk space
AVAILABLE_SPACE=$(df -h "$BACKUP_DIR" | awk 'NR==2 {print $4}')
echo "Available space in backup directory: $AVAILABLE_SPACE"
echo ""

# Create backup using tar with compression
echo "Creating backup archive..."
echo "This may take a while depending on data size..."
echo ""

START_TIME=$(date +%s)

# Create tar.gz archive
# -c: create archive
# -z: compress with gzip
# -f: output file
# --exclude: exclude files we don't want to backup
tar -czf "$BACKUP_PATH.tar.gz" \
    --exclude="*.tmp" \
    --exclude=".DS_Store" \
    --exclude="*.swp" \
    -C "$(dirname "$DATA_DIR")" \
    "$(basename "$DATA_DIR")"

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# Calculate backup size
BACKUP_SIZE=$(du -sh "$BACKUP_PATH.tar.gz" | cut -f1)

echo -e "${GREEN}✓${NC} Backup created successfully!"
echo ""
echo "Backup details:"
echo "  File: $BACKUP_PATH.tar.gz"
echo "  Size: $BACKUP_SIZE (compressed from $SOURCE_SIZE)"
echo "  Duration: ${DURATION}s"
echo ""

# Create backup metadata file
cat > "$BACKUP_PATH.meta.json" << EOF
{
  "timestamp": "$TIMESTAMP",
  "backup_name": "$BACKUP_NAME",
  "source_dir": "$DATA_DIR",
  "backup_file": "$BACKUP_PATH.tar.gz",
  "source_size": "$SOURCE_SIZE",
  "backup_size": "$BACKUP_SIZE",
  "duration_seconds": $DURATION,
  "created_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "hostname": "$(hostname)",
  "nuimanbot_version": "$(git describe --tags --always 2>/dev/null || echo 'unknown')"
}
EOF

echo -e "${GREEN}✓${NC} Created backup metadata"
echo ""

# Verify backup integrity
echo "Verifying backup integrity..."
if tar -tzf "$BACKUP_PATH.tar.gz" > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} Backup archive is valid"
else
    echo -e "${RED}✗${NC} Backup archive verification failed!"
    exit 1
fi
echo ""

# List recent backups
echo "Recent backups in $BACKUP_DIR:"
ls -lth "$BACKUP_DIR"/*.tar.gz 2>/dev/null | head -5 || echo "  (no backups found)"
echo ""

# Cleanup old backups (keep last 7 days by default)
RETENTION_DAYS=${BACKUP_RETENTION_DAYS:-7}
echo "Checking for backups older than $RETENTION_DAYS days..."
OLD_BACKUPS=$(find "$BACKUP_DIR" -name "nuimanbot_backup_*.tar.gz" -type f -mtime +$RETENTION_DAYS 2>/dev/null || true)

if [ -n "$OLD_BACKUPS" ]; then
    echo "Found old backups to remove:"
    echo "$OLD_BACKUPS"
    echo ""
    read -p "Remove these old backups? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "$OLD_BACKUPS" | xargs rm -f
        # Also remove corresponding metadata files
        echo "$OLD_BACKUPS" | sed 's/\.tar\.gz$/.meta.json/' | xargs rm -f 2>/dev/null || true
        echo -e "${GREEN}✓${NC} Old backups removed"
    else
        echo "Kept old backups"
    fi
else
    echo "No old backups to remove"
fi
echo ""

echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN}✓ Backup Complete!${NC}"
echo -e "${GREEN}================================================${NC}"
echo ""
echo "Backup location: $BACKUP_PATH.tar.gz"
echo "Metadata: $BACKUP_PATH.meta.json"
echo ""
echo "To restore this backup, run:"
echo "  ./scripts/restore-file-storage.sh $BACKUP_PATH.tar.gz"
echo ""
