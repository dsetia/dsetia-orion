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
    "archive/tar"
    "compress/gzip"
    "errors"
    "io"
    "log"
    "os"
    "os/exec"
    "orion/common"
    "path/filepath"
    "strings"
)

// ExtractTarGz extracts a .tar.gz file to a specified directory.
func ExtractTarGz(tarGzPath, destDir string) error {
    log.Println("Extracting file:", tarGzPath)
    file, err := os.Open(tarGzPath)
    if err != nil {
        log.Printf("opening %s: %w", tarGzPath, err)
        return err
    }
    defer file.Close()

    gzReader, err := gzip.NewReader(file)
    if err != nil {
        log.Printf("creating gzip reader: %w", err)
        return err
    }
    defer gzReader.Close()

    tarReader := tar.NewReader(gzReader)

    for {
        header, err := tarReader.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Printf("reading tar header: %w", err)
            return err
        }

        target := filepath.Join(destDir, header.Name)

        switch header.Typeflag {
        case tar.TypeDir:
            log.Printf("Creating directory %s", target)
            err = os.MkdirAll(target, 0755)
            if err != nil {
                log.Printf("Error: creating directory %s: %w", target, err)
                common.DescriptiveError(err)
                return err
            }
        case tar.TypeReg:
            log.Printf("Creating file %s", target)
            outFile, err := common.FileCreate(target)
            if err != nil {
                return err
            }
            defer outFile.Close()

            log.Printf("Writing file %s", target)
            _, err = io.Copy(outFile, tarReader)
            if err != nil {
                log.Printf("Error: writing file %s: %w", target, err)
                common.DescriptiveError(err)
                return err
            }

            if strings.Contains(target, "bin/suricata") {
                log.Printf("Change file permissions %s", target)
                err = common.FileChmod(target, 0755)
                if err != nil {
                    return err
                }

                // Verify the change
                err = common.FileExists(target)
                if err != nil {
                    return err
                }
            }

        case tar.TypeSymlink:
            err = os.Symlink(header.Linkname, target)
            if err != nil {
                log.Printf("creating symlink %s -> %s: %w", target, header.Linkname, err)
                common.DescriptiveError(err)
                return err
            }
        }
    }

    return nil
}

func DeleteFile(swFilePath string) {
    common.FileRemove(swFilePath)
}

func GetRealPath(symlinkPath string) (string, error) {
    realPath, err := filepath.EvalSymlinks(symlinkPath)
    if err != nil {
        log.Println("Error:", err)
        return "", err
    }

    absRealPath, err := filepath.Abs(realPath)
    if err != nil {
        log.Println("Error:", err)
        return "", err
    }

    return absRealPath, err
}

func GetFolderName(absRealPath string) string {
    folderName := absRealPath[strings.LastIndex(absRealPath, "/")+1:]
    return folderName
}

func GetFolderToDeploy(absRealPath, folderOne, folderTwo string) string {
    if strings.EqualFold(absRealPath, folderOne) {
        return folderTwo
    } else if strings.EqualFold(absRealPath, folderTwo) {
        return folderOne
    }

    return ""
}

func ExecuteSupervisorCmd(cmd, serviceName string) error {
    executor := exec.Command("supervisorctl", cmd, serviceName)
    output, err := executor.CombinedOutput()
    if err != nil {
        log.Printf("Error: failed to '%s' service '%s': %v\n  Output: %s", cmd, serviceName, err, string(output))
        return err
    }
    log.Printf("Successfully '%s' service '%s'\n  Output: %s", cmd, serviceName, string(output))
    return nil
}

func UnlinkAndLink(symlinkPath, absRealPath string) error {
    err := os.Remove(symlinkPath)
    if err != nil {
        log.Println("Error removing symlink:", err)
        return err
    }
    log.Println("Symlink removed successfully.")

    err = os.Symlink(absRealPath, symlinkPath)
    if err != nil {
        log.Println("Error creating symlink:", err)
        return err
    }
    log.Println("Symlink created successfully.")
    return err
}

