#!/bin/bash
# test_ui_auth.sh — Manual end-to-end REST tests for the UI auth API.
#
# Usage:
#   ./utils/test_ui_auth.sh [-v] [-c DB_CONFIG] [BASE_URL]
#
#   -v              Verbose: print the JSON response body after every request.
#   -c DB_CONFIG    Path to db.json config file (default: config/db.json).
#                   Required to create/delete the admin user via dbtool.
#
# Prerequisites:
#   - apis server running at BASE_URL
#   - dbtool binary on PATH (or in ../bin/)
#   - jq is installed
#   - TENANT_ID set to the tenant the admin user will be created under

set -euo pipefail

VERBOSE=0
DB_CONFIG="config/db.json"

while [[ $# -gt 0 ]]; do
    case "${1:-}" in
        -v) VERBOSE=1; shift ;;
        -c) DB_CONFIG="$2"; shift 2 ;;
        *)  break ;;
    esac
done

BASE_URL="${1:-http://localhost:8080}"
TENANT_ID="${TENANT_ID:-1}"

# ─── Credentials (override via env) ──────────────────────────────────────────
ADMIN_EMAIL="${ADMIN_EMAIL:-test-admin-$$@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-adminpassword123}"

ANALYST_EMAIL="test-analyst-$$@example.com"
ANALYST_PASSWORD="analystpassword123"

# ─── Colour / counters ───────────────────────────────────────────────────────
GREEN="\033[32m"; RED="\033[31m"; YELLOW="\033[33m"; DIM="\033[2m"; RESET="\033[0m"
PASS=0; FAIL=0

pass()    { echo -e "  ${GREEN}PASS${RESET}  $1"; (( PASS++ )) || true; }
fail()    { echo -e "  ${RED}FAIL${RESET}  $1"; (( FAIL++ )) || true; }
section() { echo -e "\n${YELLOW}── $1 ──${RESET}"; }

expect_status() {
    local label="$1" expected="$2"
    if [[ "$STATUS" == "$expected" ]]; then
        pass "$label (HTTP $STATUS)"
    else
        fail "$label — expected HTTP $expected, got $STATUS"
    fi
}

# ─── dbtool helper ───────────────────────────────────────────────────────────
# Prefer ../bin/dbtool (relative to script location) then fall back to PATH.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DBTOOL="${SCRIPT_DIR}/../bin/dbtool"
if [[ ! -x "$DBTOOL" ]]; then
    DBTOOL="dbtool"
fi

# ─── Request helper ───────────────────────────────────────────────────────────
# Sets globals: STATUS (HTTP code), BODY (response text).
# Called without $() so the globals survive in the parent shell.
BODY=""; STATUS=""
do_curl() {
    local method="$1" path="$2"; shift 2
    local raw
    raw=$(curl -s -w "\n__STATUS__%{http_code}" -X "$method" \
        "${BASE_URL}${path}" "$@")
    STATUS="${raw##*__STATUS__}"
    BODY="${raw%$'\n'__STATUS__*}"
    if [[ "$VERBOSE" -eq 1 ]]; then
        echo -e "  ${DIM}${method} ${path}  →  HTTP ${STATUS}${RESET}"
        if [[ -n "$BODY" ]]; then
            echo "$BODY" | jq . 2>/dev/null || echo "$BODY"
        fi
    fi
}

# ─── State ───────────────────────────────────────────────────────────────────
ADMIN_ACCESS=""; ADMIN_REFRESH=""; ADMIN_USER_ID=""
ANALYST_ACCESS=""; ANALYST_USER_ID=""
TENANT_CREATED=0

# =============================================================================
section "Setup — create tenant via dbtool"
# =============================================================================

TENANT_NAME="test-tenant-$$"
echo -e "  ${DIM}tenant_name=$TENANT_NAME  tenant_id=$TENANT_ID${RESET}"

tenant_out=""
tenant_rc=0
tenant_out=$("$DBTOOL" -db "$DB_CONFIG" -op insert-tenant \
    -tenant-name "$TENANT_NAME" \
    -tenant-id "$TENANT_ID" 2>&1) || tenant_rc=$?

if [[ $tenant_rc -ne 0 ]]; then
    echo -e "${RED}dbtool insert-tenant failed (exit $tenant_rc):${RESET}"
    echo "$tenant_out"
    exit 1
fi

TENANT_CREATED=1
pass "Tenant created (tenant_id=$TENANT_ID)"

# =============================================================================
section "Setup — create admin user via dbtool"
# =============================================================================

