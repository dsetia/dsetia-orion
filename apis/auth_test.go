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
// Created On:       04/12/2026
//
// Unit tests covering login, logout, lockout, token refresh, /me and 
// role enforcement

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"orion/common"
	_ "github.com/lib/pq"
)

// ─── Fixtures ─────────────────────────────────────────────────────────────────

var testAuthConfig = common.AuthConfig{
	JWTSecret:           "test-secret-key-32-characters-xx",
	AccessTokenTTLMins:  15,
	RefreshTokenTTLDays: 7,
}

const (
	testAdminEmail      = "admin@test.com"
	testAdminPassword   = "adminpassword123"
	testAnalystEmail    = "analyst@test.com"
	testAnalystPassword = "analystpassword123"
)

// ─── Setup helpers ────────────────────────────────────────────────────────────

// setupAuthServer builds a clean DB with both the base schema and the auth
// schema applied, inserts one system_admin and one security_analyst user,
// and returns a Server ready for auth handler tests.
func setupAuthServer(t *testing.T) (*Server, int64) {
	t.Helper()
	cleanupTestDB(t)
	db := setupTestDB(t)

	var tenantID int64
	if err := db.QueryRow(
		`SELECT tenant_id FROM tenants WHERE tenant_name = 'test-tenant'`,
	).Scan(&tenantID); err != nil {
		t.Fatalf("get tenant ID: %v", err)
	}

	// Use MinCost so bcrypt doesn't slow down the test suite.
	adminHash, _ := bcrypt.GenerateFromPassword([]byte(testAdminPassword), bcrypt.MinCost)
	if _, err := db.InsertUser(tenantID, testAdminEmail, string(adminHash), "system_admin"); err != nil {
		t.Fatalf("insert admin user: %v", err)
	}
	analystHash, _ := bcrypt.GenerateFromPassword([]byte(testAnalystPassword), bcrypt.MinCost)
	if _, err := db.InsertUser(tenantID, testAnalystEmail, string(analystHash), "security_analyst"); err != nil {
		t.Fatalf("insert analyst user: %v", err)
	}

	return &Server{db: db, authConfig: testAuthConfig}, tenantID
}

// ─── Request / response helpers ───────────────────────────────────────────────

// loginResp calls handleUserLogin and returns the HTTP status and the decoded body.
func loginResp(t *testing.T, s *Server, email, password string) (int, map[string]interface{}) {
	t.Helper()
	body := `{"email":"` + email + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/ma/auth/login", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handleUserLogin(rr, req)
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp) //nolint:errcheck
	return rr.Code, resp
}

// mustLogin logs in and fatals on failure; returns (accessToken, refreshToken).
func mustLogin(t *testing.T, s *Server, email, password string) (string, string) {
	t.Helper()
	code, resp := loginResp(t, s, email, password)
	if code != http.StatusOK {
		t.Fatalf("login %s: expected 200, got %d body=%v", email, code, resp)
	}
	access, _ := resp["access_token"].(string)
	refresh, _ := resp["refresh_token"].(string)
	if access == "" {
		t.Fatal("login returned empty access_token")
	}
	return access, refresh
}

