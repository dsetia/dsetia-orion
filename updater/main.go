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

package main

import (
    "context"
    "crypto/tls"
    "flag"
    "log"
    "net/http"
    "os"
    "os/signal"
    "time"
    "updater/core"
    "orion/common"
)

type CmdArguments struct {
    config  string
    daemon  bool
    verbose bool
}

type UpdateItems struct {
    Url    string
    status string
}

type UpdateInfo struct {
    Name  string
    Items UpdateItems
}

func ParseArgs() CmdArguments {
    // Define flags and their default values.
    configPtr := flag.String("config", "/opt/hndr/updater/config/updater-config.json", "Path to config file")
    daemonPtr := flag.Bool("deamon", false, "Run in foreground")
    verbosePtr := flag.Bool("verbose", false, "Enable verbose mode")

    // Parse the command-line arguments.
    flag.Parse()

    cmdArgs := CmdArguments{config: *configPtr, daemon: *daemonPtr, verbose: *verbosePtr}
    return cmdArgs
}

func CheckUpdates(updaterCfg core.UpdaterConfig, snrCfg core.SensorConfig) {
    log.Println("Updater config: ", updaterCfg)
    log.Println("Sensor config: ", snrCfg)

    apiClient := &core.Client{
        BaseURL:  updaterCfg.APIServerURL,
        APIKey:   snrCfg.ApiKey,
        DeviceID: snrCfg.DeviceID,
        HTTPClient: &http.Client{
            Timeout: time.Duration(updaterCfg.APIServerTimeout) * time.Second,
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{InsecureSkipVerify: updaterCfg.CertificateVerifySkip},
            },
        },
    }

    // Explicitly close the connection, open new connection in next poll interval.
    // If the poll interval changes to higher frequency consider reusing the connection.
    defer apiClient.HTTPClient.CloseIdleConnections()

    log.Println("Authenticating...")
    if err := apiClient.Authenticate(snrCfg.TenantID); err != nil {
        log.Println("Auth failed:", err)
        return
    }
    log.Println("Authenticated successfully.")

    // Read the existing update config
    var updtCfg core.HndrConfig
    if err := core.LoadJSONConfig(updaterCfg.HndrConfig, &updtCfg); err != nil {
        return
    }
    log.Println("Updater config: ", updtCfg)

    // Make request to update service with existing config
    log.Println("Fetching updates manifest...")
    updates, err := apiClient.GetUpdateManifest(snrCfg.TenantID, updtCfg)
    if err != nil {
        log.Println("Error fetching updates:", err)
        return
    }
    log.Printf("Update manifest: %+v\n", updates)

    upInfo := common.StatusRequest{
        Software: struct {
            Status string `json:"status"`
        }{Status: ""},
        Rules: struct {
            Status string `json:"status"`
        }{Status: ""},
        ThreatIntel: struct {
            Status string `json:"status"`
        }{Status: ""},
    }
    sendSts := false
    if updates.Software != nil && len(updates.Software.DownloadURL) != 0 {
        log.Printf("url: %s\n", updates.Software.DownloadURL)
        sendSts = true
        log.Println("Fetching software...")
        content, err := apiClient.DownloadFile(updates.Software.DownloadURL)
        if err != nil {
            log.Println("Error downloading software :", err)
            upInfo.Software.Status = "FAILED"
        } else {
            status, err := core.UpateSoftwareNow(content, updates.Software.Version, updates.Software.Digest, updates.Software.DownloadURL, updaterCfg)
            upInfo.Software.Status = status
            if err != nil {
                log.Println("Error updating software :", err)
            }
        }

    }
    if updates.Rules != nil && len(updates.Rules.DownloadURL) != 0 {
        log.Println("Fetching Rules...")
        sendSts = true

        content, err := apiClient.DownloadFile(updates.Rules.DownloadURL)
        if err != nil {
            log.Println("Error downloading rules :", err)
            upInfo.Rules.Status = "FAILED"
        } else {
            status, err := core.UpateRulesNow(content, updates.Rules.Version, updates.Rules.Digest, updates.Rules.DownloadURL, updaterCfg)
            upInfo.Rules.Status = status
            if err != nil {
                log.Println("Error updating rules :", err)
            }
        }
    }
    if updates.ThreatIntel != nil && len(updates.ThreatIntel.DownloadURL) != 0 {
        log.Println("Fetching ThreatIntel...")
        sendSts = true
        content, err := apiClient.DownloadFile(updates.ThreatIntel.DownloadURL)
        if err != nil {
            log.Println("Error downloading threatintel :", err)
            upInfo.ThreatIntel.Status = "FAILED"
        } else {
            status, err := core.UpateThreatIntelNow(content, updates.ThreatIntel.Version, updates.ThreatIntel.Digest, updates.ThreatIntel.DownloadURL, updaterCfg)
            upInfo.ThreatIntel.Status = status
            if err != nil {
                log.Println("Error updating rules :", err)
            }
        }
    }

    if sendSts {
        log.Println("Sending update status...")
        err = apiClient.SendStatus(snrCfg.TenantID, upInfo)
        if err != nil {
            log.Println("Failed to send status:", err)
            return
        }
        log.Println("Status update sent successfully.")
    }
}

func main() {
    cmdArgs := ParseArgs()
    log.Println("Cmdline Args:", cmdArgs)

    var updaterCfg core.UpdaterConfig
    if err := core.LoadJSONConfig(cmdArgs.config, &updaterCfg); err != nil {
        os.Exit(1)
    }
    log.Println("Updater config: ", updaterCfg)

    var snrCfg core.SensorConfig
    if err := core.LoadJSONConfig(updaterCfg.SensorConfig, &snrCfg); err != nil {
        os.Exit(1)
    }
    log.Println("Sensor config: ", snrCfg)

    ctx := context.Background()

    // trap Ctrl+C and call cancel on the context
    ctx, cancel := context.WithCancel(ctx)
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)

    defer func() {
        signal.Stop(c)
        cancel()
    }()

    go func() {
        select {
        case <-c:
            log.Println("Interrupt received, wait for process to complete before exiting")
            cancel()
        case <-ctx.Done():
        }
    }()

    duration := time.Duration(updaterCfg.PollIntervalMins) * time.Minute
    log.Printf("Poll interval in %+vm", duration)
    // Run update once before entering poll interval
    CheckUpdates(updaterCfg, snrCfg)
    for {
        select {
        case <-time.After(duration):
            CheckUpdates(updaterCfg, snrCfg)
        case <-ctx.Done():
            log.Println("We're done here!")
            os.Exit(0)
        }
    }
}
