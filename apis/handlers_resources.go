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
// Created On:       04/10/2026
//
// All /v1/ma/ resource handlers; role enforcement per operation

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"orion/common"
	"time"
)

// ─── /v1/ma/me ───────────────────────────────────────────────────────────────

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims == nil { // guard against misconfigured routes missing requireJWT
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	tenantName, err := s.db.GetTenantName(claims.TenantID)
	if err != nil {
		log.Printf("handleMe: GetTenantName: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":     claims.UserID,
		"email":       claims.Email,
		"role":        claims.Role,
		"tenant_id":   claims.TenantID,
		"tenant_name": tenantName,
	})
}

// ─── Devices ─────────────────────────────────────────────────────────────────

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	tenantID := claimsFromContext(r.Context()).TenantID
	devices, err := s.db.ListDevices(tenantID)
	if err != nil {
		log.Printf("handleListDevices: %v", err)
		jsonError(w, "failed to list devices", http.StatusInternalServerError)
		return
	}
	writeJSON(w, devices)
}

func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	tenantID := claimsFromContext(r.Context()).TenantID
	deviceID := r.PathValue("device_id")
	device, err := s.db.GetDeviceEntry(deviceID, tenantID)
	if err != nil {
		jsonError(w, "device not found", http.StatusNotFound)
		return
	}
	writeJSON(w, device)
}

// ─── Versions ────────────────────────────────────────────────────────────────

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	tenantID := claimsFromContext(r.Context()).TenantID
	versions, err := s.db.ListVersionsByTenant(tenantID)
	if err != nil {
		log.Printf("handleListVersions: %v", err)
		jsonError(w, "failed to list versions", http.StatusInternalServerError)
		return
	}
	writeJSON(w, versions)
}

// ─── Status ──────────────────────────────────────────────────────────────────

// StatusResponse is the API-level status response; it extends the DB Status
// row with a liveness field derived from the version table.
type StatusResponse struct {
	DeviceID    string    `json:"device_id"`
	TenantID    int64     `json:"tenant_id"`
	Software    string    `json:"software"`
	Rules       string    `json:"rules"`
	ThreatIntel string    `json:"threatintel"`
	UpdatedAt   time.Time `json:"updated_at"`
	Liveness    string    `json:"liveness"`
}

func (s *Server) handleListStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := claimsFromContext(r.Context()).TenantID

	statuses, err := s.db.ListStatusByTenant(tenantID)
	if err != nil {
		log.Printf("handleListStatus: ListStatusByTenant: %v", err)
		jsonError(w, "failed to list status", http.StatusInternalServerError)
		return
	}

	versions, err := s.db.ListVersionsByTenant(tenantID)
	if err != nil {
		log.Printf("handleListStatus: ListVersionsByTenant: %v", err)
		jsonError(w, "failed to list status", http.StatusInternalServerError)
		return
	}

	// Build device_id → last-poll-time index from the version table.
	lastSeen := make(map[string]*time.Time, len(versions))
	for i := range versions {
		t := versions[i].UpdatedAt
		lastSeen[versions[i].DeviceID] = &t
	}

	result := make([]StatusResponse, 0, len(statuses))
	for _, st := range statuses {
		result = append(result, StatusResponse{
			DeviceID:    st.DeviceID,
			TenantID:    st.TenantID,
			Software:    st.Software,
			Rules:       st.Rules,
			ThreatIntel: st.ThreatIntel,
			UpdatedAt:   st.UpdatedAt,
			Liveness:    common.DeviceLiveness(lastSeen[st.DeviceID]),
		})
	}

	writeJSON(w, result)
}