// uiReq creates a management API request, optionally including a Bearer token.
func uiReq(method, path, token, body string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// callScoped dispatches a request through the production ServeMux.
func callScoped(s *Server, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s.newMux().ServeHTTP(rr, req)
	return rr
}

// callMe dispatches a /v1/ma/me request through the production ServeMux.
func callMe(s *Server, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s.newMux().ServeHTTP(rr, req)
	return rr
}

// jsonField unmarshals body and returns the named field as a string.
func jsonField(body []byte, key string) string {
	var m map[string]interface{}
	json.Unmarshal(body, &m) //nolint:errcheck
	v, _ := m[key].(string)
	return v
}

// ─── TestUILogin ──────────────────────────────────────────────────────────────

func TestUILogin(t *testing.T) {
	s, _ := setupAuthServer(t)
	defer s.db.Close()

	tests := []struct {
		name           string
		method         string
		email          string
		password       string
		expectedStatus int
		checkBody      func(t *testing.T, body []byte)
	}{
		{
			name:           "valid admin login returns tokens",
			method:         http.MethodPost,
			email:          testAdminEmail,
			password:       testAdminPassword,
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				json.Unmarshal(body, &resp) //nolint:errcheck
				if resp["access_token"] == "" {
					t.Error("expected non-empty access_token")
				}
				if resp["refresh_token"] == "" {
					t.Error("expected non-empty refresh_token")
				}
				if resp["token_type"] != "Bearer" {
					t.Errorf("expected token_type Bearer, got %v", resp["token_type"])
				}
				if resp["expires_in"] == nil {
					t.Error("expected expires_in in response")
				}
			},
		},
		{
			name:           "valid analyst login",
			method:         http.MethodPost,
			email:          testAnalystEmail,
			password:       testAnalystPassword,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "wrong password",
			method:         http.MethodPost,
			email:          testAdminEmail,
			password:       "wrongpassword!",
			expectedStatus: http.StatusUnauthorized,
			checkBody: func(t *testing.T, body []byte) {
				if !bytes.Contains(body, []byte("invalid credentials")) {
					t.Errorf("expected 'invalid credentials' in body, got %s", body)
				}
			},
		},
		{
			name:           "unknown email",
			method:         http.MethodPost,
			email:          "nobody@test.com",
			password:       "somepassword123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "empty email",
			method:         http.MethodPost,
			email:          "",
			password:       testAdminPassword,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty password",
			method:         http.MethodPost,
			email:          testAdminEmail,
			password:       "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "wrong HTTP method",
			method:         http.MethodGet,
			email:          testAdminEmail,
			password:       testAdminPassword,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body string
			if tt.email != "" || tt.password != "" {
				body = `{"email":"` + tt.email + `","password":"` + tt.password + `"}`
			} else {
				body = `{"email":"","password":""}`
			}
			req := httptest.NewRequest(tt.method, "/v1/ma/auth/login", strings.NewReader(body))
			rr := httptest.NewRecorder()
			s.handleUserLogin(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.expectedStatus, rr.Code, rr.Body.String())
			}
			if tt.checkBody != nil {
				tt.checkBody(t, rr.Body.Bytes())
			}
		})
	}
}

// TestUILoginLockout verifies that three consecutive wrong passwords lock the
// account; a subsequent correct-password attempt is rejected while locked.
func TestUILoginLockout(t *testing.T) {
	s, _ := setupAuthServer(t)
	defer s.db.Close()

	// Three bad attempts
	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/ma/auth/login",
			strings.NewReader(`{"email":"`+testAnalystEmail+`","password":"wrongpassword123"}`))
		rr := httptest.NewRecorder()
		s.handleUserLogin(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i, rr.Code)
		}
	}

	// Correct password should now be rejected because of the lockout
	req := httptest.NewRequest(http.MethodPost, "/v1/ma/auth/login",
		strings.NewReader(`{"email":"`+testAnalystEmail+`","password":"`+testAnalystPassword+`"}`))
	rr := httptest.NewRecorder()
	s.handleUserLogin(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected locked account to return 401, got %d", rr.Code)
	}
}

// TestUILoginInactiveUser verifies that a deactivated account cannot log in.
func TestUILoginInactiveUser(t *testing.T) {
	s, tenantID := setupAuthServer(t)
	defer s.db.Close()

	// Fetch the analyst user ID
	var userID string
	s.db.QueryRow(`SELECT user_id FROM users WHERE email = $1`, testAnalystEmail).Scan(&userID) //nolint:errcheck

	// Deactivate the account
	if err := s.db.DeactivateUser(userID, tenantID); err != nil {
		t.Fatalf("deactivate user: %v", err)
	}

	code, _ := loginResp(t, s, testAnalystEmail, testAnalystPassword)
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401 for inactive user, got %d", code)
	}
}

// ─── TestUIRefresh ────────────────────────────────────────────────────────────

