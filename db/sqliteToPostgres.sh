#!/bin/bash
#
# Copyright (c) 2025 SecurITe
# All rights reserved.
#
# This source code is the property of SecurITe.
# Unauthorized copying, modification, or distribution of this file,
# via any medium is strictly prohibited unless explicitly authorized
# by SecurITe.
#
# This software is proprietary and confidential.

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
