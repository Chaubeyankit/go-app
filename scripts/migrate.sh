#!/bin/bash

# Migration helper script
# Usage: ./scripts/migrate.sh [command] [args]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Database connection
DB_DSN="postgres://ankit:Root@123@localhost:5432/myapp?sslmode=disable"
MIGRATIONS_PATH="./migrations"

# Function to get the next migration version
get_next_version() {
    local last_version=$(ls -1 "$MIGRATIONS_PATH"/*.up.sql 2>/dev/null | sed 's/.*\/\([0-9]*\)_.*/\1/' | sort -n | tail -1)
    if [ -z "$last_version" ]; then
        echo "000001"
    else
        # Remove leading zeros, add 1, pad back to 6 digits
        local num=$((10#$last_version + 1))
        printf "%06d" $num
    fi
}

# Function to create new migration
create_migration() {
    local name="$1"

    if [ -z "$name" ]; then
        echo -e "${RED}Error: Migration name is required${NC}"
        echo "Usage: ./scripts/migrate.sh create <migration_name>"
        echo "Example: ./scripts/migrate.sh create add_posts_table"
        exit 1
    fi

    # Convert name to lowercase and replace spaces with underscores
    name=$(echo "$name" | tr '[:upper:]' '[:lower:]' | tr ' ' '_')

    local version=$(get_next_version)
    local up_file="$MIGRATIONS_PATH/${version}_${name}.up.sql"
    local down_file="$MIGRATIONS_PATH/${version}_${name}.down.sql"

    # Create files
    cat > "$up_file" <<EOF
-- Migration: ${name}
-- Version: ${version}
-- Created at: $(date '+%Y-%m-%d %H:%M:%S')

-- Write your UP migration here
-- Example:
-- CREATE TABLE example (
--     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
--     created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
-- );

EOF

    cat > "$down_file" <<EOF
-- Migration: ${name}
-- Version: ${version}
-- Created at: $(date '+%Y-%m-%d %H:%M:%S')

-- Write your DOWN migration here (to rollback)
-- Example:
-- DROP TABLE IF EXISTS example;

EOF

    echo -e "${GREEN}✓ Created migration files:${NC}"
    echo "  - $up_file"
    echo "  - $down_file"
    echo ""
    echo -e "${YELLOW}Next steps:${NC}"
    echo "  1. Edit the migration files with your schema changes"
    echo "  2. Run: ./scripts/migrate.sh up"
}

# Function to run up migrations
migrate_up() {
    echo -e "${GREEN}Running migrations UP...${NC}"
    ~/go/bin/migrate -path "$MIGRATIONS_PATH" -database "$DB_DSN" up
    echo -e "${GREEN}✓ Migrations completed successfully${NC}"
}

# Function to run down migrations
migrate_down() {
    local steps="${1:-1}"
    echo -e "${YELLOW}Rolling back $steps migration(s)...${NC}"
    ~/go/bin/migrate -path "$MIGRATIONS_PATH" -database "$DB_DSN" down $steps
    echo -e "${GREEN}✓ Rollback completed${NC}"
}

# Function to check migration version
migrate_version() {
    echo -e "${GREEN}Current migration version:${NC}"
    ~/go/bin/migrate -path "$MIGRATIONS_PATH" -database "$DB_DSN" version
}

# Function to force migration version
migrate_force() {
    local version="$1"
    if [ -z "$version" ]; then
        echo -e "${RED}Error: Version is required${NC}"
        echo "Usage: ./scripts/migrate.sh force <version>"
        exit 1
    fi
    echo -e "${YELLOW}Forcing migration version to $version...${NC}"
    ~/go/bin/migrate -path "$MIGRATIONS_PATH" -database "$DB_DSN" force $version
    echo -e "${GREEN}✓ Version forced${NC}"
}

# Main command dispatcher
case "${1:-}" in
    create)
        create_migration "$2"
        ;;
    up)
        migrate_up
        ;;
    down)
        migrate_down "$2"
        ;;
    version)
        migrate_version
        ;;
    force)
        migrate_force "$2"
        ;;
    status)
        migrate_version
        ;;
    *)
        cat <<EOF
${GREEN}Migration Helper Script${NC}

Usage: ./scripts/migrate.sh <command> [args]

${YELLOW}Commands:${NC}
  create <name>       Create a new migration file pair
                      Example: ./scripts/migrate.sh create add_posts_table

  up                  Run all pending migrations
                      Example: ./scripts/migrate.sh up

  down [steps]        Rollback migrations (default: 1 step)
                      Example: ./scripts/migrate.sh down 2

  version             Show current migration version
                      Example: ./scripts/migrate.sh version

  force <version>     Force migration to specific version
                      Example: ./scripts/migrate.sh force 000002

${YELLOW}Examples:${NC}
  ./scripts/migrate.sh create add_users_table
  ./scripts/migrate.sh up
  ./scripts/migrate.sh down
  ./scripts/migrate.sh version

${YELLOW}Quick Start:${NC}
  1. Create migration: ./scripts/migrate.sh create your_migration_name
  2. Edit the generated files in migrations/ directory
  3. Run migrations: ./scripts/migrate.sh up
EOF
        ;;
esac
