#!/bin/bash
#
# test_multi_environment.sh
# Comprehensive test script for multi-environment tenant system (simplified schema)
#
# Copyright (c) 2025 SecurITe
# File Owner: deepinder@securite.world
#

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Configuration
DBTOOL="./dbtool"
CONFIG_DIR="/opt/config"

print_header() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

print_test() {
    echo -e "${YELLOW}TEST: $1${NC}"
}

print_pass() {
    echo -e "${GREEN}✓ PASS: $1${NC}"
    ((TESTS_PASSED++))
}

print_fail() {
    echo -e "${RED}✗ FAIL: $1${NC}"
    ((TESTS_FAILED++))
}

print_info() {
    echo -e "${BLUE}ℹ INFO: $1${NC}"
}

# Test 1: Verify tenant_id_blocks table
test_tenant_id_blocks() {
    print_test "Verifying tenant_id_blocks table structure"
    
    if $DBTOOL -db $CONFIG_DIR/db-private-prod.json -op list-tenant-blocks > /dev/null 2>&1; then
        print_pass "tenant_id_blocks table exists and is accessible"
    else
        print_fail "tenant_id_blocks table missing or inaccessible"
        return 1
    fi
}

# Test 2: Create tenant in private-staging (ID range 1-1000)
test_private_staging_tenant() {
    print_test "Creating tenant in private-staging environment"
    
    TENANT_NAME="test-staging-$(date +%s)"
    OUTPUT=$($DBTOOL -db $CONFIG_DIR/db-private-staging.json \
        -op insert-tenant -tenant-name "$TENANT_NAME" 2>&1)
    
    TENANT_ID=$(echo "$OUTPUT" | grep -oP 'ID=\K[0-9]+')
    
    if [ -n "$TENANT_ID" ] && [ "$TENANT_ID" -ge 1 ] && [ "$TENANT_ID" -le 1000 ]; then
        print_pass "Private staging tenant created with ID=$TENANT_ID (within range 1-1000)"
    else
        print_fail "Private staging tenant ID=$TENANT_ID outside expected range 1-1000"
        return 1
    fi
}

# Test 3: Create tenant in private-prod (ID range 1001-10000)
test_private_prod_tenant() {
    print_test "Creating tenant in private-prod environment"
    
    TENANT_NAME="test-prod-$(date +%s)"
    OUTPUT=$($DBTOOL -db $CONFIG_DIR/db-private-prod.json \
        -op insert-tenant -tenant-name "$TENANT_NAME" 2>&1)
    
    TENANT_ID=$(echo "$OUTPUT" | grep -oP 'ID=\K[0-9]+')
    
    if [ -n "$TENANT_ID" ] && [ "$TENANT_ID" -ge 1001 ] && [ "$TENANT_ID" -le 10000 ]; then
        print_pass "Private prod tenant created with ID=$TENANT_ID (within range 1001-10000)"
        echo "$TENANT_ID" > /tmp/test_tenant_id.txt
    else
        print_fail "Private prod tenant ID=$TENANT_ID outside expected range 1001-10000"
        return 1
    fi
}

# Test 4: Create tenant in aws-prod (ID range 11000-20000)
test_aws_tenant() {
    print_test "Creating tenant in aws-prod environment"
    
    TENANT_NAME="test-aws-$(date +%s)"
    OUTPUT=$($DBTOOL -db $CONFIG_DIR/db-aws-prod.json \
        -op insert-tenant -tenant-name "$TENANT_NAME" 2>&1)
    
    TENANT_ID=$(echo "$OUTPUT" | grep -oP 'ID=\K[0-9]+')
    
    if [ -n "$TENANT_ID" ] && [ "$TENANT_ID" -ge 11000 ] && [ "$TENANT_ID" -le 20000 ]; then
        print_pass "AWS tenant created with ID=$TENANT_ID (within range 11000-20000)"
    else
        print_fail "AWS tenant ID=$TENANT_ID outside expected range 11000-20000"
        return 1
    fi
}

# Test 5: Verify tenant environment tagging
test_tenant_environment_tagging() {
    print_test "Verifying tenant environment field"
    
    OUTPUT=$($DBTOOL -db $CONFIG_DIR/db-private-prod.json -op list-tenants 2>&1)
    
    if echo "$OUTPUT" | grep -q "private-prod"; then
        print_pass "Tenants correctly tagged with environment"
    else
        print_fail "Tenant environment tagging not working"
        return 1
    fi
}

