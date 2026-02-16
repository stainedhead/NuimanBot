#!/bin/bash
#
# monitor-file-storage.sh
# Monitors NuimanBot file-based storage health and reports metrics
#
# Usage: ./scripts/monitor-file-storage.sh [data-dir]
#        ./scripts/monitor-file-storage.sh [data-dir] --json (for machine-readable output)
#

set -e  # Exit on error
set -u  # Exit on undefined variable

# Default data directory
DATA_DIR="${1:-./data}"
OUTPUT_JSON=false

# Check for --json flag
if [ "${2:-}" = "--json" ]; then
    OUTPUT_JSON=true
fi

# Colors for output (only if not JSON)
if [ "$OUTPUT_JSON" = false ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

# Verify data directory exists
if [ ! -d "$DATA_DIR" ]; then
    if [ "$OUTPUT_JSON" = true ]; then
        echo '{"error": "Data directory does not exist", "path": "'"$DATA_DIR"'"}'
    else
        echo -e "${RED}Error: Data directory does not exist: $DATA_DIR${NC}"
    fi
    exit 1
fi

# Collect metrics
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Disk usage
TOTAL_SIZE=$(du -sh "$DATA_DIR" 2>/dev/null | cut -f1 || echo "N/A")
TOTAL_SIZE_BYTES=$(du -sk "$DATA_DIR" 2>/dev/null | cut -f1 || echo "0")

# User count
USER_COUNT=$(find "$DATA_DIR/users" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')

# File counts per user directory
PROFILE_COUNT=$(find "$DATA_DIR/users" -name "profile.json" -type f 2>/dev/null | wc -l | tr -d ' ')
CONVERSATION_COUNT=$(find "$DATA_DIR/users" -type d -name "conversations" -exec sh -c 'find "$1" -name "*.jsonl" | wc -l' _ {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}')
NOTE_COUNT=$(find "$DATA_DIR/users" -type d -name "notes" -exec sh -c 'find "$1" -name "*.json" | wc -l' _ {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}')
MEMORY_CELL_COUNT=$(find "$DATA_DIR/users" -type d -name "memory" -exec sh -c 'find "$1" -name "*.json" | wc -l' _ {} \; 2>/dev/null | awk '{sum+=$1} END {print sum}')

# System files
AUDIT_LOG_SIZE="0"
AUDIT_LOG_LINES="0"
if [ -f "$DATA_DIR/system/audit.jsonl" ]; then
    AUDIT_LOG_SIZE=$(du -sh "$DATA_DIR/system/audit.jsonl" 2>/dev/null | cut -f1 || echo "0")
    AUDIT_LOG_LINES=$(wc -l < "$DATA_DIR/system/audit.jsonl" 2>/dev/null | tr -d ' ' || echo "0")
fi

# Disk space available
DISK_AVAILABLE=$(df -h "$DATA_DIR" 2>/dev/null | awk 'NR==2 {print $4}' || echo "N/A")
DISK_USED_PERCENT=$(df -h "$DATA_DIR" 2>/dev/null | awk 'NR==2 {print $5}' || echo "N/A")

# Check for issues
ISSUES=()

# Check disk usage threshold (warn if > 80%)
DISK_USED_NUM=$(echo "$DISK_USED_PERCENT" | tr -d '%')
if [ "$DISK_USED_NUM" -gt 80 ] 2>/dev/null; then
    ISSUES+=("Disk usage is high: $DISK_USED_PERCENT")
fi

# Check if data directory is writable
if [ ! -w "$DATA_DIR" ]; then
    ISSUES+=("Data directory is not writable")
fi

# Check directory permissions
EXPECTED_DIRS=("$DATA_DIR/users" "$DATA_DIR/system")
for dir in "${EXPECTED_DIRS[@]}"; do
    if [ ! -d "$dir" ]; then
        ISSUES+=("Missing directory: $dir")
    fi
done

# Health status
HEALTH_STATUS="healthy"
if [ ${#ISSUES[@]} -gt 0 ]; then
    HEALTH_STATUS="warning"
fi

# Output results
if [ "$OUTPUT_JSON" = true ]; then
    # JSON output for programmatic use
    cat << EOF
{
  "timestamp": "$TIMESTAMP",
  "data_directory": "$DATA_DIR",
  "health_status": "$HEALTH_STATUS",
  "storage": {
    "total_size": "$TOTAL_SIZE",
    "total_size_kb": $TOTAL_SIZE_BYTES,
    "disk_available": "$DISK_AVAILABLE",
    "disk_used_percent": "$DISK_USED_PERCENT"
  },
  "users": {
    "total_count": $USER_COUNT,
    "profile_count": $PROFILE_COUNT
  },
  "data": {
    "conversations": $CONVERSATION_COUNT,
    "notes": $NOTE_COUNT,
    "memory_cells": $MEMORY_CELL_COUNT,
    "audit_log_lines": $AUDIT_LOG_LINES,
    "audit_log_size": "$AUDIT_LOG_SIZE"
  },
  "issues": [
$([ ${#ISSUES[@]} -gt 0 ] && printf '    "%s"' "${ISSUES[@]}" | paste -sd ',' - || echo "")
  ]
}
EOF
else
    # Human-readable output
    echo "================================================"
    echo "NuimanBot File Storage Health Monitor"
    echo "================================================"
    echo ""
    echo "Timestamp: $TIMESTAMP"
    echo "Data Directory: $DATA_DIR"
    echo ""

    # Health status
    if [ "$HEALTH_STATUS" = "healthy" ]; then
        echo -e "Status: ${GREEN}✓ Healthy${NC}"
    else
        echo -e "Status: ${YELLOW}⚠ Warning${NC}"
    fi
    echo ""

    # Storage metrics
    echo -e "${BLUE}Storage:${NC}"
    echo "  Total Size: $TOTAL_SIZE"
    echo "  Disk Available: $DISK_AVAILABLE"
    echo "  Disk Used: $DISK_USED_PERCENT"
    echo ""

    # User metrics
    echo -e "${BLUE}Users:${NC}"
    echo "  Total Users: $USER_COUNT"
    echo "  User Profiles: $PROFILE_COUNT"
    echo ""

    # Data metrics
    echo -e "${BLUE}Data:${NC}"
    echo "  Conversations: $CONVERSATION_COUNT"
    echo "  Notes: $NOTE_COUNT"
    echo "  Memory Cells: $MEMORY_CELL_COUNT"
    echo "  Audit Log Entries: $AUDIT_LOG_LINES"
    echo "  Audit Log Size: $AUDIT_LOG_SIZE"
    echo ""

    # Issues
    if [ ${#ISSUES[@]} -gt 0 ]; then
        echo -e "${YELLOW}Issues:${NC}"
        for issue in "${ISSUES[@]}"; do
            echo -e "  ${YELLOW}⚠${NC} $issue"
        done
        echo ""
    fi

    # Recommendations
    echo -e "${BLUE}Recommendations:${NC}"
    if [ "$DISK_USED_NUM" -gt 80 ] 2>/dev/null; then
        echo "  • Consider archiving old data or expanding disk space"
    fi
    if [ "$USER_COUNT" -gt 100 ]; then
        echo "  • Monitor performance with high user count"
    fi
    if [ "$AUDIT_LOG_LINES" -gt 100000 ]; then
        echo "  • Consider rotating audit logs"
    fi
    echo "  • Run backups regularly: ./scripts/backup-file-storage.sh"
    echo "  • Monitor disk I/O performance"
    echo ""

    echo "================================================"
fi