func TestUIRefresh(t *testing.T) {
	s, _ := setupAuthServer(t)
	defer s.db.Close()

	_, refreshToken := mustLogin(t, s, testAdminEmail, testAdminPassword)

	tests := []struct {
		name           string
		body           string
		expectedStatus int
	}{
		{
			name:           "valid refresh token returns new access token",
			body:           `{"refresh_token":"` + refreshToken + `"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "garbage token",
			body:           `{"refresh_token":"notavalidtoken"}`,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing refresh_token field",
			body:           `{}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "wrong HTTP method",
			body:           "",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var method string
			if tt.name == "wrong HTTP method" {
				method = http.MethodGet
			} else {
				method = http.MethodPost
			}
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(method, "/v1/ma/auth/refresh", strings.NewReader(tt.body))
			} else {
				req = httptest.NewRequest(method, "/v1/ma/auth/refresh", nil)
			}
			rr := httptest.NewRecorder()
			s.handleAccessTokenRefresh(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected %d, got %d (body: %s)", tt.expectedStatus, rr.Code, rr.Body.String())
			}
			if tt.expectedStatus == http.StatusOK {
				if tok := jsonField(rr.Body.Bytes(), "access_token"); tok == "" {
					t.Error("expected non-empty access_token in refresh response")
				}
			}
		})
	}
}

// TestUIRefreshAfterLogout verifies that logout revokes the refresh token.
func TestUIRefreshAfterLogout(t *testing.T) {
	s, _ := setupAuthServer(t)
	defer s.db.Close()

	access, refresh := mustLogin(t, s, testAdminEmail, testAdminPassword)

	// Logout
	req := uiReq(http.MethodPost, "/v1/ma/auth/logout", access, "")
	rr := httptest.NewRecorder()
	s.requireJWT(s.handleUserLogout)(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("logout: expected 204, got %d", rr.Code)
	}

	// Refresh with the now-revoked token must fail
	req = httptest.NewRequest(http.MethodPost, "/v1/ma/auth/refresh",
		strings.NewReader(`{"refresh_token":"`+refresh+`"}`))
	rr = httptest.NewRecorder()
	s.handleAccessTokenRefresh(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 after logout, got %d", rr.Code)
	}
}

// ─── TestUILogout ─────────────────────────────────────────────────────────────