echo -e "  ${DIM}tenant_id=$TENANT_ID  email=$ADMIN_EMAIL${RESET}"

dbtool_out=""
dbtool_rc=0
dbtool_out=$(echo "$ADMIN_PASSWORD" | \
    "$DBTOOL" -db "$DB_CONFIG" -op insert-user \
        -tenant-id "$TENANT_ID" \
        -email "$ADMIN_EMAIL" \
        -role system_admin 2>&1) || dbtool_rc=$?

if [[ $dbtool_rc -ne 0 ]]; then
    echo -e "${RED}dbtool insert-user failed (exit $dbtool_rc):${RESET}"
    echo "$dbtool_out"
    exit 1
fi

ADMIN_USER_ID=$(echo "$dbtool_out" | sed -n 's/.*user_id=\([^ ]*\).*/\1/p')

if [[ -n "$ADMIN_USER_ID" ]]; then
    pass "Admin user created (user_id=$ADMIN_USER_ID)"
else
    echo -e "${RED}Failed to parse user_id from dbtool output:${RESET}"
    echo "$dbtool_out"
    exit 1
fi

# =============================================================================
section "Login"
# =============================================================================

do_curl POST /v1/ma/auth/login \
    -H "Content-Type: application/json" \
    -d '{"email":"bad@example.com","password":"wrongpassword"}'
expect_status "Unknown email returns 401" 401

do_curl POST /v1/ma/auth/login \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"wrongpassword\"}"
expect_status "Wrong password returns 401" 401

do_curl POST /v1/ma/auth/login \
    -H "Content-Type: application/json" \
    -d '{"email":"","password":""}'
expect_status "Empty credentials returns 400" 400

do_curl POST /v1/ma/auth/login \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}"
expect_status "Valid admin login returns 200" 200

ADMIN_EMAIL_UPPER=$(echo "$ADMIN_EMAIL" | tr '[:lower:]' '[:upper:]')
do_curl POST /v1/ma/auth/login \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$ADMIN_EMAIL_UPPER\",\"password\":\"$ADMIN_PASSWORD\"}"
expect_status "Login is case-insensitive for email" 200

if [[ "$STATUS" == "200" ]]; then
    ADMIN_ACCESS=$(echo "$BODY"  | jq -r '.access_token')
    ADMIN_REFRESH=$(echo "$BODY" | jq -r '.refresh_token')
    token_type=$(echo "$BODY"    | jq -r '.token_type')
    [[ -n "$ADMIN_ACCESS"  && "$ADMIN_ACCESS"  != "null" ]] && pass "access_token present"  || fail "access_token missing"
    [[ -n "$ADMIN_REFRESH" && "$ADMIN_REFRESH" != "null" ]] && pass "refresh_token present" || fail "refresh_token missing"
    [[ "$token_type" == "Bearer" ]] && pass "token_type is Bearer" || fail "token_type expected Bearer, got $token_type"
else
    fail "Admin login failed — cannot continue"
    echo -e "\n${RED}Check that the server is running at $BASE_URL.${RESET}"
    exit 1
fi

# =============================================================================
section "/me"
# =============================================================================

do_curl GET /v1/ma/me
expect_status "No token returns 401" 401

do_curl GET /v1/ma/me -H "Authorization: Bearer badtoken"
expect_status "Invalid token returns 401" 401

do_curl GET /v1/ma/me -H "Authorization: Bearer $ADMIN_ACCESS"
expect_status "Valid token returns 200" 200
if [[ "$STATUS" == "200" ]]; then
    me_email=$(echo "$BODY"       | jq -r '.email')
    me_role=$(echo "$BODY"        | jq -r '.role')
    me_user_id=$(echo "$BODY"     | jq -r '.user_id')
    me_tenant_name=$(echo "$BODY" | jq -r '.tenant_name')
    [[ "$me_email"       == "$ADMIN_EMAIL"   ]] && pass "/me email matches"              || fail "/me email mismatch (got $me_email)"
    [[ "$me_role"        == "system_admin"   ]] && pass "/me role is system_admin"       || fail "/me role mismatch (got $me_role)"
    [[ "$me_user_id"     == "$ADMIN_USER_ID" ]] && pass "/me user_id matches dbtool"     || fail "/me user_id mismatch (got $me_user_id)"
    [[ -n "$me_tenant_name" && "$me_tenant_name" != "null" ]] && pass "/me tenant_name present" || fail "/me tenant_name missing"
fi

# =============================================================================
section "Refresh"
# =============================================================================

