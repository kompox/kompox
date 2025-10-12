package naming

// Package naming provides centralized generation of short deterministic hashes
// used across Kubernetes resource names, labels, annotations and provider
// driver cloud resource names / tags. Keeping the logic here allows future
// changes (length/algorithm) without touching call sites.

import (
	"crypto/sha256"
	"fmt"
	"math/big"
)

// Hashes groups hierarchical short hashes derived from workspace, provider,
// cluster and app identifiers.
//
// Mapping (semantic scope -> field):
//
//	workspace                          -> Workspace
//	workspace/provider                 -> Provider
//	workspace/provider/cluster         -> Cluster
//	workspace/provider/app             -> AppID (cluster independent)
//	workspace/provider/cluster/app     -> AppInstance (cluster dependent)
type Hashes struct {
	Workspace   string
	Provider    string
	Cluster     string
	AppID       string
	AppInstance string
	Namespace   string
}

// defaultLength defines the base36 length of hashes.
const defaultLength = 6

// ShortHash returns the base36 SHA256 prefix of length n.
func ShortHash(s string, n int) string {
	sum := sha256.Sum256([]byte(s))
	bigInt := new(big.Int).SetBytes(sum[:])
	h := bigInt.Text(36)
	for len(h) < n {
		h = "0" + h
	}
	if len(h) > n {
		h = h[:n]
	}
	return h
}

// VolumeHash returns a short hash for a volume handle or identifier.
func VolumeHash(handle string) string {
	return ShortHash(handle, defaultLength)
}

// NewHashes computes hierarchical hashes for the given identifiers.
func NewHashes(workspace, provider, cluster, app string) Hashes {
	h := Hashes{
		Workspace:   ShortHash(workspace, defaultLength),
		Provider:    ShortHash(fmt.Sprintf("%s:%s", workspace, provider), defaultLength),
		Cluster:     ShortHash(fmt.Sprintf("%s:%s:%s", workspace, provider, cluster), defaultLength),
		AppID:       ShortHash(fmt.Sprintf("%s:%s:%s", workspace, provider, app), defaultLength),
		AppInstance: ShortHash(fmt.Sprintf("%s:%s:%s:%s", workspace, provider, cluster, app), defaultLength),
	}
	// Namespace naming format:
	//   k4x-<spHASH>-<appName>-<idHASH>
	// This is used also as a cloud resource group base name in some providers.
	h.Namespace = fmt.Sprintf("k4x-%s-%s-%s", h.Provider, app, h.AppID)
	return h
}

// VolumeResourceName returns the default resource name used for both PV and PVC
// generated from a logical volume and its provider-specific handle.
// The format is:
//
//	k4x-<spHASH>-<volumeName>-<AppID>-<volHASH>
//
// where volHASH is derived from the handle.
func (h Hashes) VolumeResourceName(volumeName, handle string) string {
	volHASH := VolumeHash(handle)
	return fmt.Sprintf("k4x-%s-%s-%s-%s", h.Provider, volumeName, h.AppID, volHASH)
}
