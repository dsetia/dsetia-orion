# Tenant Migration Quick Reference

## Common Commands

### Check Tenant Information
```bash
./tenant-info.sh --db /opt/config/db.json --tenant-id 7
```

### Dry Run (See What Would Happen)
```bash
./move-tenant.sh --db /opt/config/db.json --tenant-id 7 --environment private-prod --dry-run
```

### Execute Migration
```bash
./move-tenant.sh --db /opt/config/db.json --tenant-id 7 --environment private-prod
```

## Environment ID Ranges

| Environment      | Range          | Capacity |
|-----------------|----------------|----------|
| private-staging | 1 - 1,000      | 1,000    |
| private-prod    | 1,001 - 10,000 | 9,000    |
| aws-prod        | 11,000 - 20,000| 10,000   |
| gcloud-prod     | 21,000 - 30,000| 10,000   |
| azure-prod      | 31,000 - 40,000| 10,000   |

## Pre-Migration Checklist

- [ ] Backup database
- [ ] Run `tenant-info.sh` to check current state
- [ ] Run migration with `--dry-run` flag
- [ ] Verify new tenant ID is in correct range
- [ ] Check target environment has capacity
- [ ] Test in non-production environment first

## Example: Move Tenant from Staging to Production

```bash
# 1. Check current state
./tenant-info.sh --db /opt/config/db.json --tenant-id 7

# Output:
#   Tenant ID: 7
#   Name: test-tenant
#   Environment: private-staging
#   Devices: 1, API Keys: 1

# 2. Preview migration
./move-tenant.sh --db /opt/config/db.json --tenant-id 7 --environment private-prod --dry-run

# Output:
#   Current: ID=7, Env=private-staging
#   Target: Env=private-prod
#   New ID: 1001 (automatically allocated)
#   Changes: 1 device, 1 API key

# 3. Execute (prompts for 'yes' confirmation)
./move-tenant.sh --db /opt/config/db.json --tenant-id 7 --environment private-prod

# 4. Verify using dbtool
dbtool -db /opt/config/db.json -op list-tenants
# Shows: ID=1001, Name=test-tenant, Env=private-prod
```

## What Changes

### Changes:
- ✓ Tenant ID (7 → 1001)
- ✓ Environment (private-staging → private-prod)
- ✓ All foreign key references updated

### Does NOT Change:
- ✗ Tenant name (stays "test-tenant")
- ✗ Device IDs (stay the same)
- ✗ API keys (stay the same)
- ✗ Created/Updated timestamps (preserved)

## Safety Features

| Feature | Description |
|---------|-------------|
| Transaction | All changes in single atomic transaction |
| Rollback | Auto-rollback on any error |
| Dry Run | Preview without changes |
| Validation | Pre-checks before execution |
| Verification | Post-migration record counts verified |
| Confirmation | Requires typing "yes" to proceed |

## Troubleshooting

**"Tenant ID X not found"**
→ Check tenant exists: `./tenant-info.sh --db ... --tenant-id X`

**"Target environment not found"**
→ Use valid environment: private-prod, aws-prod, gcloud-prod, azure-prod

**"Failed to allocate new tenant ID"**
→ Environment at capacity, use different environment

**"Migration failed"**
→ Check PostgreSQL logs, transaction auto-rolled back

## Files Required

1. `move-tenant.sh` - Main migration script
2. `tenant-info.sh` - Information inspector
3. `/opt/config/db.json` - Database configuration

## Database Config Format

```json
{
    "host": "postgres",
    "port": 5432,
    "user": "pguser",
    "password": "pgpass",
    "database": "pgdb"
}
```

## Integration with Existing Tools

```bash
# Before migration: tenant ID = 7
dbtool -db /opt/config/db.json -op list-devices
# Shows: TenantID=7, Name=test-device

# After migration: tenant ID = 1001
dbtool -db /opt/config/db.json -op list-devices
# Shows: TenantID=1001, Name=test-device (same device, new tenant ID)
```

## When to Use

**Use Case: Move staging tenant to production**
```bash
./move-tenant.sh --db /opt/config/db.json --tenant-id 7 --environment private-prod
```

**Use Case: Migrate tenant to AWS cloud**
```bash
./move-tenant.sh --db /opt/config/db.json --tenant-id 1005 --environment aws-prod
```

**Use Case: Consolidate tenants in specific environment**
```bash
# Move multiple tenants to same environment
for tid in 5 6 7; do
    ./move-tenant.sh --db /opt/config/db.json --tenant-id $tid --environment private-prod
done
```

## Time Estimates

| Tenant Size | Migration Time |
|-------------|----------------|
| No devices | < 0.5 seconds |
| 1-10 devices | < 1 second |
| 100 devices | 1-2 seconds |
| 1000+ devices | 2-5 seconds |

## Important Notes

⚠️ **Tenant ID Changes**: Applications should use `tenant_name` for references, not `tenant_id`

⚠️ **Irreversible**: To reverse, migrate back (allocates new ID again)

⚠️ **Production**: Always test in staging first

⚠️ **Capacity**: Check target environment has space before migration
