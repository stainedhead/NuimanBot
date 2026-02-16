#!/bin/bash
#
# restore-file-storage.sh
# Restores NuimanBot file-based storage from a backup archive
#
# Usage: ./scripts/restore-file-storage.sh <backup-file.tar.gz> [target-data-dir]
#

set -e  # Exit on error
set -u  # Exit on undefined variable

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check arguments
if [ $# -lt 1 ]; then
    echo "Usage: $0 <backup-file.tar.gz> [target-data-dir]"
    echo ""
    echo "Example:"
    echo "  $0 ./backups/nuimanbot_backup_20260216_143022.tar.gz"
    echo "  $0 ./backups/nuimanbot_backup_20260216_143022.tar.gz ./data"
    exit 1
fi

BACKUP_FILE="$1"
TARGET_DIR="${2:-./data}"

echo "================================================"
echo "NuimanBot File-Based Storage Restore"
echo "================================================"
echo ""

# Verify backup file exists
if [ ! -f "$BACKUP_FILE" ]; then
    echo -e "${RED}Error: Backup file does not exist: $BACKUP_FILE${NC}"
    exit 1
fi

# Verify backup file is a valid tar.gz
if ! tar -tzf "$BACKUP_FILE" > /dev/null 2>&1; then
    echo -e "${RED}Error: Backup file is not a valid tar.gz archive${NC}"
    exit 1
fi

echo "Backup file: $BACKUP_FILE"
echo "Target directory: $TARGET_DIR"
echo ""

# Show backup metadata if available
META_FILE="${BACKUP_FILE%.tar.gz}.meta.json"
if [ -f "$META_FILE" ]; then
    echo "Backup metadata:"
    cat "$META_FILE" | grep -E "(timestamp|source_size|backup_size|created_at)" | sed 's/^/  /'
    echo ""
fi

# List contents of backup
echo "Backup contents:"
tar -tzf "$BACKUP_FILE" | head -10
TOTAL_FILES=$(tar -tzf "$BACKUP_FILE" | wc -l)
echo "  ... ($TOTAL_FILES total files/directories)"
echo ""

# Check if target directory exists and has data
if [ -d "$TARGET_DIR" ] && [ "$(ls -A "$TARGET_DIR" 2>/dev/null)" ]; then
    echo -e "${YELLOW}WARNING: Target directory already exists and contains data!${NC}"
    echo "Current contents will be backed up before restore."
    echo ""
    read -p "Continue with restore? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi

    # Create safety backup of existing data
    SAFETY_BACKUP="$TARGET_DIR.before_restore_$(date +%Y%m%d_%H%M%S)"
    echo ""
    echo "Creating safety backup of existing data..."
    mv "$TARGET_DIR" "$SAFETY_BACKUP"
    echo -e "${GREEN}✓${NC} Existing data backed up to: $SAFETY_BACKUP"
    echo ""
fi

# Extract backup
echo "Restoring from backup..."
echo "This may take a while..."
echo ""

START_TIME=$(date +%s)

# Create parent directory if needed
mkdir -p "$(dirname "$TARGET_DIR")"

# Extract backup
# The backup contains the data directory, so we extract to parent and rename if needed
EXTRACT_DIR="$(dirname "$TARGET_DIR")"
tar -xzf "$BACKUP_FILE" -C "$EXTRACT_DIR"

# If the extracted directory name doesn't match target, rename it
EXTRACTED_NAME=$(tar -tzf "$BACKUP_FILE" | head -1 | cut -d'/' -f1)
if [ "$EXTRACTED_NAME" != "$(basename "$TARGET_DIR")" ]; then
    mv "$EXTRACT_DIR/$EXTRACTED_NAME" "$TARGET_DIR"
fi

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo -e "${GREEN}✓${NC} Restore completed in ${DURATION}s"
echo ""

# Verify restored directory
echo "Verifying restored data..."
RESTORED_SIZE=$(du -sh "$TARGET_DIR" | cut -f1)
echo "  Restored directory size: $RESTORED_SIZE"

# Check key directories exist
DIRS=("$TARGET_DIR/users" "$TARGET_DIR/system")
ALL_OK=true

for dir in "${DIRS[@]}"; do
    if [ -d "$dir" ]; then
        echo -e "  ${GREEN}✓${NC} $dir exists"
    else
        echo -e "  ${RED}✗${NC} $dir missing"
        ALL_OK=false
    fi
done
echo ""

# Fix permissions if needed
echo "Fixing permissions..."
find "$TARGET_DIR" -type d -exec chmod 755 {} \;
find "$TARGET_DIR" -type f -exec chmod 644 {} \;
echo -e "${GREEN}✓${NC} Permissions updated"
echo ""

if [ "$ALL_OK" = true ]; then
    echo -e "${GREEN}================================================${NC}"
    echo -e "${GREEN}✓ Restore Complete!${NC}"
    echo -e "${GREEN}================================================${NC}"
    echo ""
    echo "Data has been restored to: $TARGET_DIR"
    echo ""
    echo "Next steps:"
    echo "1. Verify the restored data is correct"
    echo "2. Start NuimanBot: ./bin/nuimanbot"
    echo "3. Test functionality to ensure everything works"
    echo ""
    if [ -d "${TARGET_DIR}.before_restore_"* 2>/dev/null ]; then
        echo "Note: Your original data was backed up to:"
        echo "  $SAFETY_BACKUP"
        echo "You can remove it once you verify the restore succeeded."
        echo ""
    fi
else
    echo -e "${RED}================================================${NC}"
    echo -e "${RED}✗ Restore completed with warnings${NC}"
    echo -e "${RED}================================================${NC}"
    echo ""
    echo "Some expected directories are missing."
    echo "Please verify the backup file and try again."
    exit 1
fi
