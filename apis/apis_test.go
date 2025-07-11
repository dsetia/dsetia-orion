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
// Created On:       05/26/2025

package main

import (
    "os"
    "fmt"
    "bytes"
    "io/ioutil"
    "log"
    "net/http"
    "net/http/httptest"
    "encoding/json"
    "testing"
    "orion/common"
    _ "github.com/lib/pq"
)

func loadDBConfig(path string) (common.DBConfig, error) {
    var cfg common.DBConfig
    data, err := os.ReadFile(path)
    if err != nil {
        return cfg, err
    }
    err = json.Unmarshal(data, &cfg)
    return cfg, err
}

func cleanupTestDB(t *testing.T) {
    t.Helper()
    cfg, err := loadDBConfig("../config/db.json")
    if err != nil {
        t.Fatalf("Failed to load config: %v", err)
    }

    cfg.Host = "localhost" // test running outside docker network
    db2, err := NewDB(cfg.ConnString())
    if err != nil {
        t.Fatalf("Failed to connect to DB: %v", err)
    }

    // Drop and recreate testdb
    db2.Exec("DROP DATABASE IF EXISTS testdb")
    db2.Exec("CREATE DATABASE testdb")
    db2.Close()
}

func setupTestDB(t *testing.T) *DB {
    t.Helper()
    cfg, err := loadDBConfig("../config/db.json")
    if err != nil {
        t.Fatalf("Failed to load config: %v", err)
    }

    cfg.Host = "localhost" // test running outside docker network
    cfg.DBName = "testdb"
    db, err := NewDB(cfg.ConnString())
    if err != nil {
        t.Fatalf("Failed to connect to DB: %v", err)
    }

    // Create schema
    schemaFile := "../db/schema_pg.sql"
    schemaSQL, err := ioutil.ReadFile(schemaFile)
    if err != nil {
        t.Fatalf("Failed to create schema: %v", err)
    }

    // Initialize schema
    _, err = db.Exec(string(schemaSQL))
    if err != nil {
        log.Fatalf("Failed to apply schema: %v", err)
    }

    // Insert test data
    tenantID, err := db.GetOrInsertTenant("test-tenant")
    if err != nil {
        t.Fatalf("Failed to insert tenant: %v", err)
    }
    _, err = db.GetOrInsertDevice("dev1", tenantID, "Test Device", "v1.2.3")
    if err != nil {
        t.Fatalf("Failed to insert device: %v", err)
    }
    _, err = db.GetOrInsertAPIKey("valid-key", tenantID, "dev1", true)
    if err != nil {
        t.Fatalf("Failed to insert API key: %v", err)
    }
    _, err = db.InsertHndrSw("v1.2.3", 1024, "sw-sha256")
    if err != nil {
        t.Fatalf("Failed to insert hndr_sw: %v", err)
    }
    _, err = db.InsertHndrRules(tenantID, "r1.2.3", 512, "rules-sha256")
    if err != nil {
        t.Fatalf("Failed to insert hndr_rules: %v", err)
    }
    _, err = db.InsertThreatIntel("2025.04.10.153010", 256, "ti-sha256")
    if err != nil {
        t.Fatalf("Failed to insert threatintel: %v", err)
    }

    // 2nd device to use global software
    _, err = db.GetOrInsertDevice("dev2", tenantID, "Test Device 2", "")
    if err != nil {
        t.Fatalf("Failed to insert device: %v", err)
    }
    _, err = db.GetOrInsertAPIKey("valid-key-2", tenantID, "dev2", true)
    if err != nil {
        t.Fatalf("Failed to insert API key: %v", err)
    }
    _, err = db.InsertHndrSw("v1.2.4", 1234, "sw-sha256")
    if err != nil {
        t.Fatalf("Failed to insert hndr_sw: %v", err)
    }

    // status
    err = db.InsertStatus("dev1", tenantID, "failure", "failure", "failure")
    if err != nil {
        t.Fatalf("Failed to insert hndr_sw: %v", err)
    }

    return db
}

