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
// Server struct, initialisation, route wiring, and shared HTTP helpers.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"orion/common"
)

// Server holds the API server state.
type Server struct {
	db         *DB
	authConfig common.AuthConfig
}

// NewServer initialises the database connection and returns a ready Server.
func NewServer(dbPath string, environment string, authCfg common.AuthConfig) (*Server, error) {
	db, err := NewDB(dbPath, environment)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	return &Server{db: db, authConfig: authCfg}, nil
}

// newMux builds and returns the production ServeMux with all routes registered.
func (s *Server) newMux() *http.ServeMux {
	mux := http.NewServeMux()
	s.registerNetworkAgentRoutes(mux)
	s.registerAuthRoutes(mux)
	s.registerUserRoutes(mux)
	s.registerResourceRoutes(mux)
	return mux
}

// ─── Download URL helpers ─────────────────────────────────────────────────────

// DownloadURLFormat returns the download path for software or threatintel.
// resourceType is "software" or "threatintel"; prefix is "hndr-sw" or "threatintel".
func DownloadURLFormat(tenantID int64, resourceType, prefix, version string) string {
	return fmt.Sprintf("/v1/download/%d/%s/%s-%s.tar.gz", tenantID, resourceType, prefix, version)
}

// DownloadURLFormatRules returns the download path for per-tenant rule packages.
func DownloadURLFormatRules(tenantID int64, resourceType, prefix, version string) string {
	return fmt.Sprintf("/v1/download/%d/%s/%d/%s-tid_%d-%s.tar.gz",
		tenantID, resourceType, tenantID, prefix, tenantID, version)
}

// ─── HTTP helpers ─────────────────────────────────────────────────────────────

// writeJSON writes a 200 OK JSON response.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}

// respondJSON writes a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// logRequestBody logs the request body for debugging, then resets it so
// downstream handlers can read it again.
func (s *Server) logRequestBody(r *http.Request) {
	if r.Body == nil {
		return
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("logRequestBody: %v", err)
		return
	}
	if len(bodyBytes) > 0 {
		log.Printf("Request body: %s", string(bodyBytes))
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
}
