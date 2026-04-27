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
// Created On:       04/14/2026
//
// Sensor-facing API handlers: authenticate, updates, status, healthcheck.
// These endpoints are consumed by the updater daemon running on each sensor.

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"

	"orion/common"

	goversion "github.com/hashicorp/go-version"
)

// ─── Authentication ───────────────────────────────────────────────────────────

// authenticate validates X-API-KEY and X-DEVICE-ID headers against the DB.
// Returns (tenantID, deviceID, error).
func (s *Server) authenticate(r *http.Request) (int64, string, error) {
	apiKey := r.Header.Get("X-API-KEY")
	deviceID := r.Header.Get("X-DEVICE-ID")
	if apiKey == "" || deviceID == "" {
		return 0, "", fmt.Errorf("missing API key")
	}

	isActive, tenantID, keyDeviceID, err := s.db.ValidateAPIKey(apiKey)
	if err != nil {
		log.Printf("authenticate: api_key=%s device_id=%s tenant_id=%d", apiKey, deviceID, tenantID)
		return 0, "", fmt.Errorf("failed to validate API key")
	}
	if !isActive {
		log.Printf("authenticate: inactive key api_key=%s device_id=%s tenant_id=%d", apiKey, deviceID, tenantID)
		return 0, "", fmt.Errorf("inactive API key")
	}
	if keyDeviceID != deviceID {
		log.Printf("authenticate: device mismatch api_key=%s device_id=%s tenant_id=%d", apiKey, deviceID, tenantID)
		return 0, "", fmt.Errorf("failed to validate device id")
	}
	return tenantID, deviceID, nil
}