func TestAuthenticate(t *testing.T) {
    cleanupTestDB(t)
    db := setupTestDB(t)

    server := &Server{db: db}
    tests := []struct {
        name           string
        url            string
        apiKey         string
        deviceID       string
        expectedStatus int
        expectedBody   string
    }{
        {
            name:           "Valid credentials",
            url:            "/v1/authenticate/1",
            apiKey:         "valid-key",
            deviceID:       "dev1",
            expectedStatus: http.StatusOK,
            expectedBody:   "{\"status\":\"authenticated\"}\n",
        },
        {
            name:           "Invalid API key",
            url:            "/v1/authenticate/1",
            apiKey:         "invalid-key",
            deviceID:       "dev1",
            expectedStatus: http.StatusUnauthorized,
	    expectedBody:   "Unauthorized: failed to validate API key\n",
        },
        {
            name:           "Missing headers",
            url:            "/v1/authenticate/1",
            apiKey:         "",
            deviceID:       "dev1",
            expectedStatus: http.StatusUnauthorized,
            expectedBody:   "Unauthorized: missing API key\n",
        },
        {
            name:           "Wrong tenant_id",
            url:            "/v1/authenticate/2",
            apiKey:         "valid-key",
            deviceID:       "dev1",
            expectedStatus: http.StatusUnauthorized,
            expectedBody:   "Unauthorized: tenant mismatch\n",
        },
        {
            name:           "Wrong device ID",
            url:            "/v1/authenticate/1",
            apiKey:         "valid-key",
            deviceID:       "dev2",
            expectedStatus: http.StatusUnauthorized,
	    expectedBody:   "Unauthorized: failed to validate device id\n",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req, err := http.NewRequest("GET", tt.url, nil)
            if err != nil {
                t.Fatal(err)
            }
            req.Header.Set("X-API-KEY", tt.apiKey)
            req.Header.Set("X-DEVICE-ID", tt.deviceID)

            rr := httptest.NewRecorder()
            server.handleAuthenticate(rr, req)

            if rr.Code != tt.expectedStatus {
                t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
            }
            if body := rr.Body.String(); body != tt.expectedBody {
                t.Errorf("Expected body %q, got %q", tt.expectedBody, body)
            }
        })
    }
    defer db.Close()
}

