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
// Created On:       04/12/2026
//

package main

import "net/http"

// registerResourceRoutes mounts routes currently for UI
func (s *Server) registerResourceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/ma/me", s.requireJWT(s.handleMe))
	mux.HandleFunc("GET /v1/ma/devices",             s.requireJWT(s.handleListDevices))
	mux.HandleFunc("GET /v1/ma/devices/{device_id}", s.requireJWT(s.handleGetDevice))
	mux.HandleFunc("GET /v1/ma/versions", s.requireJWT(s.handleListVersions))
	mux.HandleFunc("GET /v1/ma/status", s.requireJWT(s.handleListStatus))

}
