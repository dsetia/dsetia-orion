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
    RulesFolder           string `json:"rules_folder"`
    IDSServiceName        string `json:"ids_service_name"`
    HndrCfgFile           string `json:"hndr_config_file"`
    ScratchFolder         string `json:"scratch_folder"`
    APIServerURL          string `json:"api_server_url"`
    APIServerPort         int    `json:"api_server_port"`
    PollIntervalMins      int    `json:"poll_interval_mins"`
    APIServerTimeout      int    `json:"api_server_timeout"`
    CertificateVerifySkip bool   `json:"certificate_verify_skip"`
    Daemonize             bool   `json:"daemonize"`
    Verbose               bool   `json:"verbose"`
}

type SensorConfig struct {
    ApiKey     string `json:"api_key"`
    DeviceID   string `json:"device_id"`
    LicenseKey string `json:"license_key"`
    TenantID   string `json:"tenant_id"`
}

type Configuration struct {
    Users  []string
    Groups []string
}

type HndrConfig struct {
    Software struct {
        Version string `json:"version"`
        Sha256 string `json:"sha256"`
    } `json:"software"`
    Rules struct {
        Version string `json:"version"`
        Sha256 string `json:"sha256"`
    } `json:"rules"`
    ThreatIntel struct {
        Version string `json:"version"`
        Sha256 string `json:"sha256"`
    } `json:"threatintel"`
}
