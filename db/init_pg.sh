#!/bin/bash

DB_NAME=$1
SCHEMA_FILE=$2

if [ -z "$DB_NAME" ] || [ -z "$SCHEMA_FILE" ]; then
  echo "Usage: $0 <db_name> <schema_file.sql>"
  exit 1
fi

psql -h localhost -U pguser -d "$DB_NAME" -f "$SCHEMA_FILE"
