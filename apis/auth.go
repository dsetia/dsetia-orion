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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"orion/common"
)

// ─── Lockout constants ───────────────────────────────────────────────────────

const (
	lockoutThreshold = 3
	lockoutDuration  = 10 * time.Minute
)

// ─── JWT helpers ─────────────────────────────────────────────────────────────

// jwtClaims is the internal JWT payload type.  It embeds RegisteredClaims for
// standard fields (sub, iat, exp) and adds the application-specific fields.
// This type stays in the apis package; callers work with common.UserClaims.
type jwtClaims struct {
	jwt.RegisteredClaims
	Email    string `json:"email"`
	Role     string `json:"role"`
	TenantID int64  `json:"tenant_id"`
}

// signJWT mints a new access JWT for the given user.
func (s *Server) signJWT(user *User) (string, error) {
	ttl := time.Duration(s.authConfig.AccessTokenTTLMins) * time.Minute
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.UserID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
		Email:    user.Email,
		Role:     user.Role,
		TenantID: user.TenantID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.authConfig.JWTSecret))
}

// verifyJWT validates the token string and returns the extracted claims.
// Returns an error for any invalid token (expired, bad signature, wrong alg).
func (s *Server) verifyJWT(tokenStr string) (*common.UserClaims, error) {
	c := &jwtClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.authConfig.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return &common.UserClaims{
		UserID:   c.Subject,
		Email:    c.Email,
		Role:     c.Role,
		TenantID: c.TenantID,
	}, nil
}

// ─── Login ───────────────────────────────────────────────────────────────────

func (s *Server) handleUILogin(w http.ResponseWriter, r *http.Request) {
	log.Printf("UI auth: method=%s path=%s client=%s", r.Method, r.URL.Path, r.RemoteAddr)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		jsonError(w, "email and password are required", http.StatusBadRequest)
		return
	}

	ip := r.RemoteAddr

	// Step 1 — look up user by email.
	user, err := s.db.GetUserByEmail(req.Email)
	if err != nil {
		s.db.InsertLoginAuditLog(nil, req.Email, ip, "unknown_user", false)
		jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Step 2 — check is_active.
	if !user.IsActive {
		s.db.InsertLoginAuditLog(&user.UserID, req.Email, ip, "inactive_user", false)
		jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Step 3 — check lockout.
	if user.LockoutUntil != nil && time.Now().Before(*user.LockoutUntil) {
		s.db.InsertLoginAuditLog(&user.UserID, req.Email, ip, "account_locked", false)
		jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Step 4 — verify password.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.db.RecordFailedAttempt(user.UserID, lockoutThreshold, lockoutDuration) //nolint:errcheck
		s.db.InsertLoginAuditLog(&user.UserID, req.Email, ip, "incorrect_password", false)
		jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Step 5 — clear lockout on success.
	s.db.ClearLockout(user.UserID) //nolint:errcheck

	// Step 6 — audit log success.
	s.db.InsertLoginAuditLog(&user.UserID, req.Email, ip, "", true)

	// Step 7 — sign access JWT.
	accessToken, err := s.signJWT(user)
	if err != nil {
		log.Printf("handleUILogin: signJWT: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Step 8 — generate and store refresh token.
	ttl := time.Duration(s.authConfig.RefreshTokenTTLDays) * 24 * time.Hour
	refreshToken, err := s.db.InsertRefreshToken(user.UserID, ttl)
	if err != nil {
		log.Printf("handleUILogin: InsertRefreshToken: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Step 9 — return tokens.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    s.authConfig.AccessTokenTTLMins * 60,
		"refresh_token": refreshToken,
	})
}

// ─── Refresh ─────────────────────────────────────────────────────────────────

func (s *Server) handleUIRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		jsonError(w, "refresh_token is required", http.StatusBadRequest)
		return
	}

	// Step 1 — look up by hash.
	rt, err := s.db.GetRefreshToken(req.RefreshToken)
	if err != nil {
		jsonError(w, "invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	// Step 2 — check revocation and expiry.
	if rt.Revoked || time.Now().After(rt.ExpiresAt) {
		jsonError(w, "invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	// Step 3 — load user and check is_active.
	user, err := s.db.GetUserByEmail("") // placeholder; we look up by user_id below
	_ = user
	// Look up by user_id directly.
	var u User
	err = s.db.QueryRow(`
		SELECT user_id, tenant_id, email, role, is_active
		FROM users WHERE user_id = $1
	`, rt.UserID).Scan(&u.UserID, &u.TenantID, &u.Email, &u.Role, &u.IsActive)
	if err != nil || !u.IsActive {
		jsonError(w, "invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	// Step 4 — record last_used_at.
	s.db.UpdateRefreshTokenLastUsed(rt.TokenID)

	// Step 5 — issue new access JWT.
	accessToken, err := s.signJWT(&u)
	if err != nil {
		log.Printf("handleUIRefresh: signJWT: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   s.authConfig.AccessTokenTTLMins * 60,
	})
}

// ─── Logout ──────────────────────────────────────────────────────────────────

func (s *Server) handleUILogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims := claimsFromContext(r.Context())
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := s.db.RevokeRefreshTokens(claims.UserID); err != nil {
		log.Printf("handleUILogout: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
