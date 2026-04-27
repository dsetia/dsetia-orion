#!/bin/bash
# Copyright (c) 2025 SecurITe
# All rights reserved.
#
# useradm.sh — Thin wrapper around dbtool that exposes only UI user management
# operations.  All other dbtool operations are blocked to prevent accidental
# misuse.

set -euo pipefail

# ─── Configuration ────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DBTOOL="${DBTOOL:-${SCRIPT_DIR}/../bin/dbtool}"

# ─── Allowed operations ───────────────────────────────────────────────────────

ALLOWED_OPS=(
    insert-user
    list-users
    delete-user
    reset-user-password
    deactivate-user
    list-login-audit
)

# ─── Usage ────────────────────────────────────────────────────────────────────

usage() {
    cat <<EOF
Usage: $(basename "$0") -db <db-config> -op <operation> [flags]

Operations:
  insert-user          -tenant-id N -email E -role R  (prompts for password)
  list-users           -tenant-id N
  delete-user          -tenant-id N -user-id U
  reset-user-password  -tenant-id N -user-id U        (prompts for new password)
  deactivate-user      -tenant-id N -user-id U
  list-login-audit     [-tenant-id N] [-user-id U | -email E] [-limit N]

Roles: security_analyst, system_admin

Examples:
  $(basename "$0") -db config/db.json -op insert-user -tenant-id 1 -email admin@example.com -role system_admin
  $(basename "$0") -db config/db.json -op list-users -tenant-id 1
  $(basename "$0") -db config/db.json -op delete-user -tenant-id 1 -user-id <uuid>
  $(basename "$0") -db config/db.json -op reset-user-password -tenant-id 1 -user-id <uuid>
  $(basename "$0") -db config/db.json -op list-login-audit -limit 20
EOF
    exit 1
}

# ─── Validate dbtool binary ───────────────────────────────────────────────────

if [[ ! -x "$DBTOOL" ]]; then
    echo "Error: dbtool binary not found at $DBTOOL" >&2
    echo "Build it first: cd db && go build -o ../bin/dbtool ." >&2
    exit 1
fi

# ─── Parse -op from arguments ────────────────────────────────────────────────

if [[ $# -lt 1 ]]; then
    usage
fi

OP=""
for (( i=1; i<=$#; i++ )); do
    if [[ "${!i}" == "-op" ]]; then
        j=$((i+1))
        OP="${!j:-}"
        break
    fi
done

if [[ -z "$OP" ]]; then
    echo "Error: -op flag is required" >&2
    usage
fi

# ─── Allowlist check ─────────────────────────────────────────────────────────

allowed=false
for allowed_op in "${ALLOWED_OPS[@]}"; do
    if [[ "$OP" == "$allowed_op" ]]; then
        allowed=true
        break
    fi
done

if [[ "$allowed" != "true" ]]; then
    echo "Error: operation '$OP' is not permitted via useradm." >&2
    echo "Allowed operations: ${ALLOWED_OPS[*]}" >&2
    exit 1
fi

# ─── Delegate to dbtool ──────────────────────────────────────────────────────

exec "$DBTOOL" "$@"
