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

// registerNetworkAgentRoutes mounts all sensor-facing routes on mux.
func (s *Server) registerNetworkAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/authenticate/", s.handleAuthenticate)
	mux.HandleFunc("/v1/updates/", s.handleUpdates)
	mux.HandleFunc("/v1/status/", s.handleStatus)
	mux.HandleFunc("/v1/healthcheck", s.handleHealthCheck)
}