// handleAuthenticate serves GET /v1/authenticate/{tenant_id}.
// Used by Nginx auth_request to gate /v1/download/* proxying to MinIO.
func (s *Server) handleAuthenticate(w http.ResponseWriter, r *http.Request) {
	log.Printf("API access: method=%s path=%s client=%s", r.Method, r.URL.Path, r.RemoteAddr)
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantIDStr := path.Base(r.URL.Path)
	tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
	if err != nil {
		log.Printf("handleAuthenticate: invalid tenant_id %q", tenantIDStr)
		http.Error(w, "Unauthorized: invalid tenant id", http.StatusBadRequest)
		return
	}

	authTenantID, _, err := s.authenticate(r)
	if err != nil {
		log.Printf("handleAuthenticate: %v", err)
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if authTenantID != tenantID {
		log.Printf("handleAuthenticate: tenant mismatch want=%d got=%d", tenantID, authTenantID)
		http.Error(w, "Unauthorized: tenant mismatch", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "authenticated"}) //nolint:errcheck
}

// handleUpdates serves POST /v1/updates/{tenant_id}.
// Receives the device's current versions and returns an update manifest.
func (s *Server) handleUpdates(w http.ResponseWriter, r *http.Request) {
	log.Printf("API access: method=%s path=%s client=%s", r.Method, r.URL.Path, r.RemoteAddr)
	s.logRequestBody(r)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantIDStr := path.Base(r.URL.Path)
	tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
	if err != nil {
		log.Printf("handleUpdates: invalid tenant_id %q", tenantIDStr)
		http.Error(w, "Unauthorized: invalid tenant id", http.StatusBadRequest)
		return
	}

	authTenantID, deviceID, err := s.authenticate(r)
	if err != nil {
		log.Printf("handleUpdates: %v", err)
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		return
	}
	if authTenantID != tenantID {
		log.Printf("handleUpdates: tenant mismatch want=%d got=%d", tenantID, authTenantID)
		http.Error(w, "Unauthorized: tenant mismatch", http.StatusUnauthorized)
		return
	}

	var deviceVersions common.DeviceVersions
	if err := json.NewDecoder(r.Body).Decode(&deviceVersions); err != nil {
		log.Print("handleUpdates: invalid request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.db.InsertVersions(deviceID, tenantID,
		deviceVersions.Software.Version,
		deviceVersions.Rules.Version,
		deviceVersions.ThreatIntel.Version); err != nil {
		log.Printf("handleUpdates: InsertVersions device=%s: %v", deviceID, err)
		http.Error(w, "Error updating version", http.StatusNotFound)
		return
	}

	var device Device
	var swVersion sql.NullString
	if err := s.db.QueryRow(`
		SELECT device_id, tenant_id, device_name, hndr_sw_version
		FROM devices
		WHERE device_id = $1 AND tenant_id = $2
	`, deviceID, tenantID).Scan(&device.ID, &device.TenantID, &device.Name, &swVersion); err != nil {
		log.Printf("handleUpdates: device not found device=%s", deviceID)
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}
	device.HndrSwVersion = swVersion.String

	resp := common.UpdateResponse{}

	var sw HndrSw
	source := "latest"
	if device.HndrSwVersion != "" {
		if err := s.db.QueryRow(`
			SELECT id, version, size, sha256
			FROM hndr_sw WHERE version = $1
		`, device.HndrSwVersion).Scan(&sw.ID, &sw.Version, &sw.Size, &sw.Digest); err != nil {
			log.Printf("handleUpdates: software version not found %s", device.HndrSwVersion)
			http.Error(w, "Software version not found", http.StatusNotFound)
			return
		}
		source = "device"
	} else {
		if err := s.db.QueryRow(`
			SELECT id, version, size, sha256
			FROM hndr_sw ORDER BY id DESC LIMIT 1
		`).Scan(&sw.ID, &sw.Version, &sw.Size, &sw.Digest); err != nil {
			log.Print("handleUpdates: no software versions available")
			http.Error(w, "No software versions available", http.StatusNotFound)
			return
		}
	}

	var rules HndrRules
	if err := s.db.QueryRow(`
		SELECT id, version, size, sha256
		FROM hndr_rules WHERE tenant_id = $1 ORDER BY id DESC LIMIT 1
	`, tenantID).Scan(&rules.ID, &rules.Version, &rules.Size, &rules.Digest); err != nil {
		log.Print("handleUpdates: no rules available for tenant")
		http.Error(w, "No rules available for tenant", http.StatusNotFound)
		return
	}

	var ti ThreatIntel
	if err := s.db.QueryRow(`
		SELECT id, version, size, sha256
		FROM threatintel ORDER BY id DESC LIMIT 1
	`).Scan(&ti.ID, &ti.Version, &ti.Size, &ti.Digest); err != nil {
		log.Print("handleUpdates: no threat intelligence available")
		http.Error(w, "No threat intelligence available", http.StatusNotFound)
		return
	}

	if isUpdateNeeded(sw.Version, deviceVersions.Software.Version, sw.Digest, deviceVersions.Software.Digest) {
		resp.Software = &common.SoftwareVersion{
			Version:     sw.Version,
			Size:        sw.Size,
			Digest:      sw.Digest,
			Source:      source,
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// handleStatus serves GET and POST /v1/status/{tenant_id}.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	log.Printf("API access: method=%s path=%s client=%s", r.Method, r.URL.Path, r.RemoteAddr)
	s.logRequestBody(r)
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantIDStr := path.Base(r.URL.Path)
	tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
	if err != nil {
		log.Printf("handleStatus: invalid tenant_id %q", tenantIDStr)
		http.Error(w, "Unauthorized: invalid tenant id", http.StatusBadRequest)
		return
	}

	authTenantID, deviceID, err := s.authenticate(r)
	if err != nil {
		log.Printf("handleStatus: %v", err)
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		return
	}
	if authTenantID != tenantID {
		log.Printf("handleStatus: tenant mismatch want=%d got=%d", tenantID, authTenantID)
		http.Error(w, "Unauthorized: tenant mismatch", http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodGet {
		status, err := s.db.GetStatus(deviceID, tenantID)
		if err != nil {
			log.Printf("handleStatus: device not found device=%s", deviceID)
			http.Error(w, "Device not found", http.StatusNotFound)
			return
		}
		var resp common.DeviceStatus
		resp.Software.Status = status.Software
		resp.Rules.Status = status.Rules
		resp.ThreatIntel.Status = status.ThreatIntel
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
		return
	}

	var req common.DeviceStatus
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Print("handleStatus: invalid request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.db.InsertStatus(deviceID, tenantID, req.Software.Status, req.Rules.Status, req.ThreatIntel.Status); err != nil {
		log.Printf("handleStatus: InsertStatus: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
}

// handleHealthCheck serves GET /v1/healthcheck.
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	log.Printf("API access: method=%s path=%s client=%s", r.Method, r.URL.Path, r.RemoteAddr)
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
}

// ─── Version comparison helpers ───────────────────────────────────────────────

func isUpdateNeeded(manifestVersion, deviceVersion, manifestDigest, deviceDigest string) bool {
	if deviceVersion == "" {
		return true
	}
	vDevice, err := goversion.NewVersion(strings.TrimLeft(deviceVersion, "vr"))
	if err != nil {
		return false
	}
	vManifest, err := goversion.NewVersion(strings.TrimLeft(manifestVersion, "vr"))
	if err != nil {
		return false
	}
	if vManifest.GreaterThan(vDevice) {
		return true
	}
	// Only compare digests when versions are equal AND the sensor actually
	// reported a digest. A missing device digest means the sensor is not
	// tracking integrity, so we rely solely on the version number.
	if vManifest.Equal(vDevice) && deviceDigest != "" {
		return manifestDigest != deviceDigest
	}
	return false
}
