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
// File Owner:       sumanth@securite.world
// Created On:       04/23/2025

package core

import (
    "encoding/json"
    "log"
    "os"
)

// LoadJSONConfig loads a JSON file into any provided struct pointer
func LoadJSONConfig(path string, out any) error {
    log.Println("loading json: ", path)
    f, err := os.Open(path)
    if err != nil {
        log.Printf("failed to open file: %w", err)
        return err
    }
    defer f.Close()

    if err := json.NewDecoder(f).Decode(out); err != nil {
        log.Printf("failed to decode JSON: %w", err)
        return err
    }
    return nil
}

// SaveJSONConfig writes any struct to a JSON file with indentation
func SaveJSONConfig(path string, in any) error {
    f, err := os.Create(path)
    if err != nil {
        log.Printf("failed to create file: %w", err)
        return err
    }
    defer f.Close()

    encoder := json.NewEncoder(f)
    encoder.SetIndent("", "  ") // pretty print
    if err := encoder.Encode(in); err != nil {
        log.Printf("failed to encode JSON: %w", err)
        return err
    }
    return nil
}
