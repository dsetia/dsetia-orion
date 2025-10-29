package main

import (
    "context"
    "os"
    "testing"
    "database/sql"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMain(m *testing.M) {
    // Run tests
    code := m.Run()
    os.Exit(code)
}

func setupTestDBContainer(t *testing.T) (*DB, func()) {
    ctx := context.Background()
    pgContainer, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:15-alpine"),
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("testuser"),
        postgres.WithPassword("testpass"),
        postgres.WithInitScripts("schema_pg.sql"),
    )
    if err != nil {
        t.Fatalf("failed to start postgres container: %s", err)
    }

    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        t.Fatalf("failed to get connection string: %s", err)
    }

    db, err := NewDB(connStr)
    if err != nil {
        t.Fatalf("failed to create DB: %s", err)
    }

    cleanup := func() {
        db.Close()
        pgContainer.Terminate(ctx)
    }

    return db, cleanup
}

func setupTestDBOriginal(t *testing.T) (*DB, func()) {
    connStr := "host=localhost port=5432 user=pguser password=pgpass dbname=testdb sslmode=disable"
    db, err := NewDB(connStr)
    if err != nil {
        t.Fatalf("failed to create DB: %s", err)
    }
    // Apply schema
    schema, err := os.ReadFile("schema_pg.sql")
    if err != nil {
        t.Fatalf("failed to read schema: %s", err)
    }
    _, err = db.Exec(string(schema))
    if err != nil {
        t.Fatalf("failed to apply schema: %s", err)
    }
    cleanup := func() {
        db.Close()
    }
    return db, cleanup
}

func setupTestDB(t *testing.T) (*DB, func()) {
    // Step 1: Connect to the default 'postgres' database to create testdb if it doesn't exist
    adminConnStr := "host=localhost port=5432 user=pguser password=pgpass dbname=postgres sslmode=disable"
    adminDB, err := sql.Open("postgres", adminConnStr)
    if err != nil {
        t.Fatalf("failed to connect to postgres database: %s", err)
    }

    // Check if testdb exists
    var dbExists bool
    err = adminDB.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'testdb')").Scan(&dbExists)
    if err != nil {
        adminDB.Close()
        t.Fatalf("failed to check if testdb exists: %s", err)
    }

    // Drop testdb if it exists and start afresh
    if dbExists {
        _, err = adminDB.Exec("DROP DATABASE testdb")
        if err != nil {
            adminDB.Close()
            t.Fatalf("failed to drop testdb: %s", err)
        }
    }

    // Create testdb
    _, err = adminDB.Exec("CREATE DATABASE testdb")
    if err != nil {
        adminDB.Close()
        t.Fatalf("failed to create testdb: %s", err)
    }
    adminDB.Close()

    // Step 2: Connect to testdb
    connStr := "host=localhost port=5432 user=pguser password=pgpass dbname=testdb sslmode=disable"
    db, err := NewDB(connStr)
    if err != nil {
        t.Fatalf("failed to connect to testdb: %s", err)
    }

    // Step 3: Apply schema
    schema, err := os.ReadFile("schema_pg.sql")
    if err != nil {
        db.Close()
        t.Fatalf("failed to read schema: %s", err)
    }
    _, err = db.Exec(string(schema))
    if err != nil {
        db.Close()
        t.Fatalf("failed to apply schema: %s", err)
    }

    // Step 4: Define cleanup function to drop testdb and close connection
    cleanup := func() {
        // Reconnect to postgres database to drop testdb
        adminDB, err := sql.Open("postgres", adminConnStr)
        if err != nil {
            t.Logf("failed to reconnect to postgres database for cleanup: %s", err)
            db.Close()
            return
        }
        defer adminDB.Close()

        // Terminate any active connections to testdb to allow dropping
        _, err = adminDB.Exec(`
            SELECT pg_terminate_backend(pg_stat_activity.pid)
            FROM pg_stat_activity
            WHERE pg_stat_activity.datname = 'testdb' AND pid <> pg_backend_pid()
        `)
        if err != nil {
            t.Logf("failed to terminate active connections to testdb: %s", err)
        }

        // Drop testdb
        _, err = adminDB.Exec("DROP DATABASE IF EXISTS testdb")
        if err != nil {
            t.Logf("failed to drop testdb: %s", err)
        }
    }

    return db, cleanup
}


