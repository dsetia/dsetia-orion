// Copyright (c) 2025 SecurITe
// All rights reserved.
//
// This source code is the property of SecurITe.
// Unauthorized copying, modification, or distribution of this file,
// via any medium is strictly prohibited unless explicitly authorized
// by SecurITe.
//
// This software is proprietary and confidential.
//
// File Owner:       deepinder@securite.world
// Created On:       05/26/2025

package main

import (
    "database/sql"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    _ "github.com/lib/pq"
)

// DB is the SQLite database handle
type DB struct {
    *sql.DB
}

// Tenant represents the tenants table
type Tenant struct {
    ID        int64
    Name      string
    CreatedAt time.Time
    UpdatedAt time.Time
}

// Device represents the devices table
type Device struct {
    ID            string
    TenantID      int64
    Name          string
    HndrSwVersion string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// APIKey represents the api_keys table
type APIKey struct {
    Key       string
    TenantID  int64
    DeviceID  string
    IsActive  bool
    CreatedAt time.Time
}

// HndrSw represents the hndr_sw table
type HndrSw struct {
    ID        int64
    Version   string
    Size      int64
    Sha256    string
    UpdatedAt time.Time
}

// HndrRules represents the hndr_rules table
type HndrRules struct {
    ID        int64
    TenantID  int64
    Version   string
    Size      int64
    Sha256    string
    UpdatedAt time.Time
}

// ThreatIntel represents the threatintel table
type ThreatIntel struct {
    ID        int64
    Version   string
    Size      int64
    Sha256    string
    UpdatedAt time.Time
}

// Status represents the status table
type Status struct {
    DeviceID      string
    TenantID      int64
    Software      string
    Rules         string
    ThreatIntel   string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// NewDB initializes a new SQLite database connection
func NewDB(dbPath string) (*DB, error) {
    db, err := sql.Open("postgres", dbPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    // Enable foreign keys
    //_, err = db.Exec("PRAGMA foreign_keys = ON;")
    //if err != nil {
        //return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
    //}
    return &DB{db}, nil
}

// GetOrInsertTenant retrieves an existing tenant by name or inserts a new one
func (db *DB) GetOrInsertTenant(name string) (int64, error) {
    if name == "" {
        return 0, errors.New("tenant name cannot be empty")
    }
    var tenantID int64
    err := db.QueryRow("SELECT tenant_id FROM tenants WHERE tenant_name = $1", name).Scan(&tenantID)
    if err == nil {
        return tenantID, nil
    }
    if err != sql.ErrNoRows {
        return 0, fmt.Errorf("failed to check tenant: %w", err)
    }
    err = db.QueryRow(`
        INSERT INTO tenants (tenant_name, created_at, updated_at)
        VALUES ($1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	RETURNING tenant_id
    `, name).Scan(&tenantID)
    if err != nil {
        return 0, fmt.Errorf("failed to insert tenant: %w", err)
    }
    return tenantID, nil
}

// ValidateTenant checks if a tenant exists by ID
func (db *DB) ValidateTenant(id int64) (bool, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM tenants WHERE tenant_id = $1", id).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("failed to validate tenant: %w", err)
    }
    return count > 0, nil
}

// DeleteTenant deletes a tenant by ID
func (db *DB) DeleteTenant(id int64) (error) {
    result, err := db.Exec("DELETE FROM tenants WHERE tenant_id = $1", id)
    if err != nil {
        return fmt.Errorf("failed to delete tenant: %w", err)
    }
    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("failed to determine if tenant was deleted: %w", err)
    }
    if rowsAffected == 0 {
        return fmt.Errorf("no tenant found with ID %d", id)
    }
    return nil
}

// ListTenants retrieves all tenants
func (db *DB) ListTenants() ([]Tenant, error) {
    rows, err := db.Query("SELECT tenant_id, tenant_name, created_at, updated_at FROM tenants")
    if err != nil {
        return nil, fmt.Errorf("failed to list tenants: %w", err)
    }
    defer rows.Close()

    var tenants []Tenant
    for rows.Next() {
        var t Tenant
        if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt); err != nil {
            return nil, fmt.Errorf("failed to scan tenant: %w", err)
        }
        tenants = append(tenants, t)
    }
    return tenants, nil
}