func TestUpdate(t *testing.T) {
    cleanupTestDB(t)
    db := setupTestDB(t)

    server := &Server{db: db}
    tests := []struct {
        name           string
        apiKey         string
        deviceID       string
        tenantID       string
	body           string
        expectedStatus int
        expectedBody   string
    }{
        {
            name:           "Valid request with updates",
            apiKey:         "valid-key",
            deviceID:       "dev1",
            tenantID:       "1",
	    body:           `{"software": {"version":"v1.2.2"},"rules": {"version":"r1.2.2"},"threatintel":{"version":"2025.04.10.153000"}}`,
            expectedStatus: http.StatusOK,
            expectedBody: fmt.Sprintf(
                `{"software":{"version":"v1.2.3","size":1024,"sha256":"sw-sha256","source":"device","download_url":%q},"rules":{"version":"r1.2.3","size":512,"sha256":"rules-sha256","download_url":%q},"threatintel":{"version":"2025.04.10.153010","size":256,"sha256":"ti-sha256","download_url":%q}}` + "\n",
                DownloadURLFormat(1, "software", "hndr-sw", "v1.2.3"),
                DownloadURLFormatRules(1, "rules", "hndr-rules", "r1.2.3"),
                DownloadURLFormat(1, "threatintel", "threatintel", "2025.04.10.153010"),
            ),
        },
        {
            name:           "Valid jason with incorrect fields",
            apiKey:         "valid-key",
            deviceID:       "dev1",
            tenantID:       "1",
	    body:           `{"image_version": {"version":"v1.2.3"},"rules_version": {"version":"r1.2.3"},"threatfeed_version":{"version":"2025.04.10.153010"}}`,
            expectedStatus: http.StatusOK,
            expectedBody: fmt.Sprintf(
                `{"software":{"version":"v1.2.3","size":1024,"sha256":"sw-sha256","source":"device","download_url":%q},"rules":{"version":"r1.2.3","size":512,"sha256":"rules-sha256","download_url":%q},"threatintel":{"version":"2025.04.10.153010","size":256,"sha256":"ti-sha256","download_url":%q}}` + "\n",
                DownloadURLFormat(1, "software", "hndr-sw", "v1.2.3"),
                DownloadURLFormatRules(1, "rules", "hndr-rules", "r1.2.3"),
                DownloadURLFormat(1, "threatintel", "threatintel", "2025.04.10.153010"),
            ),
        },
        {
            name:           "No updates needed",
            apiKey:         "valid-key",
            deviceID:       "dev1",
            tenantID:       "1",
	    body:           `{"software": {"version":"v1.2.10"},"rules": {"version":"r1.2.20"},"threatintel":{"version":"2025.04.10.153020"}}`,
            expectedStatus: http.StatusOK,
	    expectedBody:   `{}` + "\n",
        },
        {
            name:           "software image updates needed",
            apiKey:         "valid-key",
            deviceID:       "dev1",
            tenantID:       "1",
	    body:           `{"software": {"version":"v1.2.2"},"rules": {"version":"r1.2.3"},"threatintel":{"version":"2025.04.10.153020"}}`,
            expectedStatus: http.StatusOK,
            expectedBody: fmt.Sprintf(
	        `{"software":{"version":"v1.2.3","size":1024,"sha256":"sw-sha256","source":"device","download_url":%q}}` + "\n",
                DownloadURLFormat(1, "software", "hndr-sw", "v1.2.3"),
	    ),
        },
        {
            name:           "rules updates needed",
            apiKey:         "valid-key",
            deviceID:       "dev1",
            tenantID:       "1",
	    body:           `{"software": {"version":"v1.2.3"},"rules": {"version":"r1.2.2"},"threatintel":{"version":"2025.04.10.153020"}}`,
            expectedStatus: http.StatusOK,
            expectedBody: fmt.Sprintf(
	        `{"rules":{"version":"r1.2.3","size":512,"sha256":"rules-sha256","download_url":%q}}` + "\n",
                DownloadURLFormatRules(1, "rules", "hndr-rules", "r1.2.3"),
	    ),
        },
        {
            name:           "threal intel updates needed",
            apiKey:         "valid-key",
            deviceID:       "dev1",
            tenantID:       "1",
	    body:           `{"software": {"version":"v1.2.3"},"rules": {"version":"r1.2.3"},"threatintel":{"version":"2025.04.10.153000"}}`,
            expectedStatus: http.StatusOK,
            expectedBody: fmt.Sprintf(
	        `{"threatintel":{"version":"2025.04.10.153010","size":256,"sha256":"ti-sha256","download_url":%q}}` + "\n",
                DownloadURLFormat(1, "threatintel", "threatintel", "2025.04.10.153010"),
	    ),
        },
        {
            name:           "Invalid API key",
            apiKey:         "invalid-key",
            deviceID:       "dev1",
            tenantID:       "1",
	    body:           `{"software": {"version":"v1.2.2"},"rules": {"version":"r1.2.3"},"threatintel":{"version":"2025.04.10.153000"}}`,
            expectedStatus: http.StatusUnauthorized,
	    expectedBody:   "Unauthorized: failed to validate API key\n",
        },
        {
            name:           "Invalid device ID",
            apiKey:         "valid-key",
            deviceID:       "dev2",
            tenantID:       "1",
	    body:           `{"software": {"version":"v1.2.2"},"rules": {"version":"r1.2.3"},"threatintel":{"version":"2025.04.10.153000"}}`,
            expectedStatus: http.StatusUnauthorized,
	    expectedBody:   "Unauthorized: failed to validate device id\n",
        },
        {
            name:           "Invalid JSON body",
            apiKey:         "valid-key",
            deviceID:       "dev1",
            tenantID:       "1",
	    body:           `{"software":{"version":"v1.2.2"}, "rules":{"version": "invalid json"`,
            expectedStatus: http.StatusBadRequest,
            expectedBody:   "Invalid request body\n",
        },
        {
            name:           "global software image updates needed",
            apiKey:         "valid-key-2",
            deviceID:       "dev2",
            tenantID:       "1",
	    body:           `{"software": {"version":"v1.2.2"},"rules": {"version":"r1.2.3"},"threatintel":{"version":"2025.04.10.153020"}}`,
            expectedStatus: http.StatusOK,
            expectedBody: fmt.Sprintf(
	        `{"software":{"version":"v1.2.4","size":1234,"sha256":"sw-sha256","source":"latest","download_url":%q}}` + "\n",
                DownloadURLFormat(1, "software", "hndr-sw", "v1.2.4"),
	    ),
        },

    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req, err := http.NewRequest("POST", "/v1/updates/"+tt.tenantID, bytes.NewBufferString(tt.body))
            if err != nil {
                t.Fatal(err)
            }
            req.Header.Set("X-API-KEY", tt.apiKey)
            req.Header.Set("X-DEVICE-ID", tt.deviceID)

            rr := httptest.NewRecorder()
            server.handleUpdates(rr, req)

            if rr.Code != tt.expectedStatus {
                t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
            }
            if body := rr.Body.String(); body != tt.expectedBody {
                t.Errorf("Expected body %q, got %q", tt.expectedBody, body)
            }
        })
    }
    defer db.Close()
}

