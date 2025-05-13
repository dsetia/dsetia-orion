package main

import (
    "encoding/json"
    "flag"
    "io/ioutil"
    "log"
    "os"
    "fmt"
    "strconv"

    "orion/common"
    "github.com/google/uuid"
)

// ProvisionConfig represents provision-config.json
type ProvisionConfig struct {
    APIServerURL       string `json:"api_server_url"`
    APIServerPort      int    `json:"api_server_port"`
    CertificateSkip    bool   `json:"certificate_verify_skip"`
    SuricataConfig     string `json:"suricata_config"`
    SensorOutput       string `json:"sensor_output"`
    UpdaterOutput      string `json:"updater_output"`
    HndrOutput         string `json:"hndr_output"`
}

type ProvisionSensor struct {
    TenantName         string `json:"tenant_name"`
    DeviceName         string `json:"device_name"`
}

type ProvisionTenant struct {
    TenantName         string `json:"tenant_name"`
}

// SensorConfig represents sensor-config.json
type SensorConfig struct {
    TenantID   string `json:"tenant_id"`
    APIKey     string `json:"api_key"`
    DeviceID   string `json:"device_id"`
    LicenseKey string `json:"license_key"`
}

// UpdaterConfig represents updater-config.json (same as previous)
type UpdaterConfig struct {
    UpdateLock        string `json:"update_lock"`
    HndrConfig        string `json:"hndr_config"`
    SensorConfig      string `json:"sensor_config"`
    FolderOne         string `json:"folder_one"`
    FolderTwo         string `json:"folder_two"`
    RulesFolder       string `json:"rules_folder"`
    IDSServiceName    string `json:"ids_service_name"`
    ScratchFolder     string `json:"scratch_folder"`
    HndrSymlink       string `json:"hndr_symlink"`
    APIServerURL      string `json:"api_server_url"`
    APIServerPort     int    `json:"api_server_port"`
    CertificateSkip   bool   `json:"certificate_verify_skip"`
    PollIntervalMins  int    `json:"poll_interval_mins"`
    APIServerTimeout  int    `json:"api_server_timeout"`
    Daemonize         bool   `json:"daemonize"`
    Verbose           bool   `json:"verbose"`
}

func main() {
    // Command-line flags
    configFile := flag.String("config", "", "Path to provisioning config JSON file")
    dbPath := flag.String("db", "", "Path to postgres database config file")
    op := flag.String("op", "", "Operation to perform (e.g., provision-tenant, provision-sensor)")

    // tenant provision
    tenantName := flag.String("tenant-name", "", "Tenant name")

    // sensor provision
    deviceName := flag.String("device-name", "", "Device name")

    flag.Parse()

    if *op == "" || *configFile == "" || *dbPath == "" {
        fmt.Println("Error: -op, -config and -db flags are required")
        fmt.Println("Usage: ./provision-sensor -config <path> -db <path> -op <operation> [args]")
        fmt.Println("Operations:")
        fmt.Println("  provision-tenant")
        fmt.Println("  provision-sensor")
        os.Exit(1)
    }

    // Read config file
    configData, err := ioutil.ReadFile(*configFile)
    if err != nil {
        log.Fatalf("Failed to read config file %s: %v", *configFile, err)
    }

    // Parse config
    var config ProvisionConfig
    if err := json.Unmarshal(configData, &config); err != nil {
        log.Fatalf("Failed to parse config file %s: %v", *configFile, err)
    }

    // Open and read the DB config file
    file, err := os.Open(*dbPath)
    if err != nil {
        log.Fatalf("Error opening DB config file: %v", err)
    }
    defer file.Close()
    bytes, err := ioutil.ReadAll(file)
    if err != nil {
        log.Fatalf("Error reading DB config file: %v", err)
    }
    var cfg common.DBConfig
    if err := json.Unmarshal(bytes, &cfg); err != nil {
        log.Fatalf("Error parsing config: %v", err)
    }
    // Construct DB path
    log.Println("DB path = ", cfg.ConnString())

    db, err := NewDB(cfg.ConnString())
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    defer db.Close()

    switch *op {
    // Tenant Operations
    case "provision-tenant":
        if *tenantName == "" {
            log.Fatal("Error: -tenant-name is required for provision-tenant")
        }
        id, err := db.GetOrInsertTenant(*tenantName)
        if err != nil {
            log.Fatalf("Error: %v\n", err)
        }
        fmt.Printf("Tenant provisioned or found: ID=%d\n", id)

    // Sensor Operations
    case "provision-sensor":
        if *deviceName == "" || *tenantName == "" {
            log.Fatal("Error: -device-name and -tenant-name are required for provision-sensor")
        }

        // Step 1: Get Tenant ID
        tenantID, err := db.GetOrInsertTenant(*tenantName)
        if err != nil {
            log.Fatalf("Error: %v\n", err)
        }

        // Step 2: Create device ID
        finalDeviceID := "dev-" + uuid.New().String()[:8]
        _, err = db.GetOrInsertDevice(finalDeviceID, tenantID, *deviceName, "")
        if err != nil {
            log.Fatalf("Failed to get or insert device %s: %v", finalDeviceID, err)
        }

        // Step 3: Create API key
        finalAPIKey := "key-" + uuid.New().String()
        _, err = db.GetOrInsertAPIKey(finalAPIKey, tenantID, finalDeviceID, true)
        if err != nil {
            log.Fatalf("Failed to get or insert API key: %v", err)
        }

        // Step 4: Generate sensor-config.json
        sensorConfig := SensorConfig{
            TenantID:   strconv.FormatInt(tenantID, 10),
            APIKey:     finalAPIKey,
            DeviceID:   finalDeviceID,
            LicenseKey: "lic-" + uuid.New().String(),
        }
        sensorData, err := json.MarshalIndent(sensorConfig, "", "    ")
        if err != nil {
            log.Fatalf("Failed to marshal sensor config: %v", err)
        }
        if err := ioutil.WriteFile(config.SensorOutput, sensorData, 0644); err != nil {
            log.Fatalf("Failed to write sensor config to %s: %v", config.SensorOutput, err)
        }
        log.Printf("Generated %s successfully", config.SensorOutput)

        // Step 5: Generate updater-config.json
        templateData, err := ioutil.ReadFile("./config/updater-config-template.json")
        if err != nil {
            log.Fatalf("Failed to read updater template: %v", err)
        }
        var updaterConfig UpdaterConfig
        if err := json.Unmarshal(templateData, &updaterConfig); err != nil {
            log.Fatalf("Failed to parse updater template: %v", err)
        }

        // Apply overrides
        if config.APIServerURL != "" {
            updaterConfig.APIServerURL = config.APIServerURL
        }
        if  config.APIServerPort != 0 {
            updaterConfig.APIServerPort = config.APIServerPort
        }
        if config.CertificateSkip  {
            updaterConfig.CertificateSkip = config.CertificateSkip
        }

        // Write updater-config.json
        updaterData, err := json.MarshalIndent(updaterConfig, "", "    ")
        if err != nil {
            log.Fatalf("Failed to marshal updater config: %v", err)
        }
        if err := ioutil.WriteFile(config.UpdaterOutput, updaterData, 0644); err != nil {
            log.Fatalf("Failed to write updater config: %v", err)
        }
        log.Printf("Generated %s successfully", config.UpdaterOutput)

    default:
        fmt.Printf("Error: Unknown operation: %s\n", *op)
        fmt.Println("Valid operations: provision-tenant, provision-sensor, ...")
        os.Exit(1)
    }
}
