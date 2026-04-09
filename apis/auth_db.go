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
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"time"
)

// RefreshToken represents a row in the refresh_tokens table.
type RefreshToken struct {
	TokenID    string
	UserID     string
	TokenHash  string
	ExpiresAt  time.Time
	Revoked    bool
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// ─── User lookup ─────────────────────────────────────────────────────────────

// GetUIUserByEmail looks up a user by email address (case-insensitive).
// Returns sql.ErrNoRows wrapped in a plain error when not found.
func (db *DB) GetUIUserByEmail(email string) (*UIUser, error) {
	var u UIUser
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
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		log.Printf("GetUIUserByEmail: %v", err)
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
		return nil, fmt.Errorf("refresh token not found")
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

// ─── Tenant-scoped resource queries for UI handlers ──────────────────────────

// ListStatusByTenant returns all status rows for a given tenant.
func (db *DB) ListStatusByTenant(tenantID int64) ([]Status, error) {
	rows, err := db.Query(`
		SELECT device_id, tenant_id, software, rules, threatintel, created_at, updated_at
		FROM status
		WHERE tenant_id = $1
		ORDER BY device_id
	`, tenantID)
	if err != nil {
		log.Printf("ListStatusByTenant: %v", err)
		return nil, fmt.Errorf("failed to list status: %w", err)
	}
	defer rows.Close()

	var results []Status
	for rows.Next() {
		var s Status
		if err := rows.Scan(&s.DeviceID, &s.TenantID, &s.Software, &s.Rules,
			&s.ThreatIntel, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan status: %w", err)
		}
		results = append(results, s)
	}
	return results, nil
}

// ListVersionsByTenant returns all version rows for a given tenant.
func (db *DB) ListVersionsByTenant(tenantID int64) ([]Version, error) {
	rows, err := db.Query(`
		SELECT device_id, tenant_id, software, rules, threatintel, created_at, updated_at
		FROM version
		WHERE tenant_id = $1
		ORDER BY device_id
	`, tenantID)
	if err != nil {
		log.Printf("ListVersionsByTenant: %v", err)
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}
	defer rows.Close()

	var results []Version
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.DeviceID, &v.TenantID, &v.Software, &v.Rules,
			&v.ThreatIntel, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan version: %w", err)
		}
		results = append(results, v)
	}
	return results, nil
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