func CleanupFolder(dirPath, subDirPath string) error {
    absDirPath := dirPath + "/" + subDirPath
    log.Println("Cleaning up folder: ", absDirPath)
    err := os.RemoveAll(absDirPath)
    if err != nil {
        log.Printf("Error: Removing folder %s: %w", absDirPath, err)
        return err
    }
    return err
}

func CleanupFilesInFolder(dirPath, wildcardMatch string) error {
    filesPath := dirPath + wildcardMatch
    log.Println("Cleaning up files: ", filesPath)
    files, err := filepath.Glob(filesPath)
    if err != nil {
        log.Printf("Error: listing folder %s: %w", filesPath, err)
        return err
    }
    for _, f := range files {
        if err := os.Remove(f); err != nil {
            log.Printf("Error: removing file %s: %w", f, err)
            return err
        }
    }
    return err
}

func CleanupSoftwareFolder(dirPath string) error {
    err := CleanupFolder(dirPath + "/bin", "*")
    if err != nil {
        return err
    }

    CleanupFilesInFolder(dirPath + "/lib/", "lib*")
    return err
}

// WriteToFile saves the downloaded content to the specified file path.
func WriteToFile(content []byte, filepath string) error {
    // Create the file on the local filesystem
    outFile, err := os.Create(filepath)
    if err != nil {
        log.Println("Error: failed to create file:", err)
        return err
    }
    defer outFile.Close()

    // Write the byte slice content to the file
    _, err = outFile.Write(content)
    if err != nil {
        log.Printf("Error: failed to write to file: %v", err)
        return err
    }

    log.Printf("Downloaded file to: %s", filepath)
    return nil
}

func RemoveUpdateLock(filePath string) error {
    err := common.FileRemove(filePath)
    if err != nil {
        log.Println("Error removing update lock:", err)
    }
    log.Println("Removed lock file:", filePath)
    return err
}

func IsUpdateInProgress(filePath string) error {
    _, err := os.Stat(filePath)
    if os.IsNotExist(err) {
        file, err := os.Create(filePath)
        if err != nil {
            log.Println("Error creating file:", err)
            return err
        }
        defer file.Close()
        log.Println("File created successfully.")
    } else if err == nil {
        log.Println("File already exists.")
    } else {
        log.Println("Error checking file:", err)
    }
    return err
}

// Update the sensor sw using the provided binary
func UpateSoftwareNow(content []byte, swVersion, shaDigest, filePath string, config UpdaterConfig) (string, error) {
    status := "FAILED"

    defer RemoveUpdateLock(config.UpdateLock)

    err := IsUpdateInProgress(config.UpdateLock)
    if err == nil {
        log.Printf("Error update in progress, skipping", err)
        return status, errors.New("update in progress")
    }

    fileName := GetFolderName(filePath)
    if len(fileName) == 0 {
        log.Println("Error extracting file name from URL: ", filePath)
        return status, nil
    }

    swFilepath := config.ScratchFolder + "/" + fileName
    // Set a defer to remove file
    defer DeleteFile(swFilepath)
    log.Println("Writing downloaded artifacts at:", swFilepath)
    err = WriteToFile(content, swFilepath)
    if err != nil {
        log.Printf("Error saving software file: %v", err)
        return status, err
    }

    absRealPath, err := GetRealPath(config.HndrSymlink)
    if err != nil {
        log.Println("Error: Non-existent path")
        return status, err
    }

    folderName := GetFolderName(absRealPath)
    if 0 == len(folderName) {
        log.Println("Error: folder not found")
        return status, errors.New("folder not found")
    }

    folderToDeploy := GetFolderToDeploy(absRealPath, config.FolderOne, config.FolderTwo)
    if 0 == len(folderToDeploy) {
        log.Println("Error: Folder to deploy does not exist")
        return status, errors.New("Folder to deploy does not exist")
    }

    log.Println("Sym path:", config.HndrSymlink)
    log.Println("Real path:", absRealPath)
    log.Println("Folder Name:", folderName)
    log.Println("Folder to Deploy:", folderToDeploy)

    err = CleanupSoftwareFolder(folderToDeploy)
    if err != nil {
        return status, err
    }

    err = ExtractTarGz(swFilepath, folderToDeploy)
    if err != nil {
        return status, err
    }

    err = ExecuteSupervisorCmd("stop", "hndr")
    if err != nil {
        return status, err
    }

    err = UnlinkAndLink(config.HndrSymlink, folderToDeploy)
    if err != nil {
        return status, err
    }

    err = common.FileExists(folderToDeploy + "/" + config.HndrCfgFile)
    if err != nil {
        // Configuration file suricata.yaml doesn't exists, so don't try
        // restart the hNDR service.  When the configuration file updated
        // during rule updates the service will GET STArted.
        log.Println("Warn: hNDR service NOT started")
    } else {
        err = ExecuteSupervisorCmd("start", "hndr")
        if err != nil {
            return status, err
        }
    }

    //Read, update and write configuration file with latest version and digest details
    var hndrCfg HndrConfig
    if err = LoadJSONConfig(config.HndrConfig, &hndrCfg); err != nil {
        return status, err
    }
    log.Println("hndr config: ", hndrCfg)
    hndrCfg.Software.Version = swVersion
    hndrCfg.Software.Sha256 = shaDigest

    if err = SaveJSONConfig(config.HndrConfig, &hndrCfg); err != nil {
        return status, err
    }
    log.Println("hndr config updated successfully: ")

    status = "SUCCESS"
    log.Println("Status of config after Update: ", status)

    return status, err
}