do_curl POST /v1/ma/auth/refresh \
    -H "Content-Type: application/json" \
    -d '{"refresh_token":"garbage"}'
expect_status "Garbage refresh token returns 401" 401

do_curl POST /v1/ma/auth/refresh \
    -H "Content-Type: application/json" \
    -d "{\"refresh_token\":\"$ADMIN_REFRESH\"}"
expect_status "Valid refresh token returns 200" 200
if [[ "$STATUS" == "200" ]]; then
    new_access=$(echo "$BODY" | jq -r '.access_token')
    [[ -n "$new_access" && "$new_access" != "null" ]] \
        && pass "Refresh returns new access_token" \
        || fail "Refresh missing access_token"
    ADMIN_ACCESS="$new_access"
fi

# =============================================================================
section "Users — list"
# =============================================================================

do_curl GET /v1/ma/users -H "Authorization: Bearer $ADMIN_ACCESS"
expect_status "system_admin can list users" 200

# =============================================================================
section "Users — create"
# =============================================================================

do_curl POST /v1/ma/users \
    -H "Authorization: Bearer $ADMIN_ACCESS" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$ANALYST_EMAIL\",\"password\":\"$ANALYST_PASSWORD\",\"role\":\"security_analyst\"}"
expect_status "system_admin creates analyst user" 201
if [[ "$STATUS" == "201" ]]; then
    ANALYST_USER_ID=$(echo "$BODY" | jq -r '.user_id')
    [[ -n "$ANALYST_USER_ID" && "$ANALYST_USER_ID" != "null" ]] \
        && pass "Create response contains user_id" \
        || fail "Create response missing user_id"
fi

do_curl POST /v1/ma/users \
    -H "Authorization: Bearer $ADMIN_ACCESS" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$ANALYST_EMAIL\",\"password\":\"$ANALYST_PASSWORD\",\"role\":\"security_analyst\"}"
expect_status "Duplicate email returns 409" 409

ANALYST_EMAIL_UPPER=$(echo "$ANALYST_EMAIL" | tr '[:lower:]' '[:upper:]')
do_curl POST /v1/ma/users \
    -H "Authorization: Bearer $ADMIN_ACCESS" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$ANALYST_EMAIL_UPPER\",\"password\":\"$ANALYST_PASSWORD\",\"role\":\"security_analyst\"}"
expect_status "Duplicate email (different case) returns 409" 409

do_curl POST /v1/ma/users \
    -H "Authorization: Bearer $ADMIN_ACCESS" \
    -H "Content-Type: application/json" \
    -d '{"email":"x@example.com","password":"short","role":"security_analyst"}'
expect_status "Short password returns 400" 400

do_curl POST /v1/ma/users \
    -H "Authorization: Bearer $ADMIN_ACCESS" \
    -H "Content-Type: application/json" \
    -d '{"email":"x@example.com","password":"validpassword123","role":"superuser"}'
expect_status "Invalid role returns 400" 400

# =============================================================================
section "Users — analyst role restrictions"
# =============================================================================

do_curl POST /v1/ma/auth/login \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$ANALYST_EMAIL\",\"password\":\"$ANALYST_PASSWORD\"}"
expect_status "Analyst login returns 200" 200
[[ "$STATUS" == "200" ]] && ANALYST_ACCESS=$(echo "$BODY" | jq -r '.access_token')

do_curl GET /v1/ma/users -H "Authorization: Bearer $ANALYST_ACCESS"
expect_status "security_analyst cannot list users (403)" 403

do_curl POST /v1/ma/users \
    -H "Authorization: Bearer $ANALYST_ACCESS" \
    -H "Content-Type: application/json" \
    -d '{"email":"other@example.com","password":"validpassword123","role":"security_analyst"}'
expect_status "security_analyst cannot create user (403)" 403

do_curl DELETE /v1/ma/users/"$ANALYST_USER_ID" \
    -H "Authorization: Bearer $ANALYST_ACCESS"
expect_status "security_analyst cannot delete user (403)" 403

# =============================================================================
section "Users — password reset"
# =============================================================================

do_curl PUT /v1/ma/users/"$ANALYST_USER_ID"/password \
    -H "Authorization: Bearer $ANALYST_ACCESS" \
    -H "Content-Type: application/json" \
    -d '{"password":"newanalystpassword123"}'
expect_status "Analyst can reset own password" 204

do_curl PUT /v1/ma/users/"$ADMIN_USER_ID"/password \
    -H "Authorization: Bearer $ANALYST_ACCESS" \
    -H "Content-Type: application/json" \
    -d '{"password":"newpassword123"}'
