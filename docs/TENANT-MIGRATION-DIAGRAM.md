# Tenant Migration Process Diagram

## Before Migration

```
┌─────────────────────────────────────┐
│  private-staging (Range: 1-1000)    │
│                                     │
│  Tenant ID: 7                       │
│  Name: test-tenant                  │
│  Environment: private-staging       │
│                                     │
│  ├─ devices (1 record)              │
│  │  └─ device_id: 941326a0...       │
│  │     tenant_id: 7                 │
│  │                                  │
│  ├─ api_keys (1 record)             │
│  │  └─ api_key: e5b21e93...         │
│  │     tenant_id: 7                 │
│  │     device_id: 941326a0...       │
│  │                                  │
│  └─ hndr_rules, status, version...  │
│     (all reference tenant_id: 7)    │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│  private-prod (Range: 1001-10000)   │
│                                     │
│  (Empty - ready for migration)      │
│                                     │
│  Next available ID: 1001            │
└─────────────────────────────────────┘
```

## Migration Steps

```
Step 1: Allocate New ID
┌──────────────────────────────────────┐
│ SELECT get_next_tenant_id(          │
│     'private-prod'                   │
│ )                                    │
│                                      │
│ Returns: 1001                        │
└──────────────────────────────────────┘

Step 2: Create New Tenant Record
┌──────────────────────────────────────┐
│ INSERT INTO tenants                  │
│   (tenant_id, tenant_name,           │
│    environment)                      │
│ VALUES                               │
│   (1001, 'test-tenant',              │
│    'private-prod')                   │
└──────────────────────────────────────┘

Step 3: Update All Child Tables
┌──────────────────────────────────────┐
│ UPDATE devices                       │
│   SET tenant_id = 1001               │
│   WHERE tenant_id = 7                │
│                                      │
│ UPDATE api_keys                      │
│   SET tenant_id = 1001               │
│   WHERE tenant_id = 7                │
│                                      │
│ UPDATE hndr_rules, status, version.. │
│   SET tenant_id = 1001               │
│   WHERE tenant_id = 7                │
└──────────────────────────────────────┘

Step 4: Remove Old Tenant
┌──────────────────────────────────────┐
│ DELETE FROM tenants                  │
│   WHERE tenant_id = 7                │
└──────────────────────────────────────┘
```

## After Migration

```
┌─────────────────────────────────────┐
│  private-staging (Range: 1-1000)    │
│                                     │
│  Tenant ID: 7                       │
│  Status: DELETED ✗                  │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│  private-prod (Range: 1001-10000)   │
│                                     │
│  Tenant ID: 1001 ✓                  │
│  Name: test-tenant                  │
│  Environment: private-prod          │
│                                     │
│  ├─ devices (1 record)              │
│  │  └─ device_id: 941326a0...       │
│  │     tenant_id: 1001 ✓            │
│  │                                  │
│  ├─ api_keys (1 record)             │
│  │  └─ api_key: e5b21e93...         │
│  │     tenant_id: 1001 ✓            │
│  │     device_id: 941326a0...       │
│  │                                  │
│  └─ hndr_rules, status, version...  │
│     (all reference tenant_id: 1001) │
└─────────────────────────────────────┘
```

## What Changed

```
┌────────────────┬─────────────┬─────────────┐
│    Field       │   Before    │    After    │
├────────────────┼─────────────┼─────────────┤
│ Tenant ID      │      7      │    1001     │
│ Tenant Name    │ test-tenant │ test-tenant │
│ Environment    │ priv-stag   │ priv-prod   │
│ Device IDs     │ 941326a0... │ 941326a0... │
│ API Keys       │ e5b21e93... │ e5b21e93... │
│ FK References  │ Point to 7  │ Point to 1001│
└────────────────┴─────────────┴─────────────┘

Changes: ✓ Tenant ID, ✓ Environment, ✓ FK References
Preserved: ✓ Name, ✓ Device IDs, ✓ API Keys, ✓ Timestamps
```

## Transaction Safety

