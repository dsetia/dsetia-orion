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
