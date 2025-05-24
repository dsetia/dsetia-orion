#!/bin/bash

# config file
CONFIG_DIR="config"
DB_DIR="db"
CONFIG_FILE=${1:-"$CONFIG_DIR/apis_config.json"}
SCHEMA_FILE=${2:-"$DB_DIR/schema_pg.sql"}

# Print usage/help
usage() {
    echo "Usage: $0 [db-config-path] [db-schema-path]"
    echo "  Drop and reinitialize the database"
    echo "  db-config-path: Path to DB config JSON (default: $CONFIG_DIR/apis_config.json)"
    echo "  db-schema-path: Path to DB schema SQL (default: $DB/schema_pg.sql)"
    exit 1
}

[[ $# -lt 1 ]] && usage

if ! command -v jq &>/dev/null; then
    echo "❌ 'jq' is required but not installed. Please run: sudo apt-get install jq"
    exit 1
fi

# Read config values into variables
# host is local since this script is run outside the containers
host=$(jq -r '.host' "$CONFIG_FILE")
port=$(jq -r '.port' "$CONFIG_FILE")
user=$(jq -r '.user' "$CONFIG_FILE")
password=$(jq -r '.password' "$CONFIG_FILE")
dbname=$(jq -r '.dbname' "$CONFIG_FILE")
sslmode=$(jq -r '.sslmode' "$CONFIG_FILE")

# Print config and ask for confirmation
echo "✅ Loaded configuration:"
echo "  Host     : $host"
echo "  Port     : $port"
echo "  User     : $user"
echo "  Password : $password"
echo "  DB Name  : $dbname"
echo "  SSL Mode : $sslmode"

read -p "❓ Proceed with this configuration? [y/N] " confirm
confirm=${confirm,,}  # to lowercase

if [[ "$confirm" != "y" && "$confirm" != "yes" ]]; then
    echo "❌ Aborting."
    exit 1
fi

export PGPASSWORD=$password

echo # Dropping database $dbname"
psql -U "$user" -h "$host" -p "$port" -d postgres -c "DROP DATABASE IF EXISTS $dbname WITH (FORCE);"

echo "Creating database $dbname (if it doesn't exist)..."
createdb -U $user -h $host -p $port $dbname

echo "Applying schema from $SCHEMA_FILE..."
psql -p $port -h $host -U "$user" -d "$dbname" -f "$SCHEMA_FILE"
