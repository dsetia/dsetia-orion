package main

import (
    "testing"
    "log"

    _ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) *DB {
    dbPath := "postgres://pguser:pgpass@localhost:5432/pgdb?sslmode=disable"
    db, err := NewDB(dbPath)
    if err != nil {
        t.Fatalf("Failed to create test DB: %v", err)
    }
    return db
}

func TestDB(t *testing.T) {
    db := setupTestDB(t)

    err := db.Ping()
    if err != nil {
    	log.Fatal("Ping failed:", err)
    }
    log.Println("✅ Successfully connected to PostgreSQL!")
    defer db.Close()
}