func TestTenantOperations(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    t.Run("Insert and Get Tenant", func(t *testing.T) {
        tenantName := "test-tenant"
        id, err := db.GetOrInsertTenant(tenantName)
        assert.NoError(t, err)
        assert.True(t, id > 0)

        id2, err := db.GetOrInsertTenant(tenantName)
        assert.NoError(t, err)
        assert.Equal(t, id, id2, "should return same ID for existing tenant")
    })

    t.Run("Validate Tenant", func(t *testing.T) {
        tenantName := "validate-tenant"
        id, err := db.GetOrInsertTenant(tenantName)
        assert.NoError(t, err)

        exists, err := db.ValidateTenant(id)
        assert.NoError(t, err)
        assert.True(t, exists)

        exists, err = db.ValidateTenant(9999)
        assert.NoError(t, err)
        assert.False(t, exists)
    })

    t.Run("List Tenants", func(t *testing.T) {
        tenantName := "list-tenant"
        _, err := db.GetOrInsertTenant(tenantName)
        assert.NoError(t, err)

        tenants, err := db.ListTenants()
        assert.NoError(t, err)
        assert.True(t, len(tenants) > 0)
    })

    t.Run("Delete Tenant", func(t *testing.T) {
        tenantName := "delete-tenant"
        id, err := db.GetOrInsertTenant(tenantName)
        assert.NoError(t, err)

        err = db.DeleteTenant(id)
        assert.NoError(t, err)

        exists, err := db.ValidateTenant(id)
        assert.NoError(t, err)
        assert.False(t, exists)
    })
}

func TestDeviceOperations(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    tenantID, err := db.GetOrInsertTenant("device-tenant")
    assert.NoError(t, err)

    t.Run("Insert and Get Device", func(t *testing.T) {
        deviceName := "test-device"
        deviceID := uuid.New().String()
        id, err := db.GetOrInsertDevice(deviceID, tenantID, deviceName, "1.0")
        assert.NoError(t, err)
        assert.Equal(t, deviceID, id)

        id2, err := db.GetOrInsertDevice("", tenantID, deviceName, "1.0")
        assert.NoError(t, err)
        assert.Equal(t, id, id2)
    })

    t.Run("Insert and Update Device", func(t *testing.T) {
        deviceName := "test-device-2"
        deviceID := uuid.New().String()
        id, err := db.GetOrInsertDevice(deviceID, tenantID, deviceName, "1.0")
        assert.NoError(t, err)
        assert.Equal(t, deviceID, id)
	device, err := db.GetDeviceEntry(deviceID, tenantID)
        assert.NoError(t, err)
        assert.Equal(t, device.HndrSwVersion, "1.0")

	err = db.UpdateDeviceVersion(id, "2.0")
        assert.NoError(t, err)
        assert.Equal(t, deviceID, id)
	device, err = db.GetDeviceEntry(deviceID, tenantID)
        assert.NoError(t, err)
        assert.Equal(t, device.HndrSwVersion, "2.0")

	err = db.UpdateDeviceVersion(id, "")
        assert.NoError(t, err)
        assert.Equal(t, deviceID, id)
	device, err = db.GetDeviceEntry(deviceID, tenantID)
        assert.NoError(t, err)
        assert.Equal(t, device.HndrSwVersion, "")
    })

    t.Run("Validate Device", func(t *testing.T) {
        deviceID := uuid.New().String()
        deviceName := "validate-device"
        _, err := db.GetOrInsertDevice(deviceID, tenantID, deviceName, "1.0")
        assert.NoError(t, err)

        exists, err := db.ValidateDevice(deviceID, tenantID)
        assert.NoError(t, err)
        assert.True(t, exists)

        exists, err = db.ValidateDevice("invalid-device", tenantID)
        assert.NoError(t, err)
        assert.False(t, exists)
    })

    t.Run("List Devices", func(t *testing.T) {
        deviceID := uuid.New().String()
        deviceName := "list-device"
        _, err := db.GetOrInsertDevice(deviceID, tenantID, deviceName, "1.0")
        assert.NoError(t, err)

        devices, err := db.ListDevices(tenantID)
        assert.NoError(t, err)
        assert.True(t, len(devices) > 0)
    })

    t.Run("Delete Device", func(t *testing.T) {
        deviceID := uuid.New().String()
        deviceName := "delete-device"
        _, err := db.GetOrInsertDevice(deviceID, tenantID, deviceName, "1.0")
        assert.NoError(t, err)

        err = db.DeleteDevice(deviceID, tenantID)
        assert.NoError(t, err)

        exists, err := db.ValidateDevice(deviceID, tenantID)
        assert.NoError(t, err)
        assert.False(t, exists)
    })
}

