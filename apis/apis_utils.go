package main

import "fmt"

// DownloadURLFormat generates a download URL for the given tenant ID, type, prefix, and version.
// resourceType is images, rules, or threatintel
// prefix is hndr-sw, hndr-rules, or threatintel
// Returns string like /v1/download/1/images/hndr-sw-v1.2.3.tar.gz
func DownloadURLFormat(tenantID int64, resourceType, prefix, version string) string {
    return fmt.Sprintf("/v1/download/%d/%s/%s-%s.tar.gz", tenantID, resourceType, prefix, version)
}