// GetOrInsertDevice retrieves an existing device or inserts a new one
func (db *DB) GetOrInsertDevice(deviceID string, tenantID int64, deviceName string, hndrSwVersion string) (string, error) {
    if deviceName == "" {
        return "", errors.New("device name cannot be empty")
    }
    exists, err := db.ValidateTenant(tenantID)
    if err != nil {
        return "", err
    }
    if !exists {
        return "", fmt.Errorf("tenant ID %d does not exist", tenantID)
    }
    var existingID string
    err = db.QueryRow("SELECT device_id FROM devices WHERE device_name = $1 AND tenant_id = $2", deviceName, tenantID).Scan(&existingID)
    if err == nil {
        return existingID, nil
    }
    if err != sql.ErrNoRows {
        return "", fmt.Errorf("failed to check device: %w", err)
    }
    if deviceID == "" {
        deviceID = uuid.New().String()
    }
    _, err = db.Exec(`
        INSERT INTO devices (device_id, tenant_id, device_name, hndr_sw_version, created_at, updated_at)
        VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    `, deviceID, tenantID, deviceName, hndrSwVersion)
    if err != nil {
        return "", fmt.Errorf("failed to insert device: %w", err)
    }
    return deviceID, nil
}

// ValidateDevice checks if a device exists and belongs to the tenant
func (db *DB) ValidateDevice(deviceID string, tenantID int64) (bool, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM devices WHERE device_id = $1 AND tenant_id = $2", deviceID, tenantID).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("failed to validate device: %w", err)
    }
    return count > 0, nil
}

// ListDevices retrieves all devices, optionally filtered by tenant_id
func (db *DB) ListDevices(tenantID int64) ([]Device, error) {
    query := "SELECT device_id, tenant_id, device_name, hndr_sw_version, created_at, updated_at FROM devices"
    args := []interface{}{}
    if tenantID > 0 {
        query += " WHERE tenant_id = $1"
        args = append(args, tenantID)
    }
    rows, err := db.Query(query, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to list devices: %w", err)
    }
    defer rows.Close()

    var devices []Device
    for rows.Next() {
        var d Device
        if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.HndrSwVersion, &d.CreatedAt, &d.UpdatedAt); err != nil {
            return nil, fmt.Errorf("failed to scan device: %w", err)
        }
        devices = append(devices, d)
    }
    return devices, nil
}

// GetOrInsertAPIKey retrieves an existing API key or inserts a new one
func (db *DB) GetOrInsertAPIKey(apiKey string, tenantID int64, deviceID string, isActive bool) (string, error) {
    if deviceID == "" {
        return "", errors.New("device ID cannot be empty")
    }
    exists, err := db.ValidateDevice(deviceID, tenantID)
    if err != nil {
        return "", err
    }
    if !exists {
        return "", fmt.Errorf("device %s does not exist for tenant %d", deviceID, tenantID)
    }
    var existingKey string
    err = db.QueryRow("SELECT api_key FROM api_keys WHERE device_id = $1", deviceID).Scan(&existingKey)
    if err == nil {
        return existingKey, nil
    }
    if err != sql.ErrNoRows {
        return "", fmt.Errorf("failed to check API key: %w", err)
    }
    if apiKey == "" {
        apiKey = uuid.New().String()
    }
    _, err = db.Exec(`
        INSERT INTO api_keys (api_key, tenant_id, device_id, is_active, created_at)
        VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
    `, apiKey, tenantID, deviceID, isActive)
    if err != nil {
        return "", fmt.Errorf("failed to insert API key: %w", err)
    }
    return apiKey, nil
}

