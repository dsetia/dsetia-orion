package main

import "net/http"

// registerAuthRoutes mounts the user auth routes on mux.
func (s *Server) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/ma/auth/login",   s.handleUserLogin)
	mux.HandleFunc("/v1/ma/auth/refresh", s.handleAccessTokenRefresh)
	mux.HandleFunc("/v1/ma/auth/logout",  s.requireJWT(s.handleUserLogout))
}
