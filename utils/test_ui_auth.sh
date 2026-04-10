#!/bin/bash
# test_ui_auth.sh — Manual end-to-end REST tests for the UI auth API.
#
# Usage:
#   ./utils/test_ui_auth.sh [-v] [BASE_URL]
#
#   -v   Verbose: print the JSON response body after every request.
#
# Prerequisites:
#   - apis server running at BASE_URL
#   - A system_admin user exists (ADMIN_EMAIL / ADMIN_PASSWORD)
#   - jq is installed

set -euo pipefail

VERBOSE=0
if [[ "${1:-}" == "-v" ]]; then
    VERBOSE=1
    shift
fi

BASE_URL="${1:-http://localhost:8080}"

# ─── Credentials (override via env) ──────────────────────────────────────────
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
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

if [[ "$STATUS" == "200" ]]; then
    ADMIN_ACCESS=$(echo "$BODY"  | jq -r '.access_token')
    ADMIN_REFRESH=$(echo "$BODY" | jq -r '.refresh_token')
    token_type=$(echo "$BODY"    | jq -r '.token_type')
    [[ -n "$ADMIN_ACCESS"  && "$ADMIN_ACCESS"  != "null" ]] && pass "access_token present"  || fail "access_token missing"
    [[ -n "$ADMIN_REFRESH" && "$ADMIN_REFRESH" != "null" ]] && pass "refresh_token present" || fail "refresh_token missing"
    [[ "$token_type" == "Bearer" ]] && pass "token_type is Bearer" || fail "token_type expected Bearer, got $token_type"
else
    fail "Admin login failed — cannot continue"
    echo -e "\n${RED}Check ADMIN_EMAIL / ADMIN_PASSWORD and that the server is running.${RESET}"
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
    me_email=$(echo "$BODY" | jq -r '.email')
    me_role=$(echo "$BODY"  | jq -r '.role')
    ADMIN_USER_ID=$(echo "$BODY" | jq -r '.user_id')
    [[ "$me_email" == "$ADMIN_EMAIL" ]] && pass "/me email matches"        || fail "/me email mismatch (got $me_email)"
    [[ "$me_role"  == "system_admin" ]] && pass "/me role is system_admin" || fail "/me role mismatch (got $me_role)"
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
section "Users — delete (cleanup)"
# =============================================================================

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
echo -e "\n────────────────────────────────────────"
echo -e "Results: ${GREEN}${PASS} passed${RESET}  ${RED}${FAIL} failed${RESET}"
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