func TestStatus(t *testing.T) {
    cleanupTestDB(t)
    db := setupTestDB(t)

    server := &Server{db: db}
    tests := []struct {
        name           string
        url            string
        method         string
        apiKey         string
        deviceID       string
	body           string
        expectedStatus int
        expectedBody   string
    }{
        {
            name:           "Update existing device status",
            url:            "/v1/status/1",
            method:         "POST",
            apiKey:         "valid-key",
            deviceID:       "dev1",
            body:           `{"software": {"status":"success"},"rules": {"status":"failure"},"threatintel":{"status":"success"}}`,
            expectedStatus: http.StatusOK,
            expectedBody:   "{\"status\":\"ok\"}\n",
        },
        {
            name:           "Verify update of existing device status",
            url:            "/v1/status/1",
            method:         "GET",
            apiKey:         "valid-key",
            deviceID:       "dev1",
            body:           "",
            expectedStatus: http.StatusOK,
            expectedBody:   `{"software":{"status":"success"},"rules":{"status":"failure"},"threatintel":{"status":"success"}}` + "\n",
        },
        {
            name:           "Update new device status",
            url:            "/v1/status/1",
            method:         "POST",
            apiKey:         "valid-key-2",
            deviceID:       "dev2",
            body:           `{"software": {"status":"SUCCESS"},"rules": {"status":"FAILURE"},"threatintel":{"status":"success"}}`,
            expectedStatus: http.StatusOK,
            expectedBody:   "{\"status\":\"ok\"}\n",
        },
        {
            name:           "Verify update of new device status",
            url:            "/v1/status/1",
            method:         "GET",
            apiKey:         "valid-key-2",
            deviceID:       "dev2",
            body:           "",
            expectedStatus: http.StatusOK,
            expectedBody:   `{"software":{"status":"SUCCESS"},"rules":{"status":"FAILURE"},"threatintel":{"status":"success"}}` + "\n",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var err error
            var req *http.Request

            if tt.method == "POST" {
                req, err = http.NewRequest("POST", tt.url, bytes.NewBufferString(tt.body))
            } else {
                req, err = http.NewRequest("GET", tt.url, nil)
            }
            if err != nil {
                t.Fatal(err)
            }
            req.Header.Set("X-API-KEY", tt.apiKey)
            req.Header.Set("X-DEVICE-ID", tt.deviceID)

            rr := httptest.NewRecorder()
            server.handleStatus(rr, req)

            if rr.Code != tt.expectedStatus {
                t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
            }
            if body := rr.Body.String(); body != tt.expectedBody {
                t.Errorf("Expected body %q, got %q", tt.expectedBody, body)
            }
        })
    }
    defer db.Close()
}
