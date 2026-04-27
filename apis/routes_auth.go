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

// registerAuthRoutes mounts the user auth routes on mux.
func (s *Server) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/ma/auth/login",   s.handleUserLogin)
	mux.HandleFunc("/v1/ma/auth/refresh", s.handleAccessTokenRefresh)
	mux.HandleFunc("/v1/ma/auth/logout",  s.requireJWT(s.handleUserLogout))
}
