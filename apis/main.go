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
// Entry point: loads config, wires up the server, starts the HTTP listener.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"orion/common"
)

// ─── Server ───────────────────────────────────────────────────────────────────

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

// clientIP extracts the real client IP from the request. When running behind
// nginx, the real IP is in X-Real-IP (set by nginx from $remote_addr).
// X-Forwarded-For is checked as a fallback (first entry = originating client).
// r.RemoteAddr is the last resort, used when there is no proxy (e.g. tests).
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		// X-Forwarded-For may be "client, proxy1, proxy2"; take the first.
		if i := strings.IndexByte(fwd, ','); i != -1 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	// Strip port from r.RemoteAddr ("host:port" or "[::1]:port").
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
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

// ─── Entry point ──────────────────────────────────────────────────────────────

func main() {
	configPath     := flag.String("config",      "config.json", "Path to DB config file")
	authConfigPath := flag.String("config-auth", "auth.json",   "Path to auth config file")
	flag.Parse()

	// Load DB config.
	dbFile, err := os.Open(*configPath)
	if err != nil {
		log.Fatalf("Error opening config file: %v", err)
	}
	defer dbFile.Close()
	var cfg common.DBConfig
	if err := json.NewDecoder(dbFile).Decode(&cfg); err != nil {
		log.Fatalf("Error parsing config: %v", err)
	}

	// Load auth config.
	authFile, err := os.Open(*authConfigPath)
	if err != nil {
		log.Fatalf("Error opening auth config file: %v", err)
	}
	defer authFile.Close()
	var authCfg common.AuthConfig
	if err := json.NewDecoder(authFile).Decode(&authCfg); err != nil {
		log.Fatalf("Error parsing auth config: %v", err)
	}
	if authCfg.JWTSecret == "" {
		log.Fatalf("auth config: jwt_secret must not be empty")
	}

	dbPath := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
	)

	log.Println("DB path =", dbPath)
	server, err := NewServer(dbPath, cfg.GetEnvironment(), authCfg)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	mux := server.newMux()
	log.Println("Starting API server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
