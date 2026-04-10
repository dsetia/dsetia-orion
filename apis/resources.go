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

package main

import (
	"encoding/json"
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

// ─── /v1/ma/me ───────────────────────────────────────────────────────────────

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims == nil { // guard against misconfigured routes missing requireJWT
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":   claims.UserID,
		"email":     claims.Email,
		"role":      claims.Role,
		"tenant_id": claims.TenantID,
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

func (s *Server) handleListStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := claimsFromContext(r.Context()).TenantID
	statuses, err := s.db.ListStatusByTenant(tenantID)
	if err != nil {
		log.Printf("handleListStatus: %v", err)
		jsonError(w, "failed to list status", http.StatusInternalServerError)
		return
	}
	writeJSON(w, statuses)
}

// ─── Users ───────────────────────────────────────────────────────────────────

// handleListUsers serves GET /v1/ma/users — system_admin only.
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims.Role != "system_admin" {
		jsonError(w, "forbidden", http.StatusForbidden)
		return
	}
	users, err := s.db.ListUsers(claims.TenantID)
	if err != nil {
		log.Printf("handleListUsers: %v", err)
		jsonError(w, "failed to list users", http.StatusInternalServerError)
		return
	}
	writeJSON(w, users)
}

// handleCreateUser serves POST /v1/ma/users — system_admin only.
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims.Role != "system_admin" {
		jsonError(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" || req.Role == "" {
		jsonError(w, "email, password, and role are required", http.StatusBadRequest)
		return
	}
	if req.Role != "security_analyst" && req.Role != "system_admin" {
		jsonError(w, "role must be security_analyst or system_admin", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 12 {
		jsonError(w, "password must be at least 12 characters", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("handleCreateUser: bcrypt: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	userID, err := s.db.InsertUser(claims.TenantID, req.Email, string(hash), req.Role)
	if err != nil {
		log.Printf("handleCreateUser: InsertUser: %v", err)
		jsonError(w, "failed to create user (email may already exist)", http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":   userID,
		"email":     req.Email,
		"role":      req.Role,
		"tenant_id": claims.TenantID,
		"is_active": true,
	})
}

// handleDeleteUser serves DELETE /v1/ma/users/{user_id} — system_admin only.
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	if claims.Role != "system_admin" {
		jsonError(w, "forbidden", http.StatusForbidden)
		return
	}
	userID := r.PathValue("user_id")
	if claims.UserID == userID {
		jsonError(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}
	if err := s.db.DeleteUser(userID, claims.TenantID); err != nil {
		log.Printf("handleDeleteUser: %v", err)
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleResetPassword serves PUT /v1/ma/users/{user_id}/password.
// system_admin may reset any user; security_analyst may only reset their own.
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	userID := r.PathValue("user_id")
	if claims.Role == "security_analyst" && claims.UserID != userID {
		jsonError(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 12 {
		jsonError(w, "password must be at least 12 characters", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("handleResetPassword: bcrypt: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := s.db.ResetUserPassword(userID, claims.TenantID, string(hash)); err != nil {
		log.Printf("handleResetPassword: %v", err)
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	s.db.ClearLockout(userID) //nolint:errcheck
	w.WriteHeader(http.StatusNoContent)
}

// ─── Shared helpers ──────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}
