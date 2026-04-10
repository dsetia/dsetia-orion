// Copyright (c) 2025 SecurITe
// All rights reserved.
//
// This source code is the property of SecurITe.
// Unauthorized copying, modification, or distribution of this file,
// via any medium is strictly prohibited unless explicitly authorized
// by SecurITe.
//
// This software is proprietary and confidential.

package main

import (
	"context"
	"net/http"
	"strings"

	"orion/common"
)

// ─── Context key types ───────────────────────────────────────────────────────
//
// Using unexported types prevents key collisions with any other package that
// uses context.WithValue.  No two packages can produce the same contextKey
// value because the type itself is unexported.

type contextKey int

const (
	claimsKey contextKey = iota // stores *common.UserClaims
)

// claimsFromContext retrieves the JWT claims stored by requireJWT.
// Returns nil if the middleware has not run (should never happen on
// authenticated routes).
func claimsFromContext(ctx context.Context) *common.UserClaims {
	v, _ := ctx.Value(claimsKey).(*common.UserClaims)
	return v
}

// ─── requireJWT ──────────────────────────────────────────────────────────────
//
// Validates the Authorization: Bearer <token> header, parses the JWT, and
// stores the resulting UserClaims in the request context.  All downstream
// UI handlers read identity exclusively from that context value.

func (s *Server) requireJWT(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			jsonError(w, "missing or malformed Authorization header", http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := s.verifyJWT(tokenStr)
		if err != nil {
			jsonError(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// jsonError writes a JSON-formatted error response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(`{"error":"` + msg + `"}`)) //nolint:errcheck
}
