# Tenant Environment Migration Tools

## Overview

These scripts allow you to safely move tenants between different environment ID blocks in your multi-tenant PostgreSQL database (Schema V2).

## Scripts

### 1. `move-tenant.sh` - Tenant Migration Script

Moves a tenant from one environment ID block to another by:
- Allocating a new tenant ID from the target environment's sequence
- Creating a new tenant record with the new ID
- Updating all related table references (devices, api_keys, hndr_rules, status, version)
- Removing the old tenant record
- All operations are performed within a single transaction for safety

### 2. `tenant-info.sh` - Tenant Information Inspector

Displays comprehensive information about a tenant including:
- Tenant details (ID, name, environment, timestamps)
- Current environment block allocation
- Count of all related records
- List of devices and API keys
- Available target environments with capacity information

## Usage

### Inspect Tenant Before Migration

```bash
./tenant-info.sh --db /opt/config/db.json --tenant-id 7
```

**Output includes:**
- Tenant details and current environment
- Related record counts
- All devices and API keys
- Available target environments with capacity

### Dry Run (Preview Changes)

```bash
./move-tenant.sh --db /opt/config/db.json --tenant-id 7 --environment private-prod --dry-run
```

**Shows:**
- Current and target environment details
- Migration plan with all steps
- Record counts that will be affected
- New tenant ID that will be allocated
- No actual changes are made

### Execute Migration

```bash
./move-tenant.sh --db /opt/config/db.json --tenant-id 7 --environment private-prod
```

**Process:**
1. Validates all parameters and database connection
2. Displays current tenant information
3. Shows migration plan
4. Requests confirmation (type "yes" to proceed)
5. Executes migration in a transaction
6. Verifies all changes
7. Displays summary

## Command-Line Options

### move-tenant.sh

| Option | Required | Description |
|--------|----------|-------------|
| `--db <path>` | Yes | Path to database config JSON file |
| `--tenant-id <id>` | Yes | ID of the tenant to move |
| `--environment <env>` | Yes | Target environment (e.g., private-prod, aws-prod) |
| `--dry-run` | No | Preview changes without executing |

### tenant-info.sh

| Option | Required | Description |
|--------|----------|-------------|
| `--db <path>` | Yes | Path to database config JSON file |
| `--tenant-id <id>` | Yes | ID of the tenant to inspect |

## Database Configuration File

The `--db` parameter expects a JSON file with the following format:

```json
{
    "host": "postgres",
    "port": 5432,
    "user": "pguser",
    "password": "pgpass",
    "database": "pgdb"
}
```

## Available Environments (Schema V2)

| Environment | ID Range | Capacity | Description |
|-------------|----------|----------|-------------|
| private-staging | 1 - 1,000 | 1,000 | Private staging tenants |
| private-prod | 1,001 - 10,000 | 9,000 | Private production tenants |
| aws-prod | 11,000 - 20,000 | 10,000 | AWS production tenants |
| gcloud-prod | 21,000 - 30,000 | 10,000 | GCloud production tenants |
| azure-prod | 31,000 - 40,000 | 10,000 | Azure production tenants |

## Migration Process Details

### What Happens During Migration

1. **Validation Phase**
   - Verifies database connection
   - Confirms tenant exists
   - Validates target environment exists
   - Checks target environment has capacity

2. **Planning Phase**
   - Counts all related records
   - Allocates new tenant ID from target environment sequence
   - Validates new ID is within target range
   - Displays migration plan

3. **Execution Phase** (Transaction)
   ```sql
   BEGIN;
   
   -- Create new tenant record with new ID
   INSERT INTO tenants (tenant_id, tenant_name, environment, ...)
   SELECT <new_id>, tenant_name, '<target_env>', ...
   FROM tenants WHERE tenant_id = <old_id>;
   
   -- Update all related tables
   UPDATE devices SET tenant_id = <new_id> WHERE tenant_id = <old_id>;
   UPDATE api_keys SET tenant_id = <new_id> WHERE tenant_id = <old_id>;
   UPDATE hndr_rules SET tenant_id = <new_id> WHERE tenant_id = <old_id>;
   UPDATE status SET tenant_id = <new_id> WHERE tenant_id = <old_id>;
   UPDATE version SET tenant_id = <new_id> WHERE tenant_id = <old_id>;
   
   -- Remove old tenant record
   DELETE FROM tenants WHERE tenant_id = <old_id>;
   
   COMMIT;
   ```

4. **Verification Phase**
   - Confirms new tenant record exists
   - Verifies all record counts match expectations
   - Confirms old tenant record is removed

## Safety Features

### Transaction Safety
- All operations occur within a single PostgreSQL transaction
- If any step fails, entire migration is rolled back
- Database remains in consistent state even if migration fails

### Validation Checks
- ✓ Database connection validated before starting
- ✓ Tenant existence verified
- ✓ Target environment validated
- ✓ New tenant ID verified within correct range
- ✓ User confirmation required before execution

### Dry Run Mode
- Test migration without making changes
- See exactly what will happen
- Verify new tenant ID allocation
- Review all affected records

### Post-Migration Verification
- Confirms new tenant record created
- Verifies all related records updated correctly
- Ensures old tenant record removed
- Displays record count comparison

## Example Workflow

### Complete Migration Example