func TestAPIKeyOperations(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    tenantID, err := db.GetOrInsertTenant("api-key-tenant")
    assert.NoError(t, err)
    deviceID := uuid.New().String()
    _, err = db.GetOrInsertDevice(deviceID, tenantID, "api-device", "1.0")
    assert.NoError(t, err)

    /*
    t.Run("Insert and Get API Key", func(t *testing.T) {
        apiKey := uuid.New().String()
        key, err := db.GetOrInsertAPIKey(apiKey, tenantID, deviceID, true)
        assert.NoError(t, err)
        assert.Equal(t, apiKey, key)
    })

    t.Run("Validate API Key", func(t *testing.T) {
        apiKey := uuid.New().String()
        _, err := db.GetOrInsertAPIKey(apiKey, tenantID, deviceID, true)
        assert.NoError(t, err)

        valid, tID, dID, err := db.ValidateAPIKey(apiKey)
        assert.NoError(t, err)
        assert.True(t, valid)
        assert.Equal(t, tenantID, tID)
        assert.Equal(t, deviceID, dID)
    })

    t.Run("List API Keys", func(t *testing.T) {
        apiKey := uuid.New().String()
        _, err := db.GetOrInsertAPIKey(apiKey, tenantID, deviceID, true)
        assert.NoError(t, err)

        keys, err := db.ListAPIKeys(tenantID)
        assert.NoError(t, err)
        assert.True(t, len(keys) > 0)
    })

    t.Run("Insert Validate and Delete API Key", func(t *testing.T) {
        apiKey := uuid.New().String()
        _, err := db.GetOrInsertAPIKey(apiKey, tenantID, deviceID, true)
        assert.NoError(t, err)

        err = db.DeleteAPIKey(apiKey)
        assert.NoError(t, err)

        valid, _, _, err := db.ValidateAPIKey(apiKey)
        assert.NoError(t, err)
        assert.False(t, valid)
    })
    */

    apiKey := uuid.New().String()
    t.Run("Insert and Get API Key", func(t *testing.T) {
        key, err := db.GetOrInsertAPIKey(apiKey, tenantID, deviceID, true)
        assert.NoError(t, err)
        assert.Equal(t, apiKey, key)
    })

    t.Run("Validate API Key", func(t *testing.T) {
        key, err := db.GetOrInsertAPIKey(uuid.New().String(), tenantID, deviceID, true)
        assert.NoError(t, err)
        assert.Equal(t, apiKey, key)

        valid, tID, dID, err := db.ValidateAPIKey(apiKey)
        assert.NoError(t, err)
        assert.True(t, valid)
        assert.Equal(t, tenantID, tID)
        assert.Equal(t, deviceID, dID)
    })

    t.Run("List API Keys", func(t *testing.T) {
        keys, err := db.ListAPIKeys(tenantID)
        assert.NoError(t, err)
        assert.True(t, len(keys) > 0)
    })

    t.Run("Delete API Key", func(t *testing.T) {
        err = db.DeleteAPIKey(apiKey)
        assert.NoError(t, err)

        valid, _, _, err := db.ValidateAPIKey(apiKey)
        assert.Error(t, err)  // no rows in result set
        assert.False(t, valid)
    })
}

// Add similar tests for HndrSw, HndrRules, ThreatIntel, and Status
func TestHndrSwOperations(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    t.Run("Insert HndrSw", func(t *testing.T) {
        version := "1.0.0"
        id, err := db.InsertHndrSw(version, 1000, "abc123")
        assert.NoError(t, err)
        assert.True(t, id > 0)

        id2, err := db.InsertHndrSw(version, 1000, "abc123")
        assert.NoError(t, err)
        assert.Equal(t, id2, id, "should not insert duplicate")
    })

    t.Run("Validate HndrSw", func(t *testing.T) {
        version := "2.0.0"
        _, err := db.InsertHndrSw(version, 1000, "def456")
        assert.NoError(t, err)

        exists, err := db.ValidateHndrSw(version)
        assert.NoError(t, err)
        assert.True(t, exists)
    })

    t.Run("List HndrSw", func(t *testing.T) {
        _, err := db.InsertHndrSw("3.0.0", 1000, "ghi789")
        assert.NoError(t, err)

        sw, err := db.ListHndrSw()
        assert.NoError(t, err)
        assert.True(t, len(sw) > 0)
    })

    t.Run("Delete HndrSw", func(t *testing.T) {
        version := "4.0.0"
        _, err := db.InsertHndrSw(version, 1000, "jkl012")
        assert.NoError(t, err)

        err = db.DeleteHndrSw(version)
        assert.NoError(t, err)

        exists, err := db.ValidateHndrSw(version)
        assert.NoError(t, err)
        assert.False(t, exists)
    })
}

