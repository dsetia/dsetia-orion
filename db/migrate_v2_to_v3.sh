#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# Migrate: Add 'location' column to devices table (idempotent)
# Reads DB connection from JSON config file
# =============================================================================

SCRIPT_NAME=$(basename "$0")

usage() {
    cat <<EOF
Usage: $SCRIPT_NAME -c /path/to/db-config.json

Options:
  -c, --config   Path to JSON config file (required)
  -h, --help     Show this help message

Example:
  $SCRIPT_NAME -c /opt/config/db.json

Expected config format:
{
  "host": "localhost",
  "port": 5432,
  "user": "pguser",
  "password": "pgpass",
  "dbname": "orion",
  "sslmode": "disable"
}
EOF
    exit 1
}

# Parse command line arguments
CONFIG_FILE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--config)
            CONFIG_FILE="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

if [[ -z "$CONFIG_FILE" ]]; then
    echo "Error: Config file is required (-c /path/to/config.json)"
    usage
fi

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "Error: Config file not found: $CONFIG_FILE"
    exit 1
fi

# Read values from JSON using jq (make sure jq is installed)
if ! command -v jq &> /dev/null; then
    echo "Error: 'jq' is required but not installed."
    echo "On Ubuntu/Debian: sudo apt-get install jq"
    echo "On macOS: brew install jq"
    exit 1
fi

HOST=$(jq -r '.host' "$CONFIG_FILE")
PORT=$(jq -r '.port' "$CONFIG_FILE")
USER=$(jq -r '.user' "$CONFIG_FILE")
PASSWORD=$(jq -r '.password' "$CONFIG_FILE")
DBNAME=$(jq -r '.dbname' "$CONFIG_FILE")
SSLMODE=$(jq -r '.sslmode // "prefer"' "$CONFIG_FILE")   # default to prefer if missing

# Basic validation
if [[ "$HOST" == "null" || "$USER" == "null" || "$PASSWORD" == "null" || "$DBNAME" == "null" ]]; then
    echo "Error: Missing required fields (host, user, password, dbname) in config file"
    exit 1
fi

# Export password for psql
export PGPASSWORD="$PASSWORD"

echo "Connecting to: $USER@$HOST:$PORT/$DBNAME (sslmode=$SSLMODE)"

# Run the migration
psql -h "$HOST" -p "$PORT" -U "$USER" -d "$DBNAME" --set=sslmode="$SSLMODE" <<'EOF'
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'devices'
          AND column_name = 'location'
    ) THEN
        ALTER TABLE devices
            ADD COLUMN location TEXT;
        RAISE NOTICE 'Added column "location" to table "devices"';
    ELSE
        RAISE NOTICE 'Column "location" already exists in table "devices" — no change needed';
    END IF;
END $$;

-- Optional: show current schema of devices table for verification
\dt+ devices
EOF

if [[ $? -eq 0 ]]; then
    echo ""
    echo "Migration completed successfully."
else
    echo ""
    echo "Migration failed. Check the output above for errors."
    exit 1
fi