// Update the sensor Rules using the provided binary
func UpateRulesNow(content []byte, rulesVersion, shaDigest, filePath string, config UpdaterConfig) (string, error) {
    status := "FAILED"
    // The rules must be deployed in the folder that is currently in use.
    defer RemoveUpdateLock(config.UpdateLock)

    err := IsUpdateInProgress(config.UpdateLock)
    if err == nil {
        log.Printf("Error update in progress, skipping", err)
        return status, errors.New("update in progress")
    }

    fileName := GetFolderName(filePath)
    if len(fileName) == 0 {
        log.Println("Error extracting file name from URL: ", filePath)
        return status, nil
    }

    swFilepath := config.ScratchFolder + "/" + fileName
    // Set a defer to remove file
    defer DeleteFile(swFilepath)
    log.Println("Writing downloaded artifacts at:", swFilepath)
    err = WriteToFile(content, swFilepath)
    if err != nil {
        log.Printf("Error saving rules file: %v", err)
        return status, err
    }

    absRealPath, err := GetRealPath(config.HndrSymlink)
    if err != nil {
        log.Println("Error: Non-existent path")
        return status, err
    }
    log.Println("Real path:", absRealPath)

    folderToDeploy := absRealPath + "/" + config.RulesFolder
    log.Println("Deployment folder:", folderToDeploy)
    err = CleanupFilesInFolder(folderToDeploy, "*.rules")
    if err != nil {
        return status, err
    }

    err = ExtractTarGz(swFilepath, folderToDeploy)
    if err != nil {
        return status, err
    }

    err = ExecuteSupervisorCmd("restart", "hndr")
    if err != nil {
        return status, err
    }

    //Read, update and write configuration file with latest version details
    var hndrCfg HndrConfig
    if err = LoadJSONConfig(config.HndrConfig, &hndrCfg); err != nil {
        return status, err
    }
    log.Println("hndr config: ", hndrCfg)
    hndrCfg.Rules.Version = rulesVersion
    hndrCfg.Rules.Sha256 = shaDigest 

    if err = SaveJSONConfig(config.HndrConfig, &hndrCfg); err != nil {
        return status, err
    }
    log.Println("hndr config updated successfully: ")

    status = "SUCCESS"
    log.Println("Status of config after Update: ", status)

    return status, err
}

// Update the  threat intel using the provided binary
func UpateThreatIntelNow(content []byte, tiVersion, shaDigest, filePath string, config UpdaterConfig) (string, error) {
    status := "SUCCESS"
    log.Println("---TI UPDATES NOT SUPPORTED YET ---, return SUCCESS")

    return status, nil
}