func TestHndrRulesOperations(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    tenantID, err := db.GetOrInsertTenant("rules-tenant")
    assert.NoError(t, err)

    t.Run("Insert HndrRules", func(t *testing.T) {
        version := "rule-1.0"
        id, err := db.InsertHndrRules(tenantID, version, 1000, "xyz789")
        assert.NoError(t, err)
        assert.True(t, id > 0)

        id2, err := db.InsertHndrRules(tenantID, version, 1000, "xyz789")
        assert.NoError(t, err)
        assert.Equal(t, id2, id, "should not insert duplicate")
    })

    t.Run("Validate HndrRules", func(t *testing.T) {
        version := "rule-2.0"
        _, err := db.InsertHndrRules(tenantID, version, 1000, "abc012")
        assert.NoError(t, err)

        exists, err := db.ValidateHndrRules(tenantID, version)
        assert.NoError(t, err)
        assert.True(t, exists)
    })

    t.Run("List HndrRules", func(t *testing.T) {
        version := "rule-3.0"
        _, err := db.InsertHndrRules(tenantID, version, 1000, "def345")
        assert.NoError(t, err)

        rules, err := db.ListHndrRules(tenantID)
        assert.NoError(t, err)
        assert.True(t, len(rules) > 0)
    })

    t.Run("Delete HndrRules", func(t *testing.T) {
        version := "rule-4.0"
        _, err := db.InsertHndrRules(tenantID, version, 1000, "ghi678")
        assert.NoError(t, err)

        err = db.DeleteHndrRules(tenantID, version)
        assert.NoError(t, err)

        exists, err := db.ValidateHndrRules(tenantID, version)
        assert.NoError(t, err)
        assert.False(t, exists)
    })
}

func TestThreatIntelOperations(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    t.Run("Insert ThreatIntel", func(t *testing.T) {
        version := "ti-1.0"
        id, err := db.InsertThreatIntel(version, 1000, "ti123")
        assert.NoError(t, err)
        assert.True(t, id > 0)

        id2, err := db.InsertThreatIntel(version, 1000, "ti123")
        assert.NoError(t, err)
        assert.Equal(t, id2, id, "should not insert duplicate")
    })

    t.Run("Validate ThreatIntel", func(t *testing.T) {
        version := "ti-2.0"
        _, err := db.InsertThreatIntel(version, 1000, "ti456")
        assert.NoError(t, err)

        exists, err := db.ValidateThreatIntel(version)
        assert.NoError(t, err)
        assert.True(t, exists)
    })

    t.Run("List ThreatIntel", func(t *testing.T) {
        version := "ti-3.0"
        _, err := db.InsertThreatIntel(version, 1000, "ti789")
        assert.NoError(t, err)

        ti, err := db.ListThreatIntel()
        assert.NoError(t, err)
        assert.True(t, len(ti) > 0)
    })

    t.Run("Delete ThreatIntel", func(t *testing.T) {
        version := "ti-4.0"
        _, err := db.InsertThreatIntel(version, 1000, "ti012")
        assert.NoError(t, err)

        err = db.DeleteThreatIntel(version)
        assert.NoError(t, err)

        exists, err := db.ValidateThreatIntel(version)
        assert.NoError(t, err)
        assert.False(t, exists)
    })
}

func TestStatusOperations(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    tenantID, err := db.GetOrInsertTenant("status-tenant")
    assert.NoError(t, err)
    deviceID := uuid.New().String()
    _, err = db.GetOrInsertDevice(deviceID, tenantID, "status-device", "1.0")
    assert.NoError(t, err)

    t.Run("Insert Status", func(t *testing.T) {
        err := db.InsertStatus(deviceID, tenantID, "sw1.0", "rule1.0", "ti1.0")
        assert.NoError(t, err)

        status, err := db.GetStatus(deviceID, tenantID)
        assert.NoError(t, err)
        assert.Equal(t, "sw1.0", status.Software)
        assert.Equal(t, "rule1.0", status.Rules)
        assert.Equal(t, "ti1.0", status.ThreatIntel)
    })

    t.Run("Update Status", func(t *testing.T) {
        err := db.InsertStatus(deviceID, tenantID, "sw2.0", "", "")
        assert.NoError(t, err)

        status, err := db.GetStatus(deviceID, tenantID)
        assert.NoError(t, err)
        assert.Equal(t, "sw2.0", status.Software)
        assert.Equal(t, "rule1.0", status.Rules)
        assert.Equal(t, "ti1.0", status.ThreatIntel)
    })

    t.Run("List Status", func(t *testing.T) {
        statuses, err := db.ListStatus()
        assert.NoError(t, err)
        assert.True(t, len(statuses) > 0)
    })

    t.Run("Delete Status", func(t *testing.T) {
        err := db.InsertStatus(deviceID, tenantID, "sw3.0", "rule3.0", "ti3.0")
        assert.NoError(t, err)

        err = db.DeleteStatus(deviceID, tenantID)
        assert.NoError(t, err)

        _, err = db.GetStatus(deviceID, tenantID)
        assert.Error(t, err)
    })
}