// ValidateAPIKey checks if an API key is valid and active
func (db *DB) ValidateAPIKey(apiKey string) (bool, int64, string, error) {
    var tenantID int64
    var deviceID string
    var isActive bool
    err := db.QueryRow(`
        SELECT tenant_id, device_id, is_active
        FROM api_keys
        WHERE api_key = $1
    `, apiKey).Scan(&tenantID, &deviceID, &isActive)
    if err != nil {
        return false, 0, "", fmt.Errorf("failed to validate API key: %w", err)
    }
    return isActive, tenantID, deviceID, nil
}

// ListAPIKeys retrieves all API keys, optionally filtered by tenant_id
func (db *DB) ListAPIKeys(tenantID int64) ([]APIKey, error) {
    query := "SELECT api_key, tenant_id, device_id, is_active, created_at FROM api_keys"
    args := []interface{}{}
    if tenantID > 0 {
        query += " WHERE tenant_id = $1"
        args = append(args, tenantID)
    }
    rows, err := db.Query(query, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to list API keys: %w", err)
    }
    defer rows.Close()

    var keys []APIKey
    for rows.Next() {
        var k APIKey
        if err := rows.Scan(&k.Key, &k.TenantID, &k.DeviceID, &k.IsActive, &k.CreatedAt); err != nil {
            return nil, fmt.Errorf("failed to scan API key: %w", err)
        }
        keys = append(keys, k)
    }
    return keys, nil
}

// InsertHndrSw adds a new software version
func (db *DB) InsertHndrSw(version string, size int64, sha256 string) (int64, error) {
    if version == "" || sha256 == "" {
        return 0, errors.New("version and sha256 cannot be empty")
    }
    if size <= 0 {
        return 0, errors.New("size must be positive")
    }

    // avoid duplicate
    exists, err := db.ValidateHndrSw(version)
    if err != nil {
        return 0, err
    }
    if exists {
        fmt.Printf("HndrRules version %s already exists\n", version)
	return 0, nil
    }

    var id int64
    err = db.QueryRow(`
        INSERT INTO hndr_sw (version, size, sha256, updated_at)
        VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
	RETURNING id
    `, version, size, sha256).Scan(&id)
    if err != nil {
        return 0, fmt.Errorf("failed to insert hndr_sw: %w", err)
    }
    return id, nil
}

// ValidateHndrSw checks if a software version exists
func (db *DB) ValidateHndrSw(version string) (bool, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM hndr_sw WHERE version = $1", version).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("failed to validate hndr_sw: %w", err)
    }
    return count > 0, nil
}

func (db *DB) DeleteHndrSw(version string) (error) {
    result, err := db.Exec("DELETE FROM hndr_sw WHERE version = $1", version)
    if err != nil {
        return fmt.Errorf("failed to delete version: %w", err)
    }
    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("failed to determine if version was deleted: %w", err)
    }
    if rowsAffected == 0 {
        return fmt.Errorf("no version found with ID %s", version)
    }
    return nil
}

// ListHndrSw retrieves all software versions
func (db *DB) ListHndrSw() ([]HndrSw, error) {
    rows, err := db.Query("SELECT id, version, size, sha256, updated_at FROM hndr_sw")
    if err != nil {
        return nil, fmt.Errorf("failed to list hndr_sw: %w", err)
    }
    defer rows.Close()

    var sw []HndrSw
    for rows.Next() {
        var s HndrSw
        if err := rows.Scan(&s.ID, &s.Version, &s.Size, &s.Sha256, &s.UpdatedAt); err != nil {
            return nil, fmt.Errorf("failed to scan hndr_sw: %w", err)
        }
        sw = append(sw, s)
    }
    return sw, nil
}

