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
    "database/sql"
    "fmt"
    "log"
    "time"
)

// UIUser represents a row in the users table.
type UIUser struct {
    UserID         string
    TenantID       int64
    Email          string
    PasswordHash   string
    Role           string
    IsActive       bool
    FailedAttempts int
    LockoutUntil   *time.Time
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

// LoginAuditEntry represents a row in the login_audit_log table.
type LoginAuditEntry struct {
    ID            int64
    UserID        *string
    Email         string
    Success       bool
    IPAddress     string
    FailureReason string
    CreatedAt     time.Time
}

// InsertUIUser creates a new user. passwordHash must already be a bcrypt digest.
// Returns the new user_id UUID.
func (db *DB) InsertUIUser(tenantID int64, email, passwordHash, role string) (string, error) {
    var userID string
    err := db.QueryRow(`
        INSERT INTO users (tenant_id, email, password_hash, role)
        VALUES ($1, $2, $3, $4)
        RETURNING user_id
    `, tenantID, email, passwordHash, role).Scan(&userID)
    if err != nil {
        log.Printf("InsertUIUser: %v", err)
        return "", fmt.Errorf("failed to insert user: %w", err)
    }
    return userID, nil
}

// ListUIUsers returns all users for the given tenant.
func (db *DB) ListUIUsers(tenantID int64) ([]UIUser, error) {
    rows, err := db.Query(`
        SELECT user_id, tenant_id, email, role, is_active, created_at, updated_at
        FROM users
        WHERE tenant_id = $1
        ORDER BY email
    `, tenantID)
    if err != nil {
        log.Printf("ListUIUsers: %v", err)
        return nil, fmt.Errorf("failed to list users: %w", err)
    }
    defer rows.Close()

    var users []UIUser
    for rows.Next() {
        var u UIUser
        if err := rows.Scan(&u.UserID, &u.TenantID, &u.Email, &u.Role,
            &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
            return nil, fmt.Errorf("failed to scan user: %w", err)
        }
        users = append(users, u)
    }
    return users, nil
}

// DeleteUIUser deletes a user (and cascades to their refresh tokens).
func (db *DB) DeleteUIUser(userID string, tenantID int64) error {
    res, err := db.Exec(
        `DELETE FROM users WHERE user_id = $1 AND tenant_id = $2`,
        userID, tenantID,
    )
    if err != nil {
        log.Printf("DeleteUIUser: %v", err)
        return fmt.Errorf("failed to delete user: %w", err)
    }
    n, _ := res.RowsAffected()
    if n == 0 {
        return fmt.Errorf("user %s not found for tenant %d", userID, tenantID)
    }
    return nil
}

// ResetUIUserPassword updates the stored bcrypt hash for a user.
func (db *DB) ResetUIUserPassword(userID string, tenantID int64, newPasswordHash string) error {
    res, err := db.Exec(`
        UPDATE users
        SET password_hash = $1, updated_at = NOW()
        WHERE user_id = $2 AND tenant_id = $3
    `, newPasswordHash, userID, tenantID)
    if err != nil {
        log.Printf("ResetUIUserPassword: %v", err)
        return fmt.Errorf("failed to reset password: %w", err)
    }
    n, _ := res.RowsAffected()
    if n == 0 {
        return fmt.Errorf("user %s not found for tenant %d", userID, tenantID)
    }
    return nil
}

// DeactivateUIUser sets is_active = false without deleting the user.
func (db *DB) DeactivateUIUser(userID string, tenantID int64) error {
    res, err := db.Exec(`
        UPDATE users SET is_active = false, updated_at = NOW()
        WHERE user_id = $1 AND tenant_id = $2
    `, userID, tenantID)
    if err != nil {
        log.Printf("DeactivateUIUser: %v", err)
        return fmt.Errorf("failed to deactivate user: %w", err)
    }
    n, _ := res.RowsAffected()
    if n == 0 {
        return fmt.Errorf("user %s not found for tenant %d", userID, tenantID)
    }
    return nil
}

// InsertLoginAuditLog records a login attempt. userID may be nil for
// unknown-email attempts. failureReason is empty on success.
func (db *DB) InsertLoginAuditLog(userID *string, email, ip, failureReason string, success bool) {
    _, err := db.Exec(`
        INSERT INTO login_audit_log (user_id, email, success, ip_address, failure_reason)
        VALUES ($1, $2, $3, $4, $5)
    `, userID, email, success, ip, failureReason)
    if err != nil {
        log.Printf("InsertLoginAuditLog: %v", err)
    }
}

// ListLoginAuditLog retrieves audit entries filtered by userID or email. Capped at limit rows.
func (db *DB) ListLoginAuditLog(userID *string, email *string, limit int) ([]LoginAuditEntry, error) {
    query := `
        SELECT id, user_id, email, success, ip_address, failure_reason, created_at
        FROM login_audit_log
    `
    args := []interface{}{}
    argIdx := 1

    if userID != nil {
        query += fmt.Sprintf(" WHERE user_id = $%d", argIdx)
        args = append(args, *userID)
        argIdx++
    } else if email != nil {
        query += fmt.Sprintf(" WHERE LOWER(email) = LOWER($%d)", argIdx)
        args = append(args, *email)
        argIdx++
    }

    query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argIdx)
    args = append(args, limit)

    rows, err := db.Query(query, args...)
    if err != nil {
        log.Printf("ListLoginAuditLog: %v", err)
        return nil, fmt.Errorf("failed to list audit log: %w", err)
    }
    defer rows.Close()

    var entries []LoginAuditEntry
    for rows.Next() {
        var e LoginAuditEntry
        var nullUserID sql.NullString
        if err := rows.Scan(&e.ID, &nullUserID, &e.Email, &e.Success,
            &e.IPAddress, &e.FailureReason, &e.CreatedAt); err != nil {
            return nil, fmt.Errorf("failed to scan audit entry: %w", err)
        }
        if nullUserID.Valid {
            e.UserID = &nullUserID.String
        }
        entries = append(entries, e)
    }
    return entries, nil
}
