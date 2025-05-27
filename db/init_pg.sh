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

DB_NAME=$1
SCHEMA_FILE=$2

if [ -z "$DB_NAME" ] || [ -z "$SCHEMA_FILE" ]; then
  echo "Usage: $0 <db_name> <schema_file.sql>"
  exit 1
fi

psql -h localhost -U pguser -d "$DB_NAME" -f "$SCHEMA_FILE"
