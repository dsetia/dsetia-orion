package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "path"
    "strconv"
    "strings"
    "github.com/hashicorp/go-version"
)

type DeviceVersions struct {
    ImageVersion      string `json:"image_version"`
    RulesVersion      string `json:"rules_version"`
    ThreatfeedVersion string `json:"threatfeed_version"`
}

// UpdateResponse represents the /v1/update response
type UpdateResponse struct {
    Software      *SoftwareVersion `json:"software,omitempty"`
    Rules         *VersionInfo     `json:"rules,omitempty"`
    ThreatIntel   *VersionInfo     `json:"threat_intel,omitempty"`
}

// SoftwareVersion includes hndr_sw details
type SoftwareVersion struct {
    Version string `json:"version"`
    Size    int64  `json:"size"`
    Sha256  string `json:"sha256"`
    Source  string `json:"source"` // "device" or "latest"
    DownloadURL string `json:"download_url"`
}

// VersionInfo includes version details for rules and threatintel
type VersionInfo struct {
    Version string `json:"version"`
    Size    int64  `json:"size"`
    Sha256  string `json:"sha256"`
    DownloadURL string `json:"download_url"`
}

// Server holds the API server state
type Server struct {
    db *DB
}

// NewServer initializes the API server
func NewServer(dbPath string) (*Server, error) {
    db, err := NewDB(dbPath)
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
        return 0, "", fmt.Errorf("failed to validate API key")
    }
    if !isActive {
        return 0, "", fmt.Errorf("inactive API key")
    }

    if keyDeviceID != deviceID {
        return 0, "", fmt.Errorf("failed to validate device id")
    }
    return tenantID, deviceID, nil
}

// handleAuthenticate handles /v1/authenticate/{tenant_id}
func (s *Server) handleAuthenticate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Extract tenant_id from URL
    tenantIDStr := path.Base(r.URL.Path)
    tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
    if err != nil {
	http.Error(w, "Unauthorized: Invalid tenant id", http.StatusBadRequest)
        return
    }

    // Authenticate
    authTenantID, _, err := s.authenticate(r)
    if err != nil {
	http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
        return
    }

    // Verify tenant_id matches
    if authTenantID != tenantID {
        http.Error(w, "Unauthorized: tenant mismatch", http.StatusUnauthorized)
        return
    }

    // Return minimal response for Nginx auth_request
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "authenticated"})
}

// handleUpdate handles /v1/updates/{tenant-id}
func (s *Server) handleUpdates(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Extract tenant_id from URL
    tenantIDStr := path.Base(r.URL.Path)
    tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
    if err != nil {
	http.Error(w, "Unauthorized: Invalid tenant id", http.StatusBadRequest)
        return
    }

    // Authenticate
    authTenantID, deviceID, err := s.authenticate(r)
    if err != nil {
	http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
        return
    }

    // Verify tenant_id matches
    if authTenantID != tenantID {
        http.Error(w, "Unauthorized: tenant mismatch", http.StatusUnauthorized)
        return
    }

    // parse request body
    var deviceVersions DeviceVersions
    if err := json.NewDecoder(r.Body).Decode(&deviceVersions); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Get device details
    var device Device
    err = s.db.QueryRow(`
        SELECT device_id, tenant_id, device_name, hndr_sw_version
        FROM devices
        WHERE device_id = ? AND tenant_id = ?
    `, deviceID, tenantID).Scan(&device.ID, &device.TenantID, &device.Name, &device.HndrSwVersion)
    if err != nil {
        http.Error(w, "Device not found", http.StatusNotFound)
        return
    }

    // Prepare response
    resp := UpdateResponse{}

    // Get software version
    var sw HndrSw
    if device.HndrSwVersion != "" {
        // Use device-specific version
        err = s.db.QueryRow(`
            SELECT id, version, size, sha256
            FROM hndr_sw
            WHERE version = ?
        `, device.HndrSwVersion).Scan(&sw.ID, &sw.Version, &sw.Size, &sw.Sha256)
        if err != nil {
            http.Error(w, "Software version not found", http.StatusNotFound)
            return
        }
    } else {
        // Use latest version
        var sw HndrSw
        err = s.db.QueryRow(`
            SELECT id, version, size, sha256
            FROM hndr_sw
            ORDER BY id DESC
            LIMIT 1
        `).Scan(&sw.ID, &sw.Version, &sw.Size, &sw.Sha256)
        if err != nil {
            http.Error(w, "No software versions available", http.StatusNotFound)
            return
        }
    }

    // Get rules version
    var rules HndrRules
    err = s.db.QueryRow(`
        SELECT id, version, size, sha256
        FROM hndr_rules
        WHERE tenant_id = ?
        ORDER BY id DESC
        LIMIT 1
    `, tenantID).Scan(&rules.ID, &rules.Version, &rules.Size, &rules.Sha256)
    if err != nil {
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
    `).Scan(&ti.ID, &ti.Version, &ti.Size, &ti.Sha256)
    if err != nil {
        http.Error(w, "No threat intelligence available", http.StatusNotFound)
        return
    }

    if isNewerNum(sw.Version, deviceVersions.ImageVersion) {
        resp.Software = &SoftwareVersion{
            Version: sw.Version,
            Size:    sw.Size,
            Sha256:  sw.Sha256,
            Source:  "device",
            DownloadURL: DownloadURLFormat(tenantID, "images", "hndr-sw", sw.Version),
        }
    }
    if isNewerNum(rules.Version, deviceVersions.RulesVersion) {
        resp.Rules = &VersionInfo{
            Version:     rules.Version,
            Size:        rules.Size,
            Sha256:      rules.Sha256,
            DownloadURL: DownloadURLFormat(tenantID, "rules", "hndr-rules", rules.Version),
        }
    }
    if isNewerNum(ti.Version, deviceVersions.ThreatfeedVersion) {
        resp.ThreatIntel = &VersionInfo{
            Version:     ti.Version,
            Size:        ti.Size,
            Sha256:      ti.Sha256,
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

func main() {
    dbPath := "/app/updater.db" // Matches Docker volume
    server, err := NewServer(dbPath)
    if err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }

    http.HandleFunc("/v1/authenticate/", server.handleAuthenticate)
    http.HandleFunc("/v1/updates/", server.handleUpdates)

    log.Println("Starting API server on :8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}
