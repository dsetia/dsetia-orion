package common

import (
    "fmt"
    "encoding/json"
)

type DeviceVersions struct {
    Software struct {
        Version string `json:"version"`
        Digest  string `json:"digest"`
    } `json:"software"`
    Rules struct {
        Version string `json:"version"`
        Digest  string `json:"digest"`
    } `json:"rules"`
    ThreatIntel struct {
        Version string `json:"version"`
        Digest  string `json:"digest"`
    } `json:"threatintel"`
}

// /v1/status request
type DeviceStatus struct {
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

type UpdateRequest DeviceStatus
type StatusRequest DeviceStatus

// UpdateResponse represents the /v1/update response
type UpdateResponse struct {
    Software      *SoftwareVersion `json:"software,omitempty"`
    Rules         *VersionInfo     `json:"rules,omitempty"`
    ThreatIntel   *VersionInfo     `json:"threatintel,omitempty"`
}

func (u UpdateResponse) String() string {
    b, err := json.MarshalIndent(u, "", "  ")
    if err != nil {
        return "Error marshaling UpdateResponse: " + err.Error()
    }
    return string(b)
}

// SoftwareVersion includes hndr_sw details
type SoftwareVersion struct {
    Version string `json:"version"`
    Size    int64  `json:"size"`
    Digest  string `json:"digest"`
    Source  string `json:"source"` // "device" or "latest"
    DownloadURL string `json:"download_url"`
}

// VersionInfo includes version details for rules and threatintel
type VersionInfo struct {
    Version string `json:"version"`
    Size    int64  `json:"size"`
    Digest  string `json:"digest"`
    DownloadURL string `json:"download_url"`
}

type DBConfig struct {
    Host     string `json:"host"`
    Port     int    `json:"port"`
    User     string `json:"user"`
    Password string `json:"password"`
    DBName   string `json:"dbname"`
    SSLMode  string `json:"sslmode"`
}

func (c DBConfig) ConnString() string {
    return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

type MinioConfig struct {
    Endpoint  string `json:"endpoint"`
    AccessKey string `json:"user"`
    SecretKey string `json:"password"`
    UseSSL    bool   `json:"usessl"`
}