// InsertHndrRules adds a new rule version for a tenant
func (db *DB) InsertHndrRules(tenantID int64, version string, size int64, sha256 string) (int64, error) {
    if version == "" || sha256 == "" {
        return 0, errors.New("version and sha256 cannot be empty")
    }
    if size <= 0 {
        return 0, errors.New("size must be positive")
    }
    exists, err := db.ValidateTenant(tenantID)
    if err != nil {
        return 0, err
    }
    if !exists {
        return 0, fmt.Errorf("tenant ID %d does not exist", tenantID)
    }

    // avoid duplicate
    exists, err = db.ValidateHndrRules(tenantID, version)
    if err != nil {
        return 0, err
    }
    if exists {
        fmt.Printf("HndrRules version %s already exists for tenant %d\n", version, tenantID)
	return 0, nil
    }

    var id int64
    err = db.QueryRow(`
        INSERT INTO hndr_rules (tenant_id, version, size, sha256, updated_at)
        VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
	RETURNING id
    `, tenantID, version, size, sha256).Scan(&id)
    if err != nil {
        return 0, fmt.Errorf("failed to insert hndr_rules: %w", err)
    }
    return id, nil
}

// ValidateHndrRules checks if a rule version exists for a tenant
func (db *DB) ValidateHndrRules(tenantID int64, version string) (bool, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM hndr_rules WHERE tenant_id = $1 AND version = $2", tenantID, version).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("failed to validate hndr_rules: %w", err)
    }
    return count > 0, nil
}

// ListHndrRules retrieves all rule versions, optionally filtered by tenant_id
func (db *DB) ListHndrRules(tenantID int64) ([]HndrRules, error) {
    query := "SELECT id, tenant_id, version, size, sha256, updated_at FROM hndr_rules"
    args := []interface{}{}
    if tenantID > 0 {
        query += " WHERE tenant_id = $1"
        args = append(args, tenantID)
    }
    rows, err := db.Query(query, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to list hndr_rules: %w", err)
    }
    defer rows.Close()

    var rules []HndrRules
    for rows.Next() {
        var r HndrRules
        if err := rows.Scan(&r.ID, &r.TenantID, &r.Version, &r.Size, &r.Sha256, &r.UpdatedAt); err != nil {
            return nil, fmt.Errorf("failed to scan hndr_rules: %w", err)
        }
        rules = append(rules, r)
    }
    return rules, nil
}

// InsertThreatIntel adds a new threat intelligence version
func (db *DB) InsertThreatIntel(version string, size int64, sha256 string) (int64, error) {
    if version == "" || sha256 == "" {
        return 0, errors.New("version and sha256 cannot be empty")
    }
    if size <= 0 {
        return 0, errors.New("size must be positive")
    }

    // avoid duplicate
    exists, err := db.ValidateThreatIntel(version)
    if err != nil {
        return 0, err
    }
    if exists {
        fmt.Printf("ThreatIntel version %s already exists\n", version)
	return 0, nil
    }

    // insert
    var id int64
    err = db.QueryRow(`
        INSERT INTO threatintel (version, size, sha256, updated_at)
        VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
	RETURNING id
    `, version, size, sha256).Scan(&id)
    if err != nil {
        return 0, fmt.Errorf("failed to insert threatintel: %w", err)
    }
    return id, nil
}

// ValidateThreatIntel checks if a threat intelligence version exists
func (db *DB) ValidateThreatIntel(version string) (bool, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM threatintel WHERE version = $1", version).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("failed to validate threatintel: %w", err)
    }
    return count > 0, nil
}

// ListThreatIntel retrieves all threat intelligence versions
func (db *DB) ListThreatIntel() ([]ThreatIntel, error) {
    rows, err := db.Query("SELECT id, version, size, sha256, updated_at FROM threatintel")
    if err != nil {
        return nil, fmt.Errorf("failed to list threatintel: %w", err)
    }
    defer rows.Close()

    var ti []ThreatIntel
    for rows.Next() {
        var t ThreatIntel
        if err := rows.Scan(&t.ID, &t.Version, &t.Size, &t.Sha256, &t.UpdatedAt); err != nil {
            return nil, fmt.Errorf("failed to scan threatintel: %w", err)
        }
        ti = append(ti, t)
    }
    return ti, nil
}