# Test 6: Create device for tenant
test_device_creation() {
    print_test "Creating device for tenant"
    
    if [ ! -f /tmp/test_tenant_id.txt ]; then
        print_info "Skipping device test - no tenant ID available"
        return 0
    fi
    
    TENANT_ID=$(cat /tmp/test_tenant_id.txt)
    DEVICE_NAME="test-device-$(date +%s)"
    
    OUTPUT=$($DBTOOL -db $CONFIG_DIR/db-private-prod.json \
        -op insert-device \
        -tenant-id "$TENANT_ID" \
        -device-name "$DEVICE_NAME" \
        -hndr-sw-version "v1.0.0" 2>&1)
    
    DEVICE_ID=$(echo "$OUTPUT" | grep -oP 'ID=\K[a-f0-9-]+')
    
    if [ -n "$DEVICE_ID" ]; then
        print_pass "Device created with ID=$DEVICE_ID"
        echo "$DEVICE_ID" > /tmp/test_device_id.txt
    else
        print_fail "Device creation failed"
        return 1
    fi
}

# Test 7: Create API key for device
test_api_key_creation() {
    print_test "Creating API key for device"
    
    if [ ! -f /tmp/test_tenant_id.txt ] || [ ! -f /tmp/test_device_id.txt ]; then
        print_info "Skipping API key test - no tenant/device ID available"
        return 0
    fi
    
    TENANT_ID=$(cat /tmp/test_tenant_id.txt)
    DEVICE_ID=$(cat /tmp/test_device_id.txt)
    
    OUTPUT=$($DBTOOL -db $CONFIG_DIR/db-private-prod.json \
        -op insert-api-key \
        -tenant-id "$TENANT_ID" \
        -device-id "$DEVICE_ID" 2>&1)
    
    if echo "$OUTPUT" | grep -q "API Key inserted"; then
        print_pass "API key created successfully"
    else
        print_fail "API key creation failed"
        return 1
    fi
}

# Test 8: Verify tenant ID range validation
test_tenant_id_validation() {
    print_test "Testing tenant ID range validation"
    
    # Try to create tenant with ID outside range (should fail)
    TENANT_NAME="invalid-id-test-$(date +%s)"
    
    if $DBTOOL -db $CONFIG_DIR/db-private-staging.json \
        -op insert-tenant \
        -tenant-name "$TENANT_NAME" \
        -tenant-id 5000 2>&1 | grep -q "outside"; then
        print_pass "Tenant ID validation working (rejected ID 5000 for private-staging)"
    else
        print_fail "Tenant ID validation not working properly"
        return 1
    fi
}

# Test 9: Check allocation status view
test_allocation_status() {
    print_test "Checking tenant allocation status view"
    
    # This requires direct psql access
    if command -v psql &> /dev/null; then
        OUTPUT=$(psql -h postgres -U pguser -d pgdb \
            -c "SELECT * FROM tenant_allocation_status;" 2>&1)
        
        if echo "$OUTPUT" | grep -q "private"; then
            print_pass "Allocation status view working"
        else
            print_fail "Allocation status view not accessible"
            return 1
        fi
    else
        print_info "Skipping allocation status test - psql not available"
    fi
}

# Test 10: Verify unique tenant names
test_unique_tenant_names() {
    print_test "Verifying tenant name uniqueness"
    
    TENANT_NAME="duplicate-test-$(date +%s)"
    
    # Create tenant in staging
    $DBTOOL -db $CONFIG_DIR/db-private-staging.json \
        -op insert-tenant -tenant-name "$TENANT_NAME" > /dev/null 2>&1
    
    # Try to create same name in staging again (should fail)
    if $DBTOOL -db $CONFIG_DIR/db-private-staging.json \
        -op insert-tenant -tenant-name "$TENANT_NAME" 2>&1 | grep -q "exists\|duplicate"; then
        print_pass "Tenant name uniqueness enforced"
    else
        print_fail "Duplicate tenant names allowed"
        return 1
    fi
}

# Test 11: Sequence incrementing
test_sequence_incrementing() {
    print_test "Testing sequence auto-increment"
    
    TENANT1="seq-test-1-$(date +%s)"
    TENANT2="seq-test-2-$(date +%s)"
    
    OUTPUT1=$($DBTOOL -db $CONFIG_DIR/db-private-staging.json \
        -op insert-tenant -tenant-name "$TENANT1" 2>&1)
    ID1=$(echo "$OUTPUT1" | grep -oP 'ID=\K[0-9]+')
    
    OUTPUT2=$($DBTOOL -db $CONFIG_DIR/db-private-staging.json \
        -op insert-tenant -tenant-name "$TENANT2" 2>&1)
    ID2=$(echo "$OUTPUT2" | grep -oP 'ID=\K[0-9]+')
    
    if [ -n "$ID1" ] && [ -n "$ID2" ] && [ "$ID2" -gt "$ID1" ]; then
        print_pass "Sequence auto-incrementing correctly (ID1=$ID1, ID2=$ID2)"
    else
        print_fail "Sequence not incrementing properly"
        return 1
    fi
}

