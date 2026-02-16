#!/bin/bash
#
# init-file-storage.sh
# Initializes the file-based storage directory structure for NuimanBot
#
# Usage: ./scripts/init-file-storage.sh [data-dir]
#

set -e  # Exit on error
set -u  # Exit on undefined variable

# Default data directory
DATA_DIR="${1:-./data}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "================================================"
echo "NuimanBot File-Based Storage Initialization"
echo "================================================"
echo ""

# Check if data directory already exists
if [ -d "$DATA_DIR" ]; then
    echo -e "${YELLOW}Warning: Data directory already exists at: $DATA_DIR${NC}"
    read -p "Do you want to continue? This will not delete existing data. (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
fi

echo "Creating directory structure at: $DATA_DIR"
echo ""

# Create main directories
mkdir -p "$DATA_DIR/users"
mkdir -p "$DATA_DIR/system"

# Set permissions
# Directories: 0755 (rwxr-xr-x)
# Files will be created with 0644 (rw-r--r--) by the application
chmod 755 "$DATA_DIR"
chmod 755 "$DATA_DIR/users"
chmod 755 "$DATA_DIR/system"

echo -e "${GREEN}✓${NC} Created directory structure:"
echo "  $DATA_DIR/"
echo "  ├── users/     (user-specific data)"
echo "  └── system/    (system-wide data)"
echo ""

# Create .gitignore in data directory to prevent committing user data
cat > "$DATA_DIR/.gitignore" << 'EOF'
# Ignore all user data
users/
system/

# Keep directory structure
!.gitignore
!README.md
EOF

echo -e "${GREEN}✓${NC} Created .gitignore to protect user data"
echo ""

# Create README in data directory
cat > "$DATA_DIR/README.md" << 'EOF'
# NuimanBot Data Directory

This directory contains file-based storage for NuimanBot.

## Structure

```
data/
├── users/          # User-specific data (profiles, conversations, notes, memory)
│   └── <user-id>/  # Per-user directory
│       ├── profile.json
│       ├── conversations/
│       ├── notes/
│       └── memory/
└── system/         # System-wide data (audit logs, configurations)
    └── audit.jsonl
```

## Permissions

- Directories: 0755 (rwxr-xr-x)
- Files: 0644 (rw-r--r--)

## Backup

Recommended backup strategy:
1. Daily incremental backups of the entire data/ directory
2. Weekly full backups
3. Store backups in a separate location
4. Test restore procedures regularly

## Security

- This directory contains user data and should be protected
- Ensure file system permissions are correctly set
- Do not commit this directory to version control
- Encrypt backups at rest

## Recovery

To restore from backup:
1. Stop the NuimanBot application
2. Replace the data/ directory with the backup
3. Verify file permissions (run init-file-storage.sh)
4. Restart the application
EOF

echo -e "${GREEN}✓${NC} Created README.md with directory documentation"
echo ""

# Create a default admin user profile if it doesn't exist
DEFAULT_ADMIN_PROFILE="$DATA_DIR/users/admin/profile.json"
if [ ! -f "$DEFAULT_ADMIN_PROFILE" ]; then
    echo "Creating default admin user..."

    mkdir -p "$DATA_DIR/users/admin"
    mkdir -p "$DATA_DIR/users/admin/conversations"
    mkdir -p "$DATA_DIR/users/admin/notes"
    mkdir -p "$DATA_DIR/users/admin/memory"

    # Create default admin profile
    cat > "$DEFAULT_ADMIN_PROFILE" << 'EOF'
{
  "userID": "admin",
  "primaryEmail": "admin@localhost",
  "userType": "admin",
  "moniker": "admin",
  "firstName": "System",
  "lastName": "Administrator",
  "platformIDs": {
    "cli": "admin"
  },
  "createdAt": "TIMESTAMP",
  "updatedAt": "TIMESTAMP"
}
EOF

    # Replace TIMESTAMP with current ISO 8601 timestamp
    TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    sed -i.bak "s/TIMESTAMP/$TIMESTAMP/g" "$DEFAULT_ADMIN_PROFILE" && rm "$DEFAULT_ADMIN_PROFILE.bak"

    chmod 644 "$DEFAULT_ADMIN_PROFILE"

    echo -e "${GREEN}✓${NC} Created default admin user profile"
    echo "  User ID: admin"
    echo "  Email: admin@localhost"
    echo ""
else
    echo -e "${YELLOW}⚠${NC} Admin user already exists, skipping creation"
    echo ""
fi

# Create system audit log if it doesn't exist
AUDIT_LOG="$DATA_DIR/system/audit.jsonl"
if [ ! -f "$AUDIT_LOG" ]; then
    touch "$AUDIT_LOG"
    chmod 644 "$AUDIT_LOG"
    echo -e "${GREEN}✓${NC} Created system audit log"
    echo ""
else
    echo -e "${YELLOW}⚠${NC} Audit log already exists"
    echo ""
fi

# Verify directory structure
echo "Verifying directory structure..."
DIRS=("$DATA_DIR" "$DATA_DIR/users" "$DATA_DIR/system")
ALL_OK=true

for dir in "${DIRS[@]}"; do
    if [ -d "$dir" ]; then
        PERMS=$(stat -f "%Op" "$dir" 2>/dev/null || stat -c "%a" "$dir" 2>/dev/null)
        if [ "$PERMS" = "755" ] || [ "$PERMS" = "40755" ]; then
            echo -e "  ${GREEN}✓${NC} $dir (permissions: 0755)"
        else
            echo -e "  ${RED}✗${NC} $dir (permissions: $PERMS, expected: 0755)"
            ALL_OK=false
        fi
    else
        echo -e "  ${RED}✗${NC} $dir (does not exist)"
        ALL_OK=false
    fi
done

echo ""

if [ "$ALL_OK" = true ]; then
    echo -e "${GREEN}================================================${NC}"
    echo -e "${GREEN}✓ Initialization Complete!${NC}"
    echo -e "${GREEN}================================================${NC}"
    echo ""
    echo "Data directory ready at: $DATA_DIR"
    echo ""
    echo "Next steps:"
    echo "1. Configure NuimanBot to use this data directory"
    echo "2. Start the application: ./bin/nuimanbot"
    echo "3. Set up automated backups (see scripts/backup-file-storage.sh)"
    echo ""
else
    echo -e "${RED}================================================${NC}"
    echo -e "${RED}✗ Initialization completed with errors${NC}"
    echo -e "${RED}================================================${NC}"
    echo ""
    echo "Please review the errors above and fix them manually."
    exit 1
fi
