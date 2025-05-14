package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "log"
    "io/ioutil"

    "orion/common"
    _ "github.com/lib/pq"
)


// Command-line interface
func main() {
    // Common flags
    configPath := flag.String("db", "", "Path to postgres database config file")
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

    // Status flags
    sSoftware := flag.String("status-software", "", "Software status")
    sRules := flag.String("status-rules", "", "Rules status")
    sThreatIntel := flag.String("status-threatintel", "", "ThreatIntel status")

    flag.Parse()

    if *op == "" || *configPath == "" {
        fmt.Println("Error: -op and -db flags are required")
        fmt.Println("Usage: ./dbutil -db <path> -op <operation> [args]")
        fmt.Println("Operations:")
        fmt.Println("  insert-tenant, validate-tenant, list-tenants")
        fmt.Println("  insert-device, validate-device, list-devices")
        fmt.Println("  insert-api-key, validate-api-key, list-api-keys")
        fmt.Println("  insert-hndr-sw, validate-hndr-sw, list-hndr-sw")
        fmt.Println("  insert-hndr-rules, validate-hndr-rules, list-hndr-rules")
        fmt.Println("  insert-threat-intel, validate-threat-intel, list-threat-intel")
        fmt.Println("  insert-status, list-status")
        os.Exit(1)
    }

    // Open and read the config file
    file, err := os.Open(*configPath)
    if err != nil {
        log.Fatalf("Error opening config file: %v", err)
    }
    defer file.Close()
    bytes, err := ioutil.ReadAll(file)
    if err != nil {
        log.Fatalf("Error reading config file: %v", err)
    }
    var cfg common.DBConfig
    if err := json.Unmarshal(bytes, &cfg); err != nil {
        log.Fatalf("Error parsing config: %v", err)
    }

    // Construct DB path
    dbPath := cfg.ConnString()
    log.Println("DB path = ", dbPath)

    db, err := NewDB(dbPath)
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
        if *deviceName == "" || *tenantID == 0 {
            fmt.Println("Error: -device-name and -tenant-id are required for insert-device")
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
        if *tenantID == 0 || *deviceID == "" {
            fmt.Println("Error: -tenant-id, and -device-id are required for insert-api-key")
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

    // Status Operations
    case "insert-status":
        if *deviceID == "" || *tenantID == 0 || (*sSoftware == "" && *sRules == "" && *sThreatIntel == "") {
            fmt.Println("Error: -device-id and -tenant-id are required for insert-status")
            os.Exit(1)
        }
        err := db.InsertStatus(*deviceID, *tenantID, *sSoftware, *sRules, *sThreatIntel)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        fmt.Printf("Status inserted\n")

    case "list-status":
        statusList, err := db.ListStatus()
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            os.Exit(1)
        }
        for _, d := range statusList {
            fmt.Printf("Device: ID=%s, TenantID=%d, Software=%s, Rules=%s, ThreatIntel=%s\n",
                d.DeviceID, d.TenantID, d.Software, d.Rules, d.ThreatIntel)
        }

    default:
        fmt.Printf("Error: Unknown operation: %s\n", *op)
        fmt.Println("Valid operations: insert-tenant, validate-tenant, list-tenants, ...")
        os.Exit(1)
    }
}