func (db *DB) InsertStatus(deviceID string, tenantID int64, sSoftware string, sRules string, sThreatIntel string) (error) {
    // Ensure at least one status field is provided
    if sSoftware == "" && sRules == "" && sThreatIntel == "" {
        return errors.New("At least one of software, rules, or threatintel must be provided")
    }

    exists, err := db.ValidateDevice(deviceID, tenantID)
    if err != nil {
        return err
    }
    if !exists {
        return fmt.Errorf("device ID %s or tenant ID %d does not exist", deviceID, tenantID)
    }

    // Check if the row exists
    var cur Status
    err = db.QueryRow(`
        SELECT device_id, tenant_id, software, rules, threatintel, created_at
        FROM status
        WHERE device_id = $1 AND tenant_id = $2`,
        deviceID, tenantID).Scan(
        &cur.DeviceID, &cur.TenantID, &cur.Software, &cur.Rules, &cur.ThreatIntel, &cur.CreatedAt,
    )
    if err == sql.ErrNoRows {
        _, err = db.Exec(`
            INSERT INTO status (device_id, tenant_id, software, rules, threatintel, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
            deviceID, tenantID, sSoftware, sRules, sThreatIntel,
        )
        if err != nil {
            return fmt.Errorf("Failed to create status: "+err.Error())
        }
    } else {
        // update existing row
	software := cur.Software
	if sSoftware != "" {
	    software = sSoftware
	}
	rules := cur.Rules
	if sRules != "" {
	    rules = sRules
	}
	threatintel := cur.ThreatIntel
	if sThreatIntel != "" {
	    threatintel = sThreatIntel
	}
        _, err = db.Exec(`
	    UPDATE status
	    SET software = $1, rules = $2, threatintel = $3, updated_at = CURRENT_TIMESTAMP
	    WHERE device_id = $4 AND tenant_id = $5`,
            software, rules, threatintel, deviceID, tenantID,
        )
        if err != nil {
            return fmt.Errorf("Failed to update status: "+err.Error())
        }
    }

    return nil
}

func (db *DB) GetStatus(deviceID string, tenantID int64) (Status, error) {
    exists, err := db.ValidateDevice(deviceID, tenantID)
    if err != nil {
        return Status{}, err
    }
    if !exists {
        return Status{}, fmt.Errorf("device ID %s or tenant ID %d does not exist", deviceID, tenantID)
    }

    // Check if the row exists
    var cur Status
    err = db.QueryRow(`
        SELECT device_id, tenant_id, software, rules, threatintel, created_at
        FROM status
        WHERE device_id = $1 AND tenant_id = $2`,
        deviceID, tenantID).Scan(
        &cur.DeviceID, &cur.TenantID, &cur.Software, &cur.Rules, &cur.ThreatIntel, &cur.CreatedAt,
    )
    if err == sql.ErrNoRows {
        return Status{}, fmt.Errorf("Status entry for device ID %s does not exist", deviceID)
    }

    return Status {
        Software: cur.Software,
	Rules: cur.Rules,
	ThreatIntel: cur.ThreatIntel,
    }, nil
}

// ListThreatIntel retrieves all threat intelligence versions
func (db *DB) ListStatus() ([]Status, error) {
    rows, err := db.Query("SELECT device_id, tenant_id, software, rules, threatintel, updated_at FROM status")
    if err != nil {
        return nil, fmt.Errorf("failed to list status: %w", err)
    }
    defer rows.Close()

    var ti []Status
    for rows.Next() {
        var t Status
        if err := rows.Scan(&t.DeviceID, &t.TenantID, &t.Software, &t.Rules, &t.ThreatIntel, &t.UpdatedAt); err != nil {
            return nil, fmt.Errorf("failed to scan status: %w", err)
        }
        ti = append(ti, t)
    }
    return ti, nil
}