expect_status "Analyst cannot reset another user password (403)" 403

do_curl PUT /v1/ma/users/"$ANALYST_USER_ID"/password \
    -H "Authorization: Bearer $ADMIN_ACCESS" \
    -H "Content-Type: application/json" \
    -d '{"password":"adminresetpassword123"}'
expect_status "Admin can reset any user password" 204

do_curl PUT /v1/ma/users/"$ANALYST_USER_ID"/password \
    -H "Authorization: Bearer $ADMIN_ACCESS" \
    -H "Content-Type: application/json" \
    -d '{"password":"short"}'
expect_status "Short new password rejected (400)" 400

# =============================================================================
section "Devices / Versions / Status"
# =============================================================================

do_curl GET /v1/ma/devices   -H "Authorization: Bearer $ADMIN_ACCESS"
expect_status "List devices returns 200" 200

do_curl GET /v1/ma/versions  -H "Authorization: Bearer $ADMIN_ACCESS"
expect_status "List versions returns 200" 200

do_curl GET /v1/ma/status    -H "Authorization: Bearer $ADMIN_ACCESS"
expect_status "List status returns 200" 200

do_curl GET /v1/ma/nosuchresource -H "Authorization: Bearer $ADMIN_ACCESS"
expect_status "Unknown resource returns 404" 404

# =============================================================================
section "Logout"
# =============================================================================

do_curl POST /v1/ma/auth/logout -H "Authorization: Bearer $ADMIN_ACCESS"
expect_status "Logout returns 204" 204

do_curl POST /v1/ma/auth/refresh \
    -H "Content-Type: application/json" \
    -d "{\"refresh_token\":\"$ADMIN_REFRESH\"}"
expect_status "Refresh after logout returns 401" 401

echo "  NOTE  Access token remains valid until TTL expires (JWT is stateless)"

# =============================================================================
section "Users — REST delete (analyst cleanup)"
# =============================================================================

# Re-login since the admin's refresh token was revoked by logout above.
do_curl POST /v1/ma/auth/login \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}"
expect_status "Admin re-login after logout" 200
[[ "$STATUS" == "200" ]] && ADMIN_ACCESS=$(echo "$BODY" | jq -r '.access_token')

do_curl DELETE /v1/ma/users/"$ADMIN_USER_ID" \
    -H "Authorization: Bearer $ADMIN_ACCESS"
expect_status "Self-delete returns 400" 400

if [[ -n "$ANALYST_USER_ID" && "$ANALYST_USER_ID" != "null" ]]; then
    do_curl DELETE /v1/ma/users/"$ANALYST_USER_ID" \
        -H "Authorization: Bearer $ADMIN_ACCESS"
    expect_status "Admin deletes test analyst user" 204

    do_curl DELETE /v1/ma/users/"$ANALYST_USER_ID" \
        -H "Authorization: Bearer $ADMIN_ACCESS"
    expect_status "Deleting already-deleted user returns 404" 404
fi

# =============================================================================
section "Teardown — delete admin user via dbtool"
# =============================================================================

if [[ -n "$ADMIN_USER_ID" ]]; then
    teardown_out=""
    teardown_rc=0
    teardown_out=$("$DBTOOL" -db "$DB_CONFIG" -op delete-user \
        -user-id "$ADMIN_USER_ID" \
        -tenant-id "$TENANT_ID" 2>&1) || teardown_rc=$?
    if [[ $teardown_rc -eq 0 ]]; then
        pass "Admin user deleted via dbtool"
    else
        fail "Admin user deletion via dbtool failed (exit $teardown_rc): $teardown_out"
    fi
fi

if [[ "$TENANT_CREATED" -eq 1 ]]; then
    tenant_del_out=""
    tenant_del_rc=0
    tenant_del_out=$("$DBTOOL" -db "$DB_CONFIG" -op delete-tenant \
        -tenant-id "$TENANT_ID" 2>&1) || tenant_del_rc=$?
    if [[ $tenant_del_rc -eq 0 ]]; then
        pass "Tenant deleted via dbtool (tenant_id=$TENANT_ID)"
    else
        fail "Tenant deletion via dbtool failed (exit $tenant_del_rc): $tenant_del_out"
    fi
fi

# =============================================================================
echo -e "\n────────────────────────────────────────"
echo -e "Results: ${GREEN}${PASS} passed${RESET}  ${RED}${FAIL} failed${RESET}"
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
