#!/bin/bash

# config file
CONFIG_FILE="config/apis_config.json"
SCHEMA_FILE="db/schema_pg.sql"

if ! command -v jq &>/dev/null; then
    echo "❌ 'jq' is required but not installed. Please run: sudo apt-get install jq"
    exit 1
fi

# Read config values into variables
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
dropdb -U $user -h $host -p $port $dbname

echo "Creating database $dbname (if it doesn't exist)..."
createdb -U $user -h $host -p $port $dbname

echo "Applying schema from $SCHEMA_FILE..."
psql -p $port -h $host -U "$user" -d "$dbname" -f "$SCHEMA_FILE"
