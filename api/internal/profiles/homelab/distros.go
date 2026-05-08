package homelab

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Distro describes a cloud image entry — either a curated catalog entry or
// a custom image resolved at request time.
type Distro struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	FileName string `json:"file_name"`
}

// Catalog is the curated set of cloud images supported by the homelab profile.
// Hashes are pinned upstream; refresh manually when bumping image versions.
//
// FileName uses .img not .qcow2: Proxmox's download-url API rejects .qcow2
// under content_type=iso ("Parameter verification failed: wrong file
// extension"). Bytes are unchanged — Proxmox stores the qcow2 payload under
// the .img name and the VM disk block consumes it fine.
var Catalog = map[string]Distro{
	"rocky9": {
		ID:       "rocky9",
		Label:    "Rocky Linux 9",
		URL:      "https://dl.rockylinux.org/pub/rocky/9/images/x86_64/Rocky-9-GenericCloud.latest.x86_64.qcow2",
		SHA256:   "15d81d3434b298142b2fdd8fb54aef2662684db5c082cc191c3c79762ed6360c",
		FileName: "Rocky-9-GenericCloud.latest.x86_64.img",
	},
}

var sha256Re = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// proxmoxSafeFileName swaps disk-image extensions Proxmox rejects under
// content_type=iso (.qcow2, .qcow, .raw) for .img. Other names pass through.
func proxmoxSafeFileName(name string) string {
	for _, ext := range []string{".qcow2", ".qcow", ".raw"} {
		if strings.HasSuffix(name, ext) {
			return strings.TrimSuffix(name, ext) + ".img"
		}
	}
	return name
}

// ResolveImage chooses between a catalog distro and a custom URL+SHA pair.
// Exactly one of distroID or customURL must be set.
func ResolveImage(distroID, customURL, customSHA string) (Distro, error) {
	distroID = strings.TrimSpace(distroID)
	customURL = strings.TrimSpace(customURL)
	customSHA = strings.TrimSpace(customSHA)

	switch {
	case distroID == "" && customURL == "":
		return Distro{}, errors.New("either distro or custom_url is required")
	case distroID != "" && customURL != "":
		return Distro{}, errors.New("distro and custom_url are mutually exclusive")
	}

	if distroID != "" {
		d, ok := Catalog[distroID]
		if !ok {
			return Distro{}, fmt.Errorf("unknown distro: %q", distroID)
		}
		return d, nil
	}

	// Custom URL path.
	if customSHA == "" {
		return Distro{}, errors.New("custom_sha256 is required when custom_url is set")
	}
	if !sha256Re.MatchString(customSHA) {
		return Distro{}, errors.New("custom_sha256 must be 64 hex characters")
	}

	fileName := path.Base(customURL)
	if fileName == "" || fileName == "." || fileName == "/" {
		fileName = "custom-image.img"
	}
	fileName = proxmoxSafeFileName(fileName)

	return Distro{
		ID:       "custom",
		Label:    "Custom image",
		URL:      customURL,
		SHA256:   customSHA,
		FileName: fileName,
	}, nil
}
