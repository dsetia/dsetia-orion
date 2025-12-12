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
    "encoding/json"
    "os"
    "fmt"
    "log"
    "net/http"
    "flag"
    "path"
    "io/ioutil"
    "strconv"
    "strings"
    "orion/common"
    "github.com/hashicorp/go-version"
)


// Server holds the API server state
type Server struct {
    db *DB
}

// DownloadURLFormat generates a download URL for the given tenant ID, type, prefix, and version.
// resourceType is software, rules, or threatintel
// prefix is hndr-sw, hndr-rules, or threatintel
// Returns string like /v1/download/1/software/hndr-sw-v1.2.3.tar.gz
func DownloadURLFormat(tenantID int64, resourceType, prefix, version string) string {
    return fmt.Sprintf("/v1/download/%d/%s/%s-%s.tar.gz", tenantID, resourceType, prefix, version)
}
func DownloadURLFormatRules(tenantID int64, resourceType, prefix, version string) string {
    return fmt.Sprintf("/v1/download/%d/%s/%d/%s-tid_%d-%s.tar.gz", tenantID, resourceType, tenantID, prefix, tenantID, version)
}

// NewServer initializes the API server
func NewServer(dbPath string, environment string) (*Server, error) {
    db, err := NewDB(dbPath, environment)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize database: %w", err)
    }
    return &Server{db: db}, nil
}

// authenticate checks X-API-KEY and X-DEVICE-ID headers
func (s *Server) authenticate(r *http.Request) (int64, string, error) {
    apiKey := r.Header.Get("X-API-KEY")
    deviceID := r.Header.Get("X-DEVICE-ID")
    if apiKey == "" || deviceID == "" {
        return 0, "", fmt.Errorf("missing API key")
    }

    // Validate API key
    isActive, tenantID, keyDeviceID, err := s.db.ValidateAPIKey(apiKey)
    if err != nil {
	log.Printf("api key = %s, device id = %s, tenant id = %d", apiKey, deviceID, tenantID)
        return 0, "", fmt.Errorf("failed to validate API key")
    }
    if !isActive {
	log.Printf("api key = %s, device id = %s, tenant id = %d", apiKey, deviceID, tenantID)
        return 0, "", fmt.Errorf("inactive API key")
    }

    if keyDeviceID != deviceID {
	log.Printf("api key = %s, device id = %s, tenant id = %d", apiKey, deviceID, tenantID)
        return 0, "", fmt.Errorf("failed to validate device id")
    }
    return tenantID, deviceID, nil
}

// handleAuthenticate handles /v1/authenticate/{tenant_id}
func (s *Server) handleAuthenticate(w http.ResponseWriter, r *http.Request) {
    log.Printf("API access: method=%s, path=%s, client_ip=%s", r.Method, r.URL.Path, r.RemoteAddr)
    if r.Method != http.MethodGet {
        log.Printf("Method not allowed")
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Extract tenant_id from URL
    tenantIDStr := path.Base(r.URL.Path)
    tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
    if err != nil {
	log.Printf("Unauthorized: Invalid tenant id %s", tenantIDStr)
	http.Error(w, "Unauthorized: Invalid tenant id", http.StatusBadRequest)
        return
    }

    // Authenticate
    authTenantID, _, err := s.authenticate(r)
    if err != nil {
	log.Printf("Unauthorized: "+err.Error())
	http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
        return
    }

    // Verify tenant_id matches
    if authTenantID != tenantID {
        log.Printf("Unauthorized: tenant mismatch, tenant_id=%d, auth_tenant_id=%d", tenantID, authTenantID)
        http.Error(w, "Unauthorized: tenant mismatch", http.StatusUnauthorized)
        return
    }

    // Return minimal response for Nginx auth_request
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "authenticated"})
}

