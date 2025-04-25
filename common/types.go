package common

import (
    "encoding/json"
)

type DeviceVersions struct {
    Software struct {
        Version string `json:"version"`
    } `json:"software"`
    Rules struct {
        Version string `json:"version"`
    } `json:"rules"`
    ThreatIntel struct {
        Version string `json:"version"`
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
    b, _ := json.MarshalIndent(u, "", "  ")
    return string(b)
}

// SoftwareVersion includes hndr_sw details
type SoftwareVersion struct {
    Version string `json:"version"`
    Size    int64  `json:"size"`
    Sha256  string `json:"sha256"`
    Source  string `json:"source"` // "device" or "latest"
    DownloadURL string `json:"download_url"`
}

// VersionInfo includes version details for rules and threatintel
type VersionInfo struct {
    Version string `json:"version"`
    Size    int64  `json:"size"`
    Sha256  string `json:"sha256"`
    DownloadURL string `json:"download_url"`
}