func TestUILogout(t *testing.T) {
	s, _ := setupAuthServer(t)
	defer s.db.Close()

	access, _ := mustLogin(t, s, testAdminEmail, testAdminPassword)

	t.Run("valid JWT returns 204", func(t *testing.T) {
		req := uiReq(http.MethodPost, "/v1/ma/auth/logout", access, "")
		rr := httptest.NewRecorder()
		s.requireJWT(s.handleUserLogout)(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rr.Code)
		}
	})

	t.Run("no JWT returns 401", func(t *testing.T) {
		req := uiReq(http.MethodPost, "/v1/ma/auth/logout", "", "")
		rr := httptest.NewRecorder()
		s.requireJWT(s.handleUserLogout)(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("wrong HTTP method returns 405", func(t *testing.T) {
		access2, _ := mustLogin(t, s, testAnalystEmail, testAnalystPassword)
		req := uiReq(http.MethodGet, "/v1/ma/auth/logout", access2, "")
		rr := httptest.NewRecorder()
		s.requireJWT(s.handleUserLogout)(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rr.Code)
		}
	})
}

// ─── TestUIMe ─────────────────────────────────────────────────────────────────

func TestUIMe(t *testing.T) {
	s, tenantID := setupAuthServer(t)
	defer s.db.Close()

	access, _ := mustLogin(t, s, testAdminEmail, testAdminPassword)

	t.Run("valid JWT returns user fields", func(t *testing.T) {
		req := uiReq(http.MethodGet, "/v1/ma/me", access, "")
		rr := callMe(s, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp) //nolint:errcheck
		if resp["email"] != testAdminEmail {
			t.Errorf("expected email %s, got %v", testAdminEmail, resp["email"])
		}
		if resp["role"] != "system_admin" {
			t.Errorf("expected role system_admin, got %v", resp["role"])
		}
		if int64(resp["tenant_id"].(float64)) != tenantID {
			t.Errorf("expected tenant_id %d, got %v", tenantID, resp["tenant_id"])
		}
		if resp["user_id"] == "" {
			t.Error("expected non-empty user_id")
		}
	})

	t.Run("no JWT returns 401", func(t *testing.T) {
		req := uiReq(http.MethodGet, "/v1/ma/me", "", "")
		rr := callMe(s, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("wrong HTTP method returns 405", func(t *testing.T) {
		req := uiReq(http.MethodPost, "/v1/ma/me", access, "")
		rr := callMe(s, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rr.Code)
		}
	})
}

// ─── TestUsers ──────────────────────────────────────────────────────────────

func TestUsersListAndCreate(t *testing.T) {
	s, _ := setupAuthServer(t)
	defer s.db.Close()

	adminToken, _ := mustLogin(t, s, testAdminEmail, testAdminPassword)
	analystToken, _ := mustLogin(t, s, testAnalystEmail, testAnalystPassword)

	t.Run("system_admin can list users", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodGet, "/v1/ma/users", adminToken, ""))
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("security_analyst cannot list users", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodGet, "/v1/ma/users", analystToken, ""))
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("system_admin can create user", func(t *testing.T) {
		body := `{"email":"new@test.com","password":"newpassword123","role":"security_analyst"}`
		rr := callScoped(s, uiReq(http.MethodPost, "/v1/ma/users", adminToken, body))
		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d (body: %s)", rr.Code, rr.Body.String())
		}
		if jsonField(rr.Body.Bytes(), "user_id") == "" {
			t.Error("expected user_id in creation response")
		}
	})

	t.Run("duplicate email returns 409", func(t *testing.T) {
		body := `{"email":"` + testAdminEmail + `","password":"somepassword123","role":"security_analyst"}`
		rr := callScoped(s, uiReq(http.MethodPost, "/v1/ma/users", adminToken, body))
		if rr.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d", rr.Code)
		}
	})

	t.Run("short password rejected", func(t *testing.T) {
		body := `{"email":"short@test.com","password":"short","role":"security_analyst"}`
		rr := callScoped(s, uiReq(http.MethodPost, "/v1/ma/users", adminToken, body))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("invalid role rejected", func(t *testing.T) {
		body := `{"email":"badrole@test.com","password":"validpassword123","role":"superuser"}`
		rr := callScoped(s, uiReq(http.MethodPost, "/v1/ma/users", adminToken, body))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("security_analyst cannot create user", func(t *testing.T) {
		body := `{"email":"another@test.com","password":"validpassword123","role":"security_analyst"}`
		rr := callScoped(s, uiReq(http.MethodPost, "/v1/ma/users", analystToken, body))
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})
}

func TestUsersDelete(t *testing.T) {
	s, tenantID := setupAuthServer(t)
	defer s.db.Close()

	adminToken, _ := mustLogin(t, s, testAdminEmail, testAdminPassword)
	analystToken, _ := mustLogin(t, s, testAnalystEmail, testAnalystPassword)

	// Create a disposable user for delete tests
	hash, _ := bcrypt.GenerateFromPassword([]byte("deletepassword123"), bcrypt.MinCost)
	targetID, err := s.db.InsertUser(tenantID, "todelete@test.com", string(hash), "security_analyst")
	if err != nil {
		t.Fatalf("insert target user: %v", err)
	}

	t.Run("security_analyst cannot delete a user", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodDelete, "/v1/ma/users/"+targetID, analystToken, ""))
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("system_admin cannot delete their own account", func(t *testing.T) {
		var adminID string
		s.db.QueryRow(`SELECT user_id FROM users WHERE email = $1`, testAdminEmail).Scan(&adminID) //nolint:errcheck
		rr := callScoped(s, uiReq(http.MethodDelete, "/v1/ma/users/"+adminID, adminToken, ""))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for self-delete, got %d", rr.Code)
		}
	})

	t.Run("system_admin can delete another user", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodDelete, "/v1/ma/users/"+targetID, adminToken, ""))
		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d (body: %s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("deleting non-existent user returns 404", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodDelete, "/v1/ma/users/"+targetID, adminToken, ""))
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404 for already-deleted user, got %d", rr.Code)
		}
	})
}

