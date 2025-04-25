package common

type DeviceVersions struct {
    Image struct {
        Version string `json:"version"`
    } `json:"image"`
    Rules struct {
        Version string `json:"version"`
    } `json:"rules"`
    Threatfeed struct {
        Version string `json:"version"`
    } `json:"threatfeed"`
}

// /v1/status request
type DeviceStatus struct {
    Image struct {
        Status string `json:"status"`
    } `json:"image"`
    Rules struct {
        Status string `json:"status"`
    } `json:"rules"`
    Malware struct {
        Status string `json:"status"`
    } `json:"Malware"`
}


// UpdateResponse represents the /v1/update response
type UpdateResponse struct {
    Software      *SoftwareVersion `json:"image,omitempty"`
    Rules         *VersionInfo     `json:"rules,omitempty"`
    ThreatIntel   *VersionInfo     `json:"threatfeed,omitempty"`
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