```
┌─────────────────────────────────────────┐
│              BEGIN;                     │
│  ┌────────────────────────────────┐    │
│  │  All migration steps execute   │    │
│  │  atomically within transaction │    │
│  │                                 │    │
│  │  If ANY step fails:             │    │
│  │    → ROLLBACK                   │    │
│  │    → Database unchanged         │    │
│  │                                 │    │
│  │  If ALL steps succeed:          │    │
│  │    → COMMIT                     │    │
│  │    → Changes permanent          │    │
│  └────────────────────────────────┘    │
│              END;                       │
└─────────────────────────────────────────┘
```

## Environment ID Allocation

```
┌─────────────────────────────────────────────────────┐
│ Environment      │ ID Range        │ Next Available  │
├──────────────────┼─────────────────┼────────────────┤
│ private-staging  │ 1 - 1,000       │ 22             │
│ private-prod     │ 1,001 - 10,000  │ 1,002 (after)  │
│ aws-prod         │ 11,000 - 20,000 │ 11,000         │
│ gcloud-prod      │ 21,000 - 30,000 │ 21,000         │
│ azure-prod       │ 31,000 - 40,000 │ 31,000         │
└─────────────────────────────────────────────────────┘

Each environment uses PostgreSQL sequences:
- seq_private_staging_tenant_id (1-1000)
- seq_private_prod_tenant_id (1001-10000)
- seq_aws_prod_tenant_id (11000-20000)
- etc.
```

## Validation Checks

```
Pre-Migration Checks:
├─ ✓ Database connection successful?
├─ ✓ Tenant exists?
├─ ✓ Target environment valid?
├─ ✓ Target environment has capacity?
├─ ✓ Can allocate new tenant ID?
└─ ✓ New ID within target range?

Post-Migration Verification:
├─ ✓ New tenant record created?
├─ ✓ All device records updated?
├─ ✓ All API key records updated?
├─ ✓ All rule records updated?
├─ ✓ All status records updated?
├─ ✓ All version records updated?
├─ ✓ Old tenant record deleted?
└─ ✓ Record counts match?
```

## Script Flow

```
┌─────────────────────────────────────────┐
│  ./move-tenant.sh                       │
│    --db /opt/config/db.json             │
│    --tenant-id 7                        │
│    --environment private-prod           │
│    [--dry-run]                          │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│  Parse & Validate Arguments             │
│  - Check required parameters            │
│  - Validate DB config file exists       │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│  Connect to Database                    │
│  - Parse DB config (JSON)               │
│  - Test connection                      │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│  Fetch Tenant Information               │
│  - Get current tenant record            │
│  - Get current environment block        │
│  - Count related records                │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│  Validate Target Environment            │
│  - Check environment exists             │
│  - Verify not already in target env     │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│  Allocate New Tenant ID                 │
│  - Call get_next_tenant_id(target_env)  │
│  - Validate ID within range             │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│  Display Migration Plan                 │
│  - Show before/after state              │
│  - Show all changes                     │
│  - Show record counts                   │
└────────────┬────────────────────────────┘
             │
             ├─ [--dry-run] ──> Exit (no changes)
             │
             ▼
┌─────────────────────────────────────────┐
│  Request Confirmation                   │
│  - Prompt: "Do you want to proceed?"    │
│  - Require typing "yes"                 │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│  Execute Migration Transaction          │
│  BEGIN;                                 │
│    1. INSERT new tenant                 │
│    2. UPDATE devices                    │
│    3. UPDATE api_keys                   │
│    4. UPDATE hndr_rules                 │
│    5. UPDATE status                     │
│    6. UPDATE version                    │
│    7. DELETE old tenant                 │
│  COMMIT;                                │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│  Verify Migration                       │
│  - Check new tenant exists              │
│  - Verify record counts                 │
│  - Confirm old tenant deleted           │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│  Display Summary                        │
│  - Show before/after comparison         │
│  - Confirm successful migration         │
└─────────────────────────────────────────┘
```
