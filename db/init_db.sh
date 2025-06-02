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

# Check for correct number of arguments
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <database_file> <schema_file>"
    exit 1
fi

DB_FILE="$1"
SCHEMA_FILE="$2"

# Check if schema file exists
if [ ! -f "$SCHEMA_FILE" ]; then
    echo "Schema file '$SCHEMA_FILE' not found!"
    exit 1
fi

# Create or initialize the database
sqlite3 "$DB_FILE" < "$SCHEMA_FILE"

if [ $? -eq 0 ]; then
    echo "Database '$DB_FILE' initialized successfully from '$SCHEMA_FILE'"
else
    echo "Failed to initialize database."
    exit 1
fi
