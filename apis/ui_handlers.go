// Copyright (c) 2025 SecurITe
// All rights reserved.
//
// This source code is the property of SecurITe.
// Unauthorized copying, modification, or distribution of this file,
// via any medium is strictly prohibited unless explicitly authorized
// by SecurITe.
//
// This software is proprietary and confidential.

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ─── /v1/ui/me ───────────────────────────────────────────────────────────────

// handleUIMe returns the calling user's identity from the JWT context.
// No DB hit required.
func (s *Server) handleUIMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims := claimsFromContext(r.Context())
	if claims == nil {
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

// ─── Catch-all dispatcher ────────────────────────────────────────────────────

// handleUITenantScoped is the catch-all handler for /v1/ui/ routes that are
// scoped to the tenant derived from the JWT.  It manually parses the path
// suffix and dispatches to the appropriate sub-handler.
//
// URL structure after stripping /v1/ui/:
//
//	parts[0] = resource      ("devices", "users", "versions", "status")
//	parts[1] = resource_id   (optional)
//	parts[2] = sub-resource  ("password", optional)
//
// The tenant_id is NEVER read from the URL.  It is obtained exclusively from
// the JWT claims via applyTenantScope → tenantIDFromContext.
func (s *Server) handleUITenantScoped(w http.ResponseWriter, r *http.Request) {
	log.Printf("UI access: method=%s path=%s client=%s", r.Method, r.URL.Path, r.RemoteAddr)

	trimmed := strings.TrimPrefix(r.URL.Path, "/v1/ui/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	parts := strings.SplitN(trimmed, "/", 3)

	if len(parts) < 1 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	resource := parts[0]

	ctx, ok := s.applyTenantScope(w, r)
	if !ok {
		return
	}
	r = r.WithContext(ctx)

	switch resource {
	case "devices":
		switch {
		case len(parts) == 1 && r.Method == http.MethodGet:
			s.handleUIListDevices(w, r)
		case len(parts) == 2 && r.Method == http.MethodGet:
			s.handleUIGetDevice(w, r, parts[1])
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}

	case "versions":
		switch {
		case len(parts) == 1 && r.Method == http.MethodGet:
			s.handleUIListVersions(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}

	case "status":
		switch {
		case len(parts) == 1 && r.Method == http.MethodGet:
			s.handleUIListStatus(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}

	case "users":
		s.handleUIUsers(w, r, parts)

	default:
		http.NotFound(w, r)
	}
}

// ─── Devices ─────────────────────────────────────────────────────────────────

func (s *Server) handleUIListDevices(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromContext(r.Context())
	devices, err := s.db.ListDevices(tenantID)
	if err != nil {
		log.Printf("handleUIListDevices: %v", err)
		jsonError(w, "failed to list devices", http.StatusInternalServerError)
		return
	}
	writeJSON(w, devices)
}

func (s *Server) handleUIGetDevice(w http.ResponseWriter, r *http.Request, deviceID string) {
	tenantID := tenantIDFromContext(r.Context())
	device, err := s.db.GetDeviceEntry(deviceID, tenantID)
	if err != nil {
		jsonError(w, "device not found", http.StatusNotFound)
		return
	}
	writeJSON(w, device)
}

// ─── Versions ────────────────────────────────────────────────────────────────

func (s *Server) handleUIListVersions(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromContext(r.Context())
	versions, err := s.db.ListVersionsByTenant(tenantID)
	if err != nil {
		log.Printf("handleUIListVersions: %v", err)
		jsonError(w, "failed to list versions", http.StatusInternalServerError)
		return
	}
	writeJSON(w, versions)
}

// ─── Status ──────────────────────────────────────────────────────────────────

func (s *Server) handleUIListStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromContext(r.Context())
	statuses, err := s.db.ListStatusByTenant(tenantID)
	if err != nil {
		log.Printf("handleUIListStatus: %v", err)
		jsonError(w, "failed to list status", http.StatusInternalServerError)
		return
	}
	writeJSON(w, statuses)
}

// ─── Users ───────────────────────────────────────────────────────────────────

// handleUIUsers dispatches user-management operations.
// Role enforcement is per-operation:
//   - GET list, POST create, DELETE: system_admin only.
//   - PUT password: any role, but security_analyst may only reset their own password.
func (s *Server) handleUIUsers(w http.ResponseWriter, r *http.Request, parts []string) {
	claims := claimsFromContext(r.Context())
	tenantID := tenantIDFromContext(r.Context())

	switch {
	// GET /v1/ui/users — list users (system_admin only)
	case len(parts) == 1 && r.Method == http.MethodGet:
		if claims.Role != "system_admin" {
			jsonError(w, "forbidden", http.StatusForbidden)
			return
		}
		users, err := s.db.ListUIUsers(tenantID)
		if err != nil {
			log.Printf("handleUIUsers list: %v", err)
			jsonError(w, "failed to list users", http.StatusInternalServerError)
			return
		}
		writeJSON(w, users)

	// POST /v1/ui/users — create user (system_admin only)
	case len(parts) == 1 && r.Method == http.MethodPost:
		if claims.Role != "system_admin" {
			jsonError(w, "forbidden", http.StatusForbidden)
			return
		}
		s.handleUICreateUser(w, r, tenantID)

	// DELETE /v1/ui/users/{user_id} — delete user (system_admin only)
	case len(parts) == 2 && r.Method == http.MethodDelete:
		if claims.Role != "system_admin" {
			jsonError(w, "forbidden", http.StatusForbidden)
			return
		}
		s.handleUIDeleteUser(w, r, parts[1], tenantID)

	// PUT /v1/ui/users/{user_id}/password — reset password
	// system_admin: any user; security_analyst: own account only
	case len(parts) == 3 && parts[2] == "password" && r.Method == http.MethodPut:
		targetUserID := parts[1]
		if claims.Role == "security_analyst" && claims.UserID != targetUserID {
			jsonError(w, "forbidden", http.StatusForbidden)
			return
		}
		s.handleUIResetPassword(w, r, targetUserID, tenantID)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUICreateUser(w http.ResponseWriter, r *http.Request, tenantID int64) {
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
		log.Printf("handleUICreateUser: bcrypt: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	userID, err := s.db.InsertUIUser(tenantID, req.Email, string(hash), req.Role)
	if err != nil {
		log.Printf("handleUICreateUser: InsertUIUser: %v", err)
		jsonError(w, "failed to create user (email may already exist)", http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":   userID,
		"email":     req.Email,
		"role":      req.Role,
		"tenant_id": tenantID,
		"is_active": true,
	})
}

func (s *Server) handleUIDeleteUser(w http.ResponseWriter, r *http.Request, userID string, tenantID int64) {
	claims := claimsFromContext(r.Context())
	// Prevent self-deletion.
	if claims.UserID == userID {
		jsonError(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}
	if err := s.db.DeleteUIUser(userID, tenantID); err != nil {
		log.Printf("handleUIDeleteUser: %v", err)
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUIResetPassword(w http.ResponseWriter, r *http.Request, userID string, tenantID int64) {
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
		log.Printf("handleUIResetPassword: bcrypt: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := s.db.ResetUIUserPassword(userID, tenantID, string(hash)); err != nil {
		log.Printf("handleUIResetPassword: %v", err)
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	// Clear any active lockout when a password is reset by an admin.
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
