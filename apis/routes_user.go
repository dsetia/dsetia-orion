package main

import "net/http"

func (s *Server) registerUserRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET    /v1/ma/users",                    s.requireJWT(s.handleListUsers))
	mux.HandleFunc("POST   /v1/ma/users",                    s.requireJWT(s.handleCreateUser))
	mux.HandleFunc("DELETE /v1/ma/users/{user_id}",          s.requireJWT(s.handleDeleteUser))
	mux.HandleFunc("PUT    /v1/ma/users/{user_id}/password", s.requireJWT(s.handleResetPassword))
}
