package main

import "net/http"

// registerNetworkAgentRoutes mounts all sensor-facing routes on mux.
func (s *Server) registerNetworkAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/authenticate/", s.handleAuthenticate)
	mux.HandleFunc("/v1/updates/", s.handleUpdates)
	mux.HandleFunc("/v1/status/", s.handleStatus)
	mux.HandleFunc("/v1/healthcheck", s.handleHealthCheck)
}
