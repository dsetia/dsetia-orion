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
	claimsKey        contextKey = iota // stores *common.UserClaims
	effectiveTenantKey                  // stores int64 effective tenant ID
)

// claimsFromContext retrieves the JWT claims stored by requireJWT.
// Returns nil if the middleware has not run (should never happen on
// authenticated routes).
func claimsFromContext(ctx context.Context) *common.UserClaims {
	v, _ := ctx.Value(claimsKey).(*common.UserClaims)
	return v
}

// tenantIDFromContext retrieves the effective tenant ID stored by
// applyTenantScope.  Returns 0 if not set.
func tenantIDFromContext(ctx context.Context) int64 {
	v, _ := ctx.Value(effectiveTenantKey).(int64)
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

// ─── requireRole ─────────────────────────────────────────────────────────────
//
// Applied inside requireJWT's chain for routes restricted to a single role.
// Relies on claims already being in context.

func requireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := claimsFromContext(r.Context())
		if claims == nil || claims.Role != role {
			jsonError(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// ─── applyTenantScope ────────────────────────────────────────────────────────
//
// Called as the first statement inside handleUITenantScoped (not used as a
// wrapper).  Reads the tenant_id from the JWT claims already in context and
// writes it as the authoritative effective tenant ID.
//
// Returns (enriched context, true) on success, or writes an error response
// and returns (nil, false) on failure.  The caller must return immediately
// when ok == false.

func (s *Server) applyTenantScope(w http.ResponseWriter, r *http.Request) (context.Context, bool) {
	claims := claimsFromContext(r.Context())
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	if claims.TenantID == 0 {
		jsonError(w, "token missing tenant_id", http.StatusUnauthorized)
		return nil, false
	}
	ctx := context.WithValue(r.Context(), effectiveTenantKey, claims.TenantID)
	return ctx, true
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// jsonError writes a JSON-formatted error response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(`{"error":"` + msg + `"}`)) //nolint:errcheck
}
