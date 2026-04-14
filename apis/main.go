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
// Created On:       04/14/2026
//
// Entry point: loads config, wires up the server, starts the HTTP listener.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"orion/common"
)

func main() {
	configPath     := flag.String("config",      "config.json", "Path to DB config file")
	authConfigPath := flag.String("auth-config", "auth.json",   "Path to auth config file")
	flag.Parse()

	// Load DB config.
	dbFile, err := os.Open(*configPath)
	if err != nil {
		log.Fatalf("Error opening config file: %v", err)
	}
	defer dbFile.Close()
	var cfg common.DBConfig
	if err := json.NewDecoder(dbFile).Decode(&cfg); err != nil {
		log.Fatalf("Error parsing config: %v", err)
	}

	// Load auth config.
	authFile, err := os.Open(*authConfigPath)
	if err != nil {
		log.Fatalf("Error opening auth config file: %v", err)
	}
	defer authFile.Close()
	var authCfg common.AuthConfig
	if err := json.NewDecoder(authFile).Decode(&authCfg); err != nil {
		log.Fatalf("Error parsing auth config: %v", err)
	}
	if authCfg.JWTSecret == "" {
		log.Fatalf("auth config: jwt_secret must not be empty")
	}

	dbPath := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
	)

	log.Println("DB path =", dbPath)
	server, err := NewServer(dbPath, cfg.GetEnvironment(), authCfg)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	mux := server.newMux()
	log.Println("Starting API server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
