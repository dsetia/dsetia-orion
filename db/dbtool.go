package main

import (
    "database/sql"
    "errors"
    "flag"
    "fmt"
    "os"
    "time"

    _ "github.com/mattn/go-sqlite3"
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

// NewDB initializes a new SQLite database connection
func NewDB(dbPath string) (*DB, error) {
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    // Enable foreign keys
    _, err = db.Exec("PRAGMA foreign_keys = ON;")
    if err != nil {
        return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
    }
    return &DB{db}, nil
}

// Tenant Operations

// GetOrInsertTenant retrieves an existing tenant by name or inserts a new one
func (db *DB) GetOrInsertTenant(name string) (int64, error) {
    if name == "" {
        return 0, errors.New("tenant name cannot be empty")
    }
    var tenantID int64
    err := db.QueryRow("SELECT tenant_id FROM tenants WHERE tenant_name = ?", name).Scan(&tenantID)
    if err == nil {
        return tenantID, nil
    }
    if err != sql.ErrNoRows {
        return 0, fmt.Errorf("failed to check tenant: %w", err)
    }
    result, err := db.Exec(`
        INSERT INTO tenants (tenant_name, created_at, updated_at)
        VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    `, name)
    if err != nil {
        return 0, fmt.Errorf("failed to insert tenant: %w", err)
    }
    id, err := result.LastInsertId()
    if err != nil {
        return 0, fmt.Errorf("failed to get tenant ID: %w", err)
    }
    return id, nil
}

// ValidateTenant checks if a tenant exists by ID
func (db *DB) ValidateTenant(id int64) (bool, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM tenants WHERE tenant_id = ?", id).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("failed to validate tenant: %w", err)
    }
    return count > 0, nil
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

// Device Operations

// GetOrInsertDevice retrieves an existing device or inserts a new one
func (db *DB) GetOrInsertDevice(deviceID string, tenantID int64, name string, hndrSwVersion string) (string, error) {
    if deviceID == "" {
        return "", errors.New("device ID cannot be empty")
    }
    exists, err := db.ValidateTenant(tenantID)
    if err != nil {
        return "", err
    }
    if !exists {
        return "", fmt.Errorf("tenant ID %d does not exist", tenantID)
    }
    var existingID string
    err = db.QueryRow("SELECT device_id FROM devices WHERE device_id = ? AND tenant_id = ?", deviceID, tenantID).Scan(&existingID)
    if err == nil {
        return existingID, nil
    }
    if err != sql.ErrNoRows {
        return "", fmt.Errorf("failed to check device: %w", err)
    }
    _, err = db.Exec(`
        INSERT INTO devices (device_id, tenant_id, device_name, hndr_sw_version, created_at, updated_at)
        VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    `, deviceID, tenantID, name, hndrSwVersion)
    if err != nil {
        return "", fmt.Errorf("failed to insert device: %w", err)
    }
    return deviceID, nil
}

// ValidateDevice checks if a device exists and belongs to the tenant
func (db *DB) ValidateDevice(deviceID string, tenantID int64) (bool, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM devices WHERE device_id = ? AND tenant_id = ?", deviceID, tenantID).Scan(&count)
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
        query += " WHERE tenant_id = ?"
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

// APIKey Operations

// GetOrInsertAPIKey retrieves an existing API key or inserts a new one
func (db *DB) GetOrInsertAPIKey(apiKey string, tenantID int64, deviceID string, isActive bool) (string, error) {
    if apiKey == "" || deviceID == "" {
        return "", errors.New("API key and device ID cannot be empty")
    }
    exists, err := db.ValidateDevice(deviceID, tenantID)
    if err != nil {
        return "", err
    }
    if !exists {
        return "", fmt.Errorf("device %s does not exist for tenant %d", deviceID, tenantID)
    }
    var existingKey string
    err = db.QueryRow("SELECT api_key FROM api_keys WHERE api_key = ?", apiKey).Scan(&existingKey)
    if err == nil {
        return existingKey, nil
    }
    if err != sql.ErrNoRows {
        return "", fmt.Errorf("failed to check API key: %w", err)
    }
    _, err = db.Exec(`
        INSERT INTO api_keys (api_key, tenant_id, device_id, is_active, created_at)
        VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
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
        WHERE api_key = ?
    `, apiKey).Scan(&tenantID, &deviceID, &isActive)
    if err == sql.ErrNoRows {
        return false, 0, "", nil
    }
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
        query += " WHERE tenant_id = ?"
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

// HndrSw Operations

// InsertHndrSw adds a new software version
func (db *DB) InsertHndrSw(version string, size int64, sha256 string) (int64, error) {
    if version == "" || sha256 == "" {
        return 0, errors.New("version and sha256 cannot be empty")
    }
    if size <= 0 {
        return 0, errors.New("size must be positive")
    }
    result, err := db.Exec(`
        INSERT INTO hndr_sw (version, size, sha256, updated_at)
        VALUES (?, ?, ?, CURRENT_TIMESTAMP)
    `, version, size, sha256)
    if err != nil {
        return 0, fmt.Errorf("failed to insert hndr_sw: %w", err)
    }
    id, err := result.LastInsertId()
    if err != nil {
        return 0, fmt.Errorf("failed to get hndr_sw ID: %w", err)
    }
    return id, nil
}

// ValidateHndrSw checks if a software version exists
func (db *DB) ValidateHndrSw(version string) (bool, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM hndr_sw WHERE version = ?", version).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("failed to validate hndr_sw: %w", err)
    }
    return count > 0, nil
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

// HndrRules Operations

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
    result, err := db.Exec(`
        INSERT INTO hndr_rules (tenant_id, version, size, sha256, updated_at)
        VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
    `, tenantID, version, size, sha256)
    if err != nil {
        return 0, fmt.Errorf("failed to insert hndr_rules: %w", err)
    }
    id, err := result.LastInsertId()
    if err != nil {
        return 0, fmt.Errorf("failed to get hndr_rules ID: %w", err)
    }
    return id, nil
}

// ValidateHndrRules checks if a rule version exists for a tenant
func (db *DB) ValidateHndrRules(tenantID int64, version string) (bool, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM hndr_rules WHERE tenant_id = ? AND version = ?", tenantID, version).Scan(&count)
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
        query += " WHERE tenant_id = ?"
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

// ThreatIntel Operations

// InsertThreatIntel adds a new threat intelligence version
func (db *DB) InsertThreatIntel(version string, size int64, sha256 string) (int64, error) {
    if version == "" || sha256 == "" {
        return 0, errors.New("version and sha256 cannot be empty")
    }
    if size <= 0 {
        return 0, errors.New("size must be positive")
    }
    result, err := db.Exec(`
        INSERT INTO threatintel (version, size, sha256, updated_at)
        VALUES (?, ?, ?, CURRENT_TIMESTAMP)
    `, version, size, sha256)
    if err != nil {
        return 0, fmt.Errorf("failed to insert threatintel: %w", err)
    }
    id, err := result.LastInsertId()
    if err != nil {
        return 0, fmt.Errorf("failed to get threatintel ID: %w", err)
    }
    return id, nil
}

// ValidateThreatIntel checks if a threat intelligence version exists
func (db *DB) ValidateThreatIntel(version string) (bool, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM threatintel WHERE version = ?", version).Scan(&count)
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

// Command-line interface
func main() {
    // Common flags
    dbPath := flag.String("db", "/home/dsetia/securite/apis/updater.db", "Path to SQLite database")
    op := flag.String("op", "", "Operation to perform (e.g., insert-tenant, list-devices)")

    // Tenant flags
    tenantName := flag.String("tenant-name", "", "Tenant name")
    tenantID := flag.Int64("tenant-id", 0, "Tenant ID")

    // Device flags
    deviceID := flag.String("device-id", "", "Device ID")
    deviceName := flag.String("device-name", "", "Device name")
    hndrSwVersion := flag.String("hndr-sw-version", "", "Handr software version")

    // APIKey flags
    apiKey := flag.String("api-key", "", "API key")
    isActive := flag.Bool("is-active", true, "API key active status")

    // HndrSw flags
    swVersion := flag.String("sw-version", "", "Software version")
    swSize := flag.Int64("sw-size", 0, "Software size in bytes")
    swSha256 := flag.String("sw-sha256", "", "Software SHA256 hash")

    // HndrRules flags
    rulesVersion := flag.String("rules-version", "", "Rules version")
    rulesSize := flag.Int64("rules-size", 0, "Rules size in bytes")
    rulesSha256 := flag.String("rules-sha256", "", "Rules SHA256 hash")

    // ThreatIntel flags
    tiVersion := flag.String("ti-version", "", "Threat intelligence version")
    tiSize := flag.Int64("ti-size", 0, "Threat intelligence size in bytes")
    tiSha256 := flag.String("ti-sha256", "", "Threat intelligence SHA256 hash")

    flag.Parse()

    if *op == "" {
        fmt.Println("Error: -op flag is required")
        fmt.Println("Usage: ./dbutil -db <path> -op <operation> [args]")
        fmt.Println("Operations:")
        fmt.Println("  insert-tenant, validate-tenant, list-tenants")
        fmt.Println("  insert-device, validate-device, list-devices")
        fmt.Println("  insert-api-key, validate-api-key, list-api-keys")
        fmt.Println("  insert-hndr-sw, validate-hndr-sw, list-hndr-sw")
        fmt.Println("  insert-hndr-rules, validate-hndr-rules, list-hndr-rules")
        fmt.Println("  insert-threat-intel, validate-threat-intel, list-threat-intel")
        os.Exit(1)
    }

    db, err := NewDB(*dbPath)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        os.Exit(1)
    }
    defer db.Close()

    switch *op {
    // Tenant Operations
    case "insert-tenant":
        if *tenantName == "" {
            fmt.Println("Error: -tenant-name is required for insert-tenant")
            os.Exit(1)
        }
        id, err := db.GetOrInsertTenant(*tenantName)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("Tenant inserted or found: ID=%d\n", id)

    case "validate-tenant":
        if *tenantID == 0 {
            fmt.Println("Error: -tenant-id is required for validate-tenant")
            os.Exit(1)
        }
        exists, err := db.ValidateTenant(*tenantID)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("Tenant exists: %v\n", exists)

    case "list-tenants":
        tenants, err := db.ListTenants()
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        for _, t := range tenants {
            fmt.Printf("Tenant: ID=%d, Name=%s, Created=%s\n", t.ID, t.Name, t.CreatedAt)
        }

    // Device Operations
    case "insert-device":
        if *deviceID == "" || *tenantID == 0 {
            fmt.Println("Error: -device-id and -tenant-id are required for insert-device")
            os.Exit(1)
        }
        id, err := db.GetOrInsertDevice(*deviceID, *tenantID, *deviceName, *hndrSwVersion)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("Device inserted or found: ID=%s\n", id)

    case "validate-device":
        if *deviceID == "" || *tenantID == 0 {
            fmt.Println("Error: -device-id and -tenant-id are required for validate-device")
            os.Exit(1)
        }
        exists, err := db.ValidateDevice(*deviceID, *tenantID)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("Device exists: %v\n", exists)

    case "list-devices":
        tenantID := *tenantID
        devices, err := db.ListDevices(tenantID)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        for _, d := range devices {
            fmt.Printf("Device: ID=%s, TenantID=%d, Name=%s, HndrSwVersion=%s\n",
                d.ID, d.TenantID, d.Name, d.HndrSwVersion)
        }

    // APIKey Operations
    case "insert-api-key":
        if *apiKey == "" || *tenantID == 0 || *deviceID == "" {
            fmt.Println("Error: -api-key, -tenant-id, and -device-id are required for insert-api-key")
            os.Exit(1)
        }
        key, err := db.GetOrInsertAPIKey(*apiKey, *tenantID, *deviceID, *isActive)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("API Key inserted or found: Key=%s\n", key)

    case "validate-api-key":
        if *apiKey == "" {
            fmt.Println("Error: -api-key is required for validate-api-key")
            os.Exit(1)
        }
        valid, tenantID, deviceID, err := db.ValidateAPIKey(*apiKey)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("API Key valid: %v, TenantID: %d, DeviceID: %s\n", valid, tenantID, deviceID)

    case "list-api-keys":
        tenantID := *tenantID
        keys, err := db.ListAPIKeys(tenantID)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        for _, k := range keys {
            fmt.Printf("APIKey: Key=%s, TenantID=%d, DeviceID=%s, Active=%v\n",
                k.Key, k.TenantID, k.DeviceID, k.IsActive)
        }

    // HndrSw Operations
    case "insert-hndr-sw":
        if *swVersion == "" || *swSha256 == "" || *swSize <= 0 {
            fmt.Println("Error: -sw-version, -sw-sha256, and -sw-size are required for insert-hndr-sw")
            os.Exit(1)
        }
        id, err := db.InsertHndrSw(*swVersion, *swSize, *swSha256)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("HndrSw inserted: ID=%d\n", id)

    case "validate-hndr-sw":
        if *swVersion == "" {
            fmt.Println("Error: -sw-version is required for validate-hndr-sw")
            os.Exit(1)
        }
        exists, err := db.ValidateHndrSw(*swVersion)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("HndrSw exists: %v\n", exists)

    case "list-hndr-sw":
        sw, err := db.ListHndrSw()
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        for _, s := range sw {
            fmt.Printf("HndrSw: ID=%d, Version=%s, Size=%d, Sha256=%s\n",
                s.ID, s.Version, s.Size, s.Sha256)
        }

    // HndrRules Operations
    case "insert-hndr-rules":
        if *tenantID == 0 || *rulesVersion == "" || *rulesSha256 == "" || *rulesSize <= 0 {
            fmt.Println("Error: -tenant-id, -rules-version, -rules-sha256, and -rules-size are required for insert-hndr-rules")
            os.Exit(1)
        }
        id, err := db.InsertHndrRules(*tenantID, *rulesVersion, *rulesSize, *rulesSha256)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("HndrRules inserted: ID=%d\n", id)

    case "validate-hndr-rules":
        if *tenantID == 0 || *rulesVersion == "" {
            fmt.Println("Error: -tenant-id and -rules-version are required for validate-hndr-rules")
            os.Exit(1)
        }
        exists, err := db.ValidateHndrRules(*tenantID, *rulesVersion)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("HndrRules exists: %v\n", exists)

    case "list-hndr-rules":
        tenantID := *tenantID
        rules, err := db.ListHndrRules(tenantID)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        for _, r := range rules {
            fmt.Printf("HndrRules: ID=%d, TenantID=%d, Version=%s, Size=%d, Sha256=%s\n",
                r.ID, r.TenantID, r.Version, r.Size, r.Sha256)
        }

    // ThreatIntel Operations
    case "insert-threat-intel":
        if *tiVersion == "" || *tiSha256 == "" || *tiSize <= 0 {
            fmt.Println("Error: -ti-version, -ti-sha256, and -ti-size are required for insert-threat-intel")
            os.Exit(1)
        }
        id, err := db.InsertThreatIntel(*tiVersion, *tiSize, *tiSha256)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("ThreatIntel inserted: ID=%d\n", id)

    case "validate-threat-intel":
        if *tiVersion == "" {
            fmt.Println("Error: -ti-version is required for validate-threat-intel")
            os.Exit(1)
        }
        exists, err := db.ValidateThreatIntel(*tiVersion)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("ThreatIntel exists: %v\n", exists)

    case "list-threat-intel":
        ti, err := db.ListThreatIntel()
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        for _, t := range ti {
            fmt.Printf("ThreatIntel: ID=%d, Version=%s, Size=%d, Sha256=%s\n",
                t.ID, t.Version, t.Size, t.Sha256)
        }

    default:
        fmt.Printf("Error: Unknown operation: %s\n", *op)
        fmt.Println("Valid operations: insert-tenant, validate-tenant, list-tenants, ...")
        os.Exit(1)
    }
}
