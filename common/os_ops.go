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
// Created On:       05/26/2025

package common

import (
    "errors"
    "io/fs"
    "log"
    "os"
)

func DescriptiveError(err error) {
    var pathError *fs.PathError
    if errors.As(err, &pathError) {
        log.Println("Operation:", pathError.Op)
        log.Println("Path:", pathError.Path)
        log.Println("Error:", pathError.Err)
    }
}

// File create
func FileCreate(file string) (*os.File, error) {
    outFile, err := os.Create(file)
    if err != nil {
        log.Printf("Error: creating file %s: %w", file, err)
        DescriptiveError(err)
        return outFile, err
    }
    return outFile, nil
}

// File change mod
func FileChmod(file string, mode uint32) error {
    filePermissions := os.FileMode(mode) 
    err := os.Chmod(file, filePermissions)
    if err != nil {
        log.Printf("Error: chmod file %s: %w", file, err)
        DescriptiveError(err)
        return err
    }
    log.Printf("Permissions for '%s' changed to %o", file, filePermissions)
    return nil
}

// File Stat
func FileExists(file string) error {
    fileInfo, err := os.Stat(file)
    if err != nil {
        log.Printf("Error: stat file %s: %w", file, err)
        DescriptiveError(err)
        return err
    }
    log.Printf("File '%s' exits, with permissions '%o'", file, fileInfo.Mode().Perm())
    return nil
}