# Test 12: Environment separation
test_environment_separation() {
    print_test "Testing environment separation"
    
    TENANT_NAME="env-sep-test-$(date +%s)"
    
    # Create tenant in private-staging
    OUTPUT1=$($DBTOOL -db $CONFIG_DIR/db-private-staging.json \
        -op insert-tenant -tenant-name "$TENANT_NAME-staging" 2>&1)
    ID1=$(echo "$OUTPUT1" | grep -oP 'ID=\K[0-9]+')
    
    # Create tenant with similar name in aws-prod
    OUTPUT2=$($DBTOOL -db $CONFIG_DIR/db-aws-prod.json \
        -op insert-tenant -tenant-name "$TENANT_NAME-aws" 2>&1)
    ID2=$(echo "$OUTPUT2" | grep -oP 'ID=\K[0-9]+')
    
    # IDs should be in completely different ranges
    if [ -n "$ID1" ] && [ -n "$ID2" ] && [ "$ID1" -lt 1001 ] && [ "$ID2" -gt 11000 ]; then
        print_pass "Environment separation working (staging ID=$ID1, aws ID=$ID2)"
    else
        print_fail "Environment separation not working properly"
        return 1
    fi
}

# Test 13: Foreign key cascade deletion
test_cascade_deletion() {
    print_test "Testing cascade deletion of tenant resources"
    
    if [ ! -f /tmp/test_tenant_id.txt ]; then
        print_info "Skipping cascade test - no tenant ID available"
        return 0
    fi
    
    TENANT_ID=$(cat /tmp/test_tenant_id.txt)
    
    # Delete tenant
    if $DBTOOL -db $CONFIG_DIR/db-private-prod.json \
        -op delete-tenant -tenant-id "$TENANT_ID" > /dev/null 2>&1; then
        
        # Verify devices are also deleted
        OUTPUT=$($DBTOOL -db $CONFIG_DIR/db-private-prod.json \
            -op list-devices -tenant-id "$TENANT_ID" 2>&1)
        
        if [ -z "$OUTPUT" ] || echo "$OUTPUT" | grep -q "0 rows"; then
            print_pass "Cascade deletion working (tenant and related resources deleted)"
        else
            print_fail "Cascade deletion not working properly"
            return 1
        fi
    else
        print_fail "Tenant deletion failed"
        return 1
    fi
}

# Test summary
print_summary() {
    echo ""
    print_header "Test Summary"
    echo -e "${GREEN}Tests Passed: $TESTS_PASSED${NC}"
    echo -e "${RED}Tests Failed: $TESTS_FAILED${NC}"
    echo ""
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All tests passed! ✓${NC}"
        return 0
    else
        echo -e "${RED}Some tests failed! ✗${NC}"
        return 1
    fi
}

# Cleanup
cleanup() {
    print_info "Cleaning up test artifacts..."
    rm -f /tmp/test_tenant_id.txt /tmp/test_device_id.txt
}

# Main execution
main() {
    print_header "Multi-Environment Tenant System Tests (Simplified Schema)"
    
    # Check prerequisites
    if [ ! -f "$DBTOOL" ]; then
        print_fail "dbtool not found at $DBTOOL"
        exit 1
    fi
    
    if [ ! -d "$CONFIG_DIR" ]; then
        print_fail "Config directory not found at $CONFIG_DIR"
        exit 1
    fi
    
    echo "Testing with simplified schema (single environment field)"
    echo "Environments: private-staging, private-prod, aws-prod, gcloud-prod, azure-prod"
    echo ""
    
    # Run tests
    test_tenant_id_blocks || true
    test_private_staging_tenant || true
    test_private_prod_tenant || true
    test_aws_tenant || true
    test_tenant_environment_tagging || true
    test_device_creation || true
    test_api_key_creation || true
    test_tenant_id_validation || true
    test_allocation_status || true
    test_unique_tenant_names || true
    test_sequence_incrementing || true
    test_environment_separation || true
    test_cascade_deletion || true
    
    # Print summary
    print_summary
    
    # Cleanup
    cleanup
}

# Run main
main
exit $?
