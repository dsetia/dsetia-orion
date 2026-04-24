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
// Created On:       04/10/2026
//
// UI authentication handlers and supporting DB helpers: login (POST /v1/ma/auth/login),
// token refresh (POST /v1/ma/auth/refresh), logout (POST /v1/ma/auth/logout).
// Also contains JWT sign/verify helpers, account lockout logic, and all
// refresh-token and user-lookup DB operations used by the auth flow.

package main

import (
	"encoding/json"
	"encoding/base64"
	"encoding/hex"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"orion/common"
)

// ErrNotFound is returned by DB lookup functions when the requested row does
// not exist. Handlers use errors.Is(err, ErrNotFound) to distinguish a
// legitimate "not found" (→ 401) from an unexpected DB failure (→ 500).
var ErrNotFound = errors.New("not found")

// ─── Lockout constants ───────────────────────────────────────────────────────

const (
	lockoutThreshold = 3
	lockoutDuration  = 10 * time.Minute
)

// ─── JWT helpers ─────────────────────────────────────────────────────────────
//
// maxJWTLen is the upper bound on the compact JWT string length this server
// will accept. Our tokens carry a fixed claim set (sub/email/role/tenant_id/
// iat/exp). With the maximum email length (255 chars) the encoded token tops
// out at ~629 characters. 2048 provides comfortable headroom while guarding
// against oversized inputs before any crypto work is done.
const maxJWTLen = 2048

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
//
// Validation layers applied (RFC 7519 + library defaults):
//   - Compact serialisation: exactly three dot-separated segments (§7.2)
//   - Algorithm:            HS256 only — rejects alg:none, RS*, ES* (§8)
//   - Signature:            HMAC-SHA256 verified against the configured secret
//   - exp claim:            required and must be in the future (§4.1.4)
//   - iat claim:            must be ≤ now when present (§4.1.6)
//   - Application claims:   sub, email, role, tenant_id must all be present
//                           and role must be a known value
func (s *Server) verifyJWT(tokenStr string) (*common.UserClaims, error) {
	// Fast structural checks before any crypto work.
	if len(tokenStr) > maxJWTLen {
		return nil, fmt.Errorf("invalid token format")
	}
	if strings.Count(tokenStr, ".") != 2 {
		return nil, fmt.Errorf("invalid token format")
	}

	c := &jwtClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"HS256"}), // reject alg:none and asymmetric algs
		jwt.WithExpirationRequired(),            // exp must be present, not just valid if present
		jwt.WithIssuedAt(),                      // iat must be <= now
	)
	token, err := parser.ParseWithClaims(tokenStr, c, func(*jwt.Token) (interface{}, error) {
		return []byte(s.authConfig.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Application-level claims must be non-empty after a valid signature.
	if c.Subject == "" || c.Email == "" || c.Role == "" || c.TenantID == 0 {
		return nil, fmt.Errorf("token missing required claims")
	}
	if c.Role != "system_admin" && c.Role != "security_analyst" {
		return nil, fmt.Errorf("token contains unknown role")
	}

	return &common.UserClaims{
		UserID:   c.Subject,
		Email:    c.Email,
		Role:     c.Role,
		TenantID: c.TenantID,
	}, nil
}

// ─── Login ───────────────────────────────────────────────────────────────────

func (s *Server) handleUserLogin(w http.ResponseWriter, r *http.Request) {
	log.Printf("auth: method=%s path=%s client=%s", r.Method, r.URL.Path, clientIP(r))
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

	ip := clientIP(r)

	// Step 1 — look up user by email.
	user, err := s.db.GetUserByEmail(req.Email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			s.db.InsertLoginAuditLog(nil, req.Email, ip, "unknown_user", false)
			jsonError(w, "invalid credentials", http.StatusUnauthorized)
		} else {
			log.Printf("handleUserLogin: GetUserByEmail: %v", err)
			jsonError(w, "internal error", http.StatusInternalServerError)
		}
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
		log.Printf("handleUserLogin: signJWT: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Step 8 — generate and store refresh token.
	ttl := time.Duration(s.authConfig.RefreshTokenTTLDays) * 24 * time.Hour
	refreshToken, err := s.db.InsertRefreshToken(user.UserID, ttl)
	if err != nil {
		log.Printf("handleUserLogin: InsertRefreshToken: %v", err)
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

func (s *Server) handleAccessTokenRefresh(w http.ResponseWriter, r *http.Request) {
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

	// Step 1 — fast format check before hashing or hitting the DB.
	// Refresh tokens are base64url-encoded 32 random bytes = exactly 44 chars.
	if len(req.RefreshToken) != 44 {
		jsonError(w, "invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	// Step 2 — look up by hash.
	rt, err := s.db.GetRefreshToken(req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(w, "invalid or expired refresh token", http.StatusUnauthorized)
		} else {
			log.Printf("handleAccessTokenRefresh: GetRefreshToken: %v", err)
			jsonError(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	// Step 3 — check revocation and expiry.
	if rt.Revoked || time.Now().After(rt.ExpiresAt) {
		jsonError(w, "invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	// Step 4 — load user and check is_active.
	u, err := s.db.GetUserByUserID(rt.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			jsonError(w, "invalid or expired refresh token", http.StatusUnauthorized)
		} else {
			log.Printf("handleAccessTokenRefresh: GetUserByUserID: %v", err)
			jsonError(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	if !u.IsActive {
		jsonError(w, "invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	// Step 5 — record last_used_at.
	s.db.UpdateRefreshTokenLastUsed(rt.TokenID)

	// Step 6 — issue new access JWT.
	accessToken, err := s.signJWT(u)
	if err != nil {
		log.Printf("handleAccessTokenRefresh: signJWT: %v", err)
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

func (s *Server) handleUserLogout(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("handleUserLogout: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── User lookup ─────────────────────────────────────────────────────────────

// GetUserByUserID looks up a user by their UUID primary key.
func (db *DB) GetUserByUserID(userID string) (*User, error) {
	var u User
	var lockoutUntil sql.NullTime
	err := db.QueryRow(`
		SELECT user_id, tenant_id, email, password_hash, role,
		       is_active, failed_attempts, lockout_until, created_at, updated_at
		FROM users
		WHERE user_id = $1
	`, userID).Scan(
		&u.UserID, &u.TenantID, &u.Email, &u.PasswordHash, &u.Role,
		&u.IsActive, &u.FailedAttempts, &lockoutUntil, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found: %w", ErrNotFound)
	}
	if err != nil {
		log.Printf("GetUserByUserID: %v", err)
		return nil, fmt.Errorf("failed to look up user: %w", err)
	}
	if lockoutUntil.Valid {
		u.LockoutUntil = &lockoutUntil.Time
	}
	return &u, nil
}

// GetUserByEmail looks up a user by email address (case-insensitive).
// Returns sql.ErrNoRows wrapped in a plain error when not found.
func (db *DB) GetUserByEmail(email string) (*User, error) {
	var u User
	var lockoutUntil sql.NullTime
	err := db.QueryRow(`
		SELECT user_id, tenant_id, email, password_hash, role,
		       is_active, failed_attempts, lockout_until, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`, email).Scan(
		&u.UserID, &u.TenantID, &u.Email, &u.PasswordHash, &u.Role,
		&u.IsActive, &u.FailedAttempts, &lockoutUntil, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found: %w", ErrNotFound)
	}
	if err != nil {
		log.Printf("GetUserByEmail: %v", err)
		return nil, fmt.Errorf("failed to look up user: %w", err)
	}
	if lockoutUntil.Valid {
		u.LockoutUntil = &lockoutUntil.Time
	}
	return &u, nil
}

// ─── Lockout ─────────────────────────────────────────────────────────────────

// RecordFailedAttempt increments the failed_attempts counter. If the count
// reaches the lockout threshold it also sets lockout_until.
func (db *DB) RecordFailedAttempt(userID string, threshold int, duration time.Duration) error {
	_, err := db.Exec(`
		UPDATE users
		SET failed_attempts = failed_attempts + 1,
		    lockout_until = CASE
		        WHEN failed_attempts + 1 >= $2 THEN NOW() + $3::interval
		        ELSE lockout_until
		    END,
		    updated_at = NOW()
		WHERE user_id = $1
	`, userID, threshold, fmt.Sprintf("%d seconds", int(duration.Seconds())))
	if err != nil {
		log.Printf("RecordFailedAttempt: %v", err)
		return fmt.Errorf("failed to record failed attempt: %w", err)
	}
	return nil
}

// ClearLockout resets failed_attempts to 0 and clears lockout_until on
// successful authentication or an admin password reset.
func (db *DB) ClearLockout(userID string) error {
	_, err := db.Exec(`
		UPDATE users
		SET failed_attempts = 0, lockout_until = NULL, updated_at = NOW()
		WHERE user_id = $1
	`, userID)
	if err != nil {
		log.Printf("ClearLockout: %v", err)
		return fmt.Errorf("failed to clear lockout: %w", err)
	}
	return nil
}

// ─── Refresh tokens ──────────────────────────────────────────────────────────

// InsertRefreshToken generates a cryptographically random opaque token,
// stores its SHA-256 digest, and returns the raw token to the caller.
func (db *DB) InsertRefreshToken(userID string, ttl time.Duration) (string, error) {
	raw, hash, err := generateRefreshToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	expiresAt := time.Now().Add(ttl)
	_, err = db.Exec(`
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hash, expiresAt)
	if err != nil {
		log.Printf("InsertRefreshToken: %v", err)
		return "", fmt.Errorf("failed to store refresh token: %w", err)
	}
	return raw, nil
}

// GetRefreshToken looks up a refresh token by its raw value.
func (db *DB) GetRefreshToken(raw string) (*RefreshToken, error) {
	hash := hashToken(raw)
	var rt RefreshToken
	var lastUsedAt sql.NullTime
	err := db.QueryRow(`
		SELECT token_id, user_id, token_hash, expires_at, revoked, created_at, last_used_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`, hash).Scan(
		&rt.TokenID, &rt.UserID, &rt.TokenHash,
		&rt.ExpiresAt, &rt.Revoked, &rt.CreatedAt, &lastUsedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("refresh token not found: %w", ErrNotFound)
	}
	if err != nil {
		log.Printf("GetRefreshToken: %v", err)
		return nil, fmt.Errorf("failed to look up refresh token: %w", err)
	}
	if lastUsedAt.Valid {
		rt.LastUsedAt = &lastUsedAt.Time
	}
	return &rt, nil
}

// RevokeRefreshTokens marks all non-expired refresh tokens for a user as revoked.
func (db *DB) RevokeRefreshTokens(userID string) error {
	_, err := db.Exec(`
		UPDATE refresh_tokens
		SET revoked = true
		WHERE user_id = $1 AND revoked = false AND expires_at > NOW()
	`, userID)
	if err != nil {
		log.Printf("RevokeRefreshTokens: %v", err)
		return fmt.Errorf("failed to revoke refresh tokens: %w", err)
	}
	return nil
}

// UpdateRefreshTokenLastUsed records the current time as last_used_at.
func (db *DB) UpdateRefreshTokenLastUsed(tokenID string) {
	_, err := db.Exec(
		`UPDATE refresh_tokens SET last_used_at = NOW() WHERE token_id = $1`,
		tokenID,
	)
	if err != nil {
		log.Printf("UpdateRefreshTokenLastUsed: %v", err)
	}
}

// ─── Token helpers ───────────────────────────────────────────────────────────

// generateRefreshToken produces a 32-byte random opaque token and its SHA-256 hex digest.
func generateRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	raw = base64.URLEncoding.EncodeToString(b)
	hash = hashToken(raw)
	return
}

// hashToken returns the SHA-256 hex digest of a token string.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