// handleUpdate handles /v1/updates/{tenant-id}
func (s *Server) handleUpdates(w http.ResponseWriter, r *http.Request) {
    log.Printf("API access: method=%s, path=%s, client_ip=%s", r.Method, r.URL.Path, r.RemoteAddr)
    if r.Method != http.MethodPost {
        log.Printf("Method not allowed")
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Extract tenant_id from URL
    tenantIDStr := path.Base(r.URL.Path)
    tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
    if err != nil {
	log.Printf("Unauthorized: Invalid tenant id %s", tenantIDStr)
	http.Error(w, "Unauthorized: Invalid tenant id", http.StatusBadRequest)
        return
    }

    // Authenticate
    authTenantID, deviceID, err := s.authenticate(r)
    if err != nil {
	log.Printf("Unauthorized: "+err.Error())
	http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
        return
    }

    // Verify tenant_id matches
    if authTenantID != tenantID {
        log.Printf("Unauthorized: tenant mismatch, tenant_id=%d, auth_tenant_id=%d", tenantID, authTenantID)
        http.Error(w, "Unauthorized: tenant mismatch", http.StatusUnauthorized)
        return
    }

    // parse request body
    var deviceVersions common.DeviceVersions
    if err := json.NewDecoder(r.Body).Decode(&deviceVersions); err != nil {
        log.Print("Invalid request body")
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Get device details
    var device Device
    err = s.db.QueryRow(`
        SELECT device_id, tenant_id, device_name, hndr_sw_version
        FROM devices
        WHERE device_id = $1 AND tenant_id = $2
    `, deviceID, tenantID).Scan(&device.ID, &device.TenantID, &device.Name, &device.HndrSwVersion)
    if err != nil {
	log.Print("Device ID not found: "+deviceID)
        http.Error(w, "Device not found", http.StatusNotFound)
        return
    }

    // Prepare response
    resp := common.UpdateResponse{}

    // Get software version
    var sw HndrSw
    var source string = "latest"
    if device.HndrSwVersion != "" {
        // Use device-specific version
        err = s.db.QueryRow(`
            SELECT id, version, size, sha256
            FROM hndr_sw
            WHERE version = $1
        `, device.HndrSwVersion).Scan(&sw.ID, &sw.Version, &sw.Size, &sw.Digest)
        if err != nil {
	    log.Print("Software version not found: " + device.HndrSwVersion)
            http.Error(w, "Software version not found", http.StatusNotFound)
            return
        }
	source = "device";
    } else {
        // Use latest version
        err = s.db.QueryRow(`
            SELECT id, version, size, sha256
            FROM hndr_sw
            ORDER BY id DESC
            LIMIT 1
        `).Scan(&sw.ID, &sw.Version, &sw.Size, &sw.Digest)
        if err != nil {
            log.Print("No software versions available")
            http.Error(w, "No software versions available", http.StatusNotFound)
            return
        }
    }

    // Get rules version
    var rules HndrRules
    err = s.db.QueryRow(`
        SELECT id, version, size, sha256
        FROM hndr_rules
        WHERE tenant_id = $1
        ORDER BY id DESC
        LIMIT 1
    `, tenantID).Scan(&rules.ID, &rules.Version, &rules.Size, &rules.Digest)
    if err != nil {
        log.Print("No rules available for tenant")
        http.Error(w, "No rules available for tenant", http.StatusNotFound)
        return
    }

    // Get threat intelligence version
    var ti ThreatIntel
    err = s.db.QueryRow(`
        SELECT id, version, size, sha256
        FROM threatintel
        ORDER BY id DESC
        LIMIT 1
    `).Scan(&ti.ID, &ti.Version, &ti.Size, &ti.Digest)
    if err != nil {
        log.Print("No threat intelligence available")
        http.Error(w, "No threat intelligence available", http.StatusNotFound)
        return
    }

    if isUpdateNeeded(sw.Version, deviceVersions.Software.Version, sw.Digest, deviceVersions.Software.Digest) {
        resp.Software = &common.SoftwareVersion{
            Version: sw.Version,
            Size:    sw.Size,
            Digest:  sw.Digest,
            Source:  source,
            DownloadURL: DownloadURLFormat(tenantID, "software", "hndr-sw", sw.Version),
        }
    }
    if isUpdateNeeded(rules.Version, deviceVersions.Rules.Version, rules.Digest, deviceVersions.Rules.Digest) {
        resp.Rules = &common.VersionInfo{
            Version:     rules.Version,
            Size:        rules.Size,
            Digest:      rules.Digest,
            DownloadURL: DownloadURLFormatRules(tenantID, "rules", "hndr-rules", rules.Version),
        }
    }
    if isUpdateNeeded(ti.Version, deviceVersions.ThreatIntel.Version, ti.Digest, deviceVersions.ThreatIntel.Digest) {
        resp.ThreatIntel = &common.VersionInfo{
            Version:     ti.Version,
            Size:        ti.Size,
            Digest:      ti.Digest,
            DownloadURL: DownloadURLFormat(tenantID, "threatintel", "threatintel", ti.Version),
        }
    }


    // Return response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(resp)
}

// isNewerLex compares two version strings lexographically (v1.2.10 < v1.2.3)
func isNewerLex(manifestVersion, deviceVersion string) bool {
    return manifestVersion != "" && deviceVersion != "" && manifestVersion > deviceVersion
}

// isNewerNum compares two version strings numerically (v1.2.10 > v1.2.3)
// threatfeed should use timestamp in the format YYYY.MM.DD.HHMMSS
func isNewerNum(manifestVersion, deviceVersion string) bool {
    // force update if version missing from device
    if deviceVersion == "" {
        return true
    }
    dvTrimmed := strings.TrimLeft(deviceVersion, "vr")
    mvTrimmed := strings.TrimLeft(manifestVersion, "vr")

    vDevice, err    := version.NewVersion(dvTrimmed)
    if err != nil {
	return false
    }
    vManifest, err := version.NewVersion(mvTrimmed)
    if err != nil {
	return false
    }
    return vManifest.GreaterThan(vDevice)
}

func isUpdateNeeded(manifestVersion, deviceVersion, manifestDigest, deviceDigest string) bool {
    // force update if version missing from device
    if deviceVersion == "" {
        return true
    }
    dvTrimmed := strings.TrimLeft(deviceVersion, "vr")
    mvTrimmed := strings.TrimLeft(manifestVersion, "vr")

    vDevice, err    := version.NewVersion(dvTrimmed)
    if err != nil {
	return false
    }
    vManifest, err := version.NewVersion(mvTrimmed)
    if err != nil {
	return false
    }
    if vManifest.GreaterThan(vDevice) {
        return true
    }
    if vManifest.Equal(vDevice) {
        return manifestDigest != deviceDigest
    }
    return false
}

// handleHealthCheck handles /v1/healthcheck
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
    log.Printf("API access: method=%s, path=%s, client_ip=%s", r.Method, r.URL.Path, r.RemoteAddr)
    if r.Method != http.MethodGet {
        log.Printf("Method not allowed")
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Return response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleStatus handles /v1/status/{tenant-id}
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
    log.Printf("API access: method=%s, path=%s, client_ip=%s", r.Method, r.URL.Path, r.RemoteAddr)
    if r.Method != http.MethodPost && r.Method != http.MethodGet {
        log.Printf("Method not allowed")
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Extract tenant_id from URL
    tenantIDStr := path.Base(r.URL.Path)
    tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
    if err != nil {
	log.Printf("Unauthorized: Invalid tenant id %s", tenantIDStr)
	http.Error(w, "Unauthorized: Invalid tenant id", http.StatusBadRequest)
        return
    }

    // Authenticate
    authTenantID, deviceID, err := s.authenticate(r)
    if err != nil {
	log.Printf("Unauthorized: "+err.Error())
	http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
        return
    }

    // Verify tenant_id matches
    if authTenantID != tenantID {
        log.Printf("Unauthorized: tenant mismatch, tenant_id=%d, auth_tenant_id=%d", tenantID, authTenantID)
        http.Error(w, "Unauthorized: tenant mismatch", http.StatusUnauthorized)
        return
    }

    if r.Method == http.MethodGet {
	var resp common.DeviceStatus
	Status, err := s.db.GetStatus(deviceID, tenantID)
	if err != nil {
	    log.Print("Device not found: " + deviceID)
            http.Error(w, "Device not found", http.StatusNotFound)
	    return
	}
	resp.Software.Status = Status.Software
	resp.Rules.Status = Status.Rules
	resp.ThreatIntel.Status = Status.ThreatIntel
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(resp)
        return
    }


    // parse request body
    var req common.DeviceStatus
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        log.Print("Invalid request body")
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    err = s.db.InsertStatus(deviceID, tenantID, req.Software.Status, req.Rules.Status, req.ThreatIntel.Status)
    if err != nil {
	log.Print("Error in inserting status: " + err.Error())
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Return response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {

    // Command line flag for config path
    configPath := flag.String("config", "config.json", "Path to config file")
    flag.Parse()

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
    dbPath := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
        cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
    )

    log.Println("DB path = ", dbPath)
    server, err := NewServer(dbPath, cfg.GetEnvironment())
    if err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }

    http.HandleFunc("/v1/authenticate/", server.handleAuthenticate)
    http.HandleFunc("/v1/updates/", server.handleUpdates)
    http.HandleFunc("/v1/healthcheck", server.handleHealthCheck)
    http.HandleFunc("/v1/status/", server.handleStatus)

    log.Println("Starting API server on :8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}
