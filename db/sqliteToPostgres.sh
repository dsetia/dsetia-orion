#!/bin/bash

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 input_schema.sql output_schema_pg.sql"
  exit 1
fi

INPUT="$1"
OUTPUT="$2"

cat "$INPUT" \
  | sed 's/INTEGER PRIMARY KEY AUTOINCREMENT/SERIAL PRIMARY KEY/g' \
  | sed 's/AUTOINCREMENT//g' \
  | sed 's/BLOB/BYTEA/g' \
  | sed 's/TEXT/VARCHAR/g' \
  | sed 's/BOOLEAN/BOOLEAN/g' \
  > "$OUTPUT"

echo "Converted schema written to $OUTPUT"
