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

func (s *Server) registerUserRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET    /v1/ma/users",                    s.requireJWT(s.handleListUsers))
	mux.HandleFunc("POST   /v1/ma/users",                    s.requireJWT(s.handleCreateUser))
	mux.HandleFunc("DELETE /v1/ma/users/{user_id}",          s.requireJWT(s.handleDeleteUser))
	mux.HandleFunc("PUT    /v1/ma/users/{user_id}/password", s.requireJWT(s.handleResetPassword))
}
