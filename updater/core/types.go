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

type UpdaterConfig struct {
    UpdateLock            string `json:"update_lock"`
    HndrSymlink           string `json:"hndr_symlink"`
    HndrConfig            string `json:"hndr_config"`
    SensorConfig          string `json:"sensor_config"`
    FolderOne             string `json:"folder_one"`
    FolderTwo             string `json:"folder_two"`
    ScratchFolder         string `json:"scratch_folder"`
    APIServerURL          string `json:"api_server_url"`
    APIServerPort         int    `json:"api_server_port"`
    PollIntervalMins      int    `json:"poll_interval_mins"`
    APIServerTimeout      int    `json:"api_server_timeout"`
    CertificateVerifySkip bool   `json:"certificate_verify_skip"`
    Daemonize             bool   `json:"daemonize"`
    Verbose               bool   `json:"verbose"`
}

type UpdateRequest struct {
    Software struct {
        Version string `json:"version"`
    } `json:"software"`

    Rules struct {
        Version string `json:"version"`
    } `json:"rules"`

    ThreatIntel struct {
        Version string `json:"version"`
    } `json:"threat_intel"`
}

type HndrConfig UpdateRequest

type SensorConfig struct {
    ApiKey     string `json:"api_key"`
    DeviceID   string `json:"device_id"`
    LicenseKey string `json:"license_key"`
    TenantID   string `json:"tenant_id"`
}

type StatusRequest struct {
    Software struct {
        Status string `json:"status"`
    } `json:"software"`

    Rules struct {
        Status string `json:"status"`
    } `json:"rules"`

    ThreatIntel struct {
        Status string `json:"status"`
    } `json:"threatintel"`
}

type ComponentUpdate struct {
    Version string `json:"version"`
    URL     string `json:"url"`
    Size    int    `json:"size"`
    Type    string `json:"type"`
}

type UpdateResponse struct {
    Software struct {
        Version     string `json:"version"`
        Size        int    `json:"size"`
        SHA256      string `json:"sha256"`
        Source      string `json:"source"`
        DownloadURL string `json:"download_url"`
    } `json:"software"`

    Rules struct {
        Version     string `json:"version"`
        Size        int    `json:"size"`
        SHA256      string `json:"sha256"`
        DownloadURL string `json:"download_url"`
    } `json:"rules"`

    ThreatIntel struct {
        Version     string `json:"version"`
        Size        int    `json:"size"`
        SHA256      string `json:"sha256"`
        DownloadURL string `json:"download_url"`
    } `json:"threat_intel"`
}

type Configuration struct {
    Users  []string
    Groups []string
}