```bash
# Step 1: Inspect current tenant state
./tenant-info.sh --db /opt/config/db.json --tenant-id 7

# Output shows:
#   Tenant ID: 7
#   Name: test-tenant
#   Environment: private-staging
#   Devices: 1
#   API Keys: 1

# Step 2: Preview migration (dry run)
./move-tenant.sh --db /opt/config/db.json --tenant-id 7 --environment private-prod --dry-run

# Output shows:
#   Current: ID=7, Env=private-staging
#   Target: Env=private-prod (1001-10000)
#   New Tenant ID: 1001
#   Will update: 1 device, 1 API key

# Step 3: Execute migration
./move-tenant.sh --db /opt/config/db.json --tenant-id 7 --environment private-prod

# Prompts for confirmation:
#   Do you want to proceed? (yes/no): yes

# Output shows:
#   ✓ Migration completed successfully
#   ✓ New tenant record verified
#   ✓ All record counts verified
#   ✓ Old tenant record removed
#   Tenant Name: test-tenant
#   Old Tenant ID: 7 (private-staging)
#   New Tenant ID: 1001 (private-prod)

# Step 4: Verify migration
./tenant-info.sh --db /opt/config/db.json --tenant-id 1001

# Shows tenant now in private-prod with ID 1001
```

## Tables Affected by Migration

The migration updates all tables with foreign key references to `tenant_id`:

1. **tenants** - Primary tenant record
2. **devices** - Device registrations
3. **api_keys** - API authentication keys
4. **hndr_rules** - Handler rules configurations
5. **status** - Device status records
6. **version** - Device version tracking

## Important Considerations

### Tenant ID Changes
- The tenant ID will change to a value within the target environment's range
- Applications must use `tenant_name` for stable references, not `tenant_id`
- API keys and device IDs remain unchanged

### Sequence State
- The target environment's sequence is incremented
- The source environment's sequence is not decremented
- This creates gaps in the source environment's ID space

### Environment Capacity
- Each environment has a fixed capacity (e.g., 1,000 or 10,000 tenants)
- Migration fails if target environment is at capacity
- Check available capacity using `tenant-info.sh`

### Concurrent Operations
- Migration locks the tenant record during transaction
- Other operations on the same tenant will wait
- Keep migration window short for production systems

### Rollback
- Automatic rollback on failure (transaction-based)
- Manual rollback not supported after successful migration
- To reverse, run migration back to original environment (allocates new ID)

## Error Handling

### Common Errors

**Tenant not found:**
```
Error: Tenant ID 999 not found
```
→ Verify tenant ID exists using `tenant-info.sh`

**Target environment not found:**
```
Error: Target environment 'invalid-env' not found
```
→ Check available environments in tenant_id_blocks table

**Environment at capacity:**
```
Error: Failed to allocate new tenant ID
```
→ Target environment sequence exhausted, use different environment

**Database connection failed:**
```
Error: Cannot connect to database
```
→ Verify database config file and network connectivity

## Testing Recommendations

### Before Production Use

1. **Test in staging environment first**
   ```bash
   # Test migration in staging
   ./move-tenant.sh --db /opt/config/staging-db.json --tenant-id 1 \
       --environment private-staging --dry-run
   ```

2. **Verify with small tenant**
   - Start with tenant having few devices and API keys
   - Confirm all records migrate correctly
   - Test application functionality after migration

3. **Monitor sequence usage**
   ```sql
   -- Check sequence current value
   SELECT last_value FROM seq_private_prod_tenant_id;
   
   -- Check remaining capacity
   SELECT * FROM tenant_allocation_status;
   ```

### Integration Testing

- Test with tenants having no related records
- Test with tenants having many devices and API keys
- Test migration between all environment pairs
- Verify foreign key constraints remain intact
- Confirm indexes still work correctly

## Troubleshooting

### Migration Hangs
- Check for locks on tenant record
- Verify PostgreSQL connection pool capacity
- Review active transactions: `SELECT * FROM pg_stat_activity;`

### Record Count Mismatch
- Review migration transaction logs
- Check for concurrent operations during migration
- Verify foreign key constraints are intact

### Cannot Allocate New ID
- Check sequence current value vs. max value
- Review tenant_allocation_status view
- May need to extend environment ID range

## Performance Considerations

- Migration speed depends on number of related records
- Typical migration (10 devices, 20 API keys): < 1 second
- Large tenant (1000+ devices): 2-5 seconds
- All operations in single transaction ensure atomicity

## Additional Tools Integration

These scripts integrate with your existing `dbtool` commands:

```bash
# List all tenants
dbtool -db /opt/config/db.json -op list-tenants

# List devices for migrated tenant (use new ID)
dbtool -db /opt/config/db.json -op list-devices

# API keys automatically reference new tenant ID
dbtool -db /opt/config/db.json -op list-api-keys
```

## Schema Version Compatibility

- **Requires:** Schema V2 with tenant_id_blocks table
- **Not compatible with:** Schema V1 (single environment)
- Scripts auto-detect schema version not implemented (assume V2)

## Support

For issues or questions:
1. Check error messages in script output
2. Review PostgreSQL logs for transaction details
3. Use `--dry-run` to diagnose issues without making changes
4. Verify tenant exists and target environment is valid