func TestUsersResetPassword(t *testing.T) {
	s, tenantID := setupAuthServer(t)
	defer s.db.Close()

	adminToken, _ := mustLogin(t, s, testAdminEmail, testAdminPassword)
	analystToken, _ := mustLogin(t, s, testAnalystEmail, testAnalystPassword)

	var adminID, analystID string
	s.db.QueryRow(`SELECT user_id FROM users WHERE email = $1`, testAdminEmail).Scan(&adminID)   //nolint:errcheck
	s.db.QueryRow(`SELECT user_id FROM users WHERE email = $1`, testAnalystEmail).Scan(&analystID) //nolint:errcheck

	// Create a second analyst to test cross-user reset restriction
	hash, _ := bcrypt.GenerateFromPassword([]byte("otherpassword123"), bcrypt.MinCost)
	otherID, err := s.db.InsertUser(tenantID, "other@test.com", string(hash), "security_analyst")
	if err != nil {
		t.Fatalf("insert other user: %v", err)
	}

	newPwBody := `{"password":"newvalidpassword123"}`

	t.Run("system_admin can reset any user password", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodPut, "/v1/ma/users/"+analystID+"/password", adminToken, newPwBody))
		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d (body: %s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("security_analyst can reset their own password", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodPut, "/v1/ma/users/"+analystID+"/password", analystToken, newPwBody))
		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d (body: %s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("security_analyst cannot reset another user password", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodPut, "/v1/ma/users/"+otherID+"/password", analystToken, newPwBody))
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("short new password rejected", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodPut, "/v1/ma/users/"+adminID+"/password", adminToken, `{"password":"short"}`))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})
}

// ─── TestUITenantScoped ───────────────────────────────────────────────────────

func TestUITenantScoped(t *testing.T) {
	s, _ := setupAuthServer(t)
	defer s.db.Close()

	adminToken, _ := mustLogin(t, s, testAdminEmail, testAdminPassword)

	t.Run("list devices returns 200", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodGet, "/v1/ma/devices", adminToken, ""))
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("get single device returns 200", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodGet, "/v1/ma/devices/dev1", adminToken, ""))
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("get non-existent device returns 404", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodGet, "/v1/ma/devices/nosuchdevice", adminToken, ""))
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("list versions returns 200", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodGet, "/v1/ma/versions", adminToken, ""))
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("list status returns 200", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodGet, "/v1/ma/status", adminToken, ""))
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("unknown resource returns 404", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodGet, "/v1/ma/nosuchresource", adminToken, ""))
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodGet, "/v1/ma/devices", "", ""))
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("invalid JWT returns 401", func(t *testing.T) {
		rr := callScoped(s, uiReq(http.MethodGet, "/v1/ma/devices", "not.a.token", ""))
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})
}

// ─── TestUIJWTIsolation ───────────────────────────────────────────────────────

// TestUIJWTIsolation verifies that an admin from tenant A cannot access resources
// by crafting a JWT with a different tenant_id — the tenant_id is always derived
// from the JWT signed by this server, so a tampered token is rejected outright.
func TestUIJWTIsolation(t *testing.T) {
	s, _ := setupAuthServer(t)
	defer s.db.Close()

	// Sign a token with an altered secret (simulating a tampered JWT)
	tamperedServer := &Server{
		db: s.db,
		authConfig: common.AuthConfig{
			JWTSecret:           "a-different-secret-key-for-test",
			AccessTokenTTLMins:  15,
			RefreshTokenTTLDays: 7,
		},
	}
	user, _ := s.db.GetUserByEmail(testAdminEmail)
	tamperedToken, _ := tamperedServer.signJWT(user)

	rr := callScoped(s, uiReq(http.MethodGet, "/v1/ma/devices", tamperedToken, ""))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for token signed with wrong secret, got %d", rr.Code)
	}
}
