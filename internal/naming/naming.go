package naming

// Package naming provides centralized generation of short deterministic hashes
// used across Kubernetes resource names, labels, annotations and provider
// driver cloud resource names / tags. Keeping the logic here allows future
// changes (length/algorithm) without touching call sites.

import (
	"crypto/sha1"
	"fmt"
)

// Hashes groups hierarchical short hashes derived from service, provider,
// cluster and app identifiers.
//
// Mapping (semantic scope -> field):
//
//	service                          -> Service
//	service/provider                 -> Provider
//	service/provider/cluster         -> Cluster
//	service/provider/app             -> AppID (cluster independent)
//	service/provider/cluster/app     -> AppInstance (cluster dependent)
type Hashes struct {
	Service     string
	Provider    string
	Cluster     string
	AppID       string
	AppInstance string
	Namespace   string
}

// defaultLength defines the hex length of hashes (bits ~ length * 4).
const defaultLength = 6

// ShortHash returns the hex SHA1 prefix of length n (clamped to digest size).
func ShortHash(s string, n int) string {
	sum := sha1.Sum([]byte(s))
	h := fmt.Sprintf("%x", sum)
	if n > len(h) {
		n = len(h)
	}
	return h[:n]
}

// VolumeHash returns a short hash for a volume handle or identifier.
func VolumeHash(handle string) string {
	return ShortHash(handle, defaultLength)
}

// NewHashes computes hierarchical hashes for the given identifiers.
func NewHashes(service, provider, cluster, app string) Hashes {
	h := Hashes{
		Service:     ShortHash(service, defaultLength),
		Provider:    ShortHash(fmt.Sprintf("%s:%s", service, provider), defaultLength),
		Cluster:     ShortHash(fmt.Sprintf("%s:%s:%s", service, provider, cluster), defaultLength),
		AppID:       ShortHash(fmt.Sprintf("%s:%s:%s", service, provider, app), defaultLength),
		AppInstance: ShortHash(fmt.Sprintf("%s:%s:%s:%s", service, provider, cluster, app), defaultLength),
	}
	h.Namespace = fmt.Sprintf("kompox-%s-%s", app, h.AppID)
	return h
}

// VolumeResourceName returns the default resource name used for both PV and PVC
// generated from a logical volume and its provider-specific handle.
// The format is:
//
//	kompox-<volumeName>-<AppID>-<volHASH>
//
// where volHASH is derived from the handle.
func (h Hashes) VolumeResourceName(volumeName, handle string) string {
	volHASH := VolumeHash(handle)
	return fmt.Sprintf("kompox-%s-%s-%s", volumeName, h.AppID, volHASH)
}
