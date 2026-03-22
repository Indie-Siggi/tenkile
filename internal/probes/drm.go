// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DRMSystem represents a Digital Rights Management system
type DRMSystem struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	SchemeIDURI string   `json:"scheme_id_uri,omitempty"`
	KeySystem   string   `json:"key_system,omitempty"`
	MimeTypes   []string `json:"mime_types,omitempty"`
	SupportedProfiles []string `json:"supported_profiles,omitempty"`
	MaxResolution string `json:"max_resolution,omitempty"`
	RequiresSecureDecoder bool `json:"requires_secure_decoder,omitempty"`
}

// DRMSupported represents DRM support information for a device
type DRMSupported struct {
	Supported   bool     `json:"supported"`
	Systems     []string `json:"systems,omitempty"`
	Details     []DRMSystemDetail `json:"details,omitempty"`
}

// DRMSystemDetail contains detailed DRM system information
type DRMSystemDetail struct {
	System          string `json:"system"`
	Supported       bool   `json:"supported"`
	LicenseURL      string `json:"license_url,omitempty"`
	OfflineLicense  bool   `json:"offline_license,omitempty"`
	MaxResolution   string `json:"max_resolution,omitempty"`
	SecurityLevel   string `json:"security_level,omitempty"` // e.g., "L1", "L2", "L3"
}

// Predefined DRM systems
var (
	// Widevine CDM - Google's DRM solution
	Widevine = DRMSystem{
		Name:  "Widevine",
		Type:  "widevine",
		Description: "Google Widevine Content Decryption Module",
		SchemeIDURI: "urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed",
		KeySystem:   "com.widevine.alpha",
		MimeTypes: []string{
			"video/mp4; codecs=\"avc1\"",
			"video/mp4; codecs=\"avc3\"",
			"video/mp4; codecs=\"hevc\"",
			"video/mp4; codecs=\"vp9\"",
			"video/mp4; codecs=\"av01\"",
			"audio/mp4; codecs=\"mp4a\"",
			"audio/webm; codecs=\"opus\"",
			"audio/webm; codecs=\"vorbis\"",
		},
		SupportedProfiles: []string{"SD", "HD", "UHD", "UHD-1", "UHD-2"},
		MaxResolution:     "3840x2160",
		RequiresSecureDecoder: true,
	}

	// PlayReady - Microsoft's DRM solution
	PlayReady = DRMSystem{
		Name:  "PlayReady",
		Type:  "playready",
		Description: "Microsoft PlayReady DRM",
		SchemeIDURI: "urn:uuid:9a04f079-9840-4286-ab92-e65be0885f95",
		KeySystem:   "com.microsoft.playready",
		MimeTypes: []string{
			"video/mp4; codecs=\"avc1\"",
			"video/mp4; codecs=\"hevc\"",
			"audio/mp4; codecs=\"mp4a\"",
			"audio/mp4; codecs=\"ac-3\"",
			"audio/mp4; codecs=\"ec-3\"",
		},
		SupportedProfiles: []string{"SD", "HD", "UHD"},
		MaxResolution:     "3840x2160",
		RequiresSecureDecoder: true,
	}

	// FairPlay - Apple's DRM solution
	FairPlay = DRMSystem{
		Name:  "FairPlay",
		Type:  "fairplay",
		Description: "Apple FairPlay Streaming",
		SchemeIDURI: "urn:uuid:8974dbce-7be7-4c51-83f6-4007584f3c74",
		KeySystem:   "com.apple.fps",
		MimeTypes: []string{
			"video/mp4; codecs=\"avc1\"",
			"video/mp4; codecs=\"hevc\"",
			"audio/mp4; codecs=\"mp4a\"",
			"audio/mp4; codecs=\"ac-3\"",
			"audio/mp4; codecs=\"ec-3\"",
		},
		SupportedProfiles: []string{"SD", "HD", "UHD", "HDR"},
		MaxResolution:     "3840x2160",
		RequiresSecureDecoder: true,
	}
)

// GetDRMSystemByName returns a DRM system by name
func GetDRMSystemByName(name string) (*DRMSystem, bool) {
	switch strings.ToLower(name) {
	case "widevine", "widevine cdm":
		return &Widevine, true
	case "playready", "playready drm":
		return &PlayReady, true
	case "fairplay", "fairplay streaming":
		return &FairPlay, true
	default:
		return nil, false
	}
}

// GetAllDRMSystems returns all known DRM systems
func GetAllDRMSystems() []DRMSystem {
	return []DRMSystem{Widevine, PlayReady, FairPlay}
}

// CheckDRMSupport checks which DRM systems are supported
func CheckDRMSupport(probeResult string) (*DRMSupported, error) {
	var capabilities map[string]interface{}
	if err := json.Unmarshal([]byte(probeResult), &capabilities); err != nil {
		return nil, fmt.Errorf("failed to parse probe result: %w", err)
	}

	supported := &DRMSupported{
		Supported: false,
		Systems:   []string{},
		Details:   []DRMSystemDetail{},
	}

	// Check for EME (Encrypted Media Extensions) support
	eme, ok := capabilities["eme"]
	if !ok {
		// Check alternative key names
		eme, ok = capabilities["encrypted_media"]
	}
	if !ok {
		eme, ok = capabilities["drm"]
	}

	if !ok {
		return supported, nil
	}

	// Parse EME/DRM support
	emeMap, ok := eme.(map[string]interface{})
	if !ok {
		return supported, nil
	}

	// Check for supported systems
	if systems, ok := emeMap["systems"]; ok {
		if systemsList, ok := systems.([]interface{}); ok {
			for _, sys := range systemsList {
				if sysMap, ok := sys.(map[string]interface{}); ok {
					if name, ok := sysMap["name"].(string); ok {
						supported.Systems = append(supported.Systems, name)
					}
					if supportedFlag, ok := sysMap["supported"].(bool); ok {
						if supportedFlag {
							supported.Supported = true
						}
					}
				}
			}
		}
	}

	// Check for Widevine specifically
	if widevine, ok := emeMap["widevine"]; ok {
		if widevineMap, ok := widevine.(map[string]interface{}); ok {
			detail := DRMSystemDetail{
				System: "Widevine",
			}
			if supportedFlag, ok := widevineMap["supported"].(bool); ok {
				detail.Supported = supportedFlag
				if supportedFlag {
					supported.Supported = true
				}
			}
			if licenseURL, ok := widevineMap["license_url"].(string); ok {
				detail.LicenseURL = licenseURL
			}
			if offlineLicense, ok := widevineMap["offline_license"].(bool); ok {
				detail.OfflineLicense = offlineLicense
			}
			if maxRes, ok := widevineMap["max_resolution"].(string); ok {
				detail.MaxResolution = maxRes
			}
			if securityLevel, ok := widevineMap["security_level"].(string); ok {
				detail.SecurityLevel = securityLevel
			}
			supported.Details = append(supported.Details, detail)
		}
	}

	// Check for PlayReady specifically
	if playready, ok := emeMap["playready"]; ok {
		if playreadyMap, ok := playready.(map[string]interface{}); ok {
			detail := DRMSystemDetail{
				System: "PlayReady",
			}
			if supportedFlag, ok := playreadyMap["supported"].(bool); ok {
				detail.Supported = supportedFlag
				if supportedFlag {
					supported.Supported = true
				}
			}
			if licenseURL, ok := playreadyMap["license_url"].(string); ok {
				detail.LicenseURL = licenseURL
			}
			if offlineLicense, ok := playreadyMap["offline_license"].(bool); ok {
				detail.OfflineLicense = offlineLicense
			}
			if maxRes, ok := playreadyMap["max_resolution"].(string); ok {
				detail.MaxResolution = maxRes
			}
			if securityLevel, ok := playreadyMap["security_level"].(string); ok {
				detail.SecurityLevel = securityLevel
			}
			supported.Details = append(supported.Details, detail)
		}
	}

	// Check for FairPlay specifically
	if fairplay, ok := emeMap["fairplay"]; ok {
		if fairplayMap, ok := fairplay.(map[string]interface{}); ok {
			detail := DRMSystemDetail{
				System: "FairPlay",
			}
			if supportedFlag, ok := fairplayMap["supported"].(bool); ok {
				detail.Supported = supportedFlag
				if supportedFlag {
					supported.Supported = true
				}
			}
			if licenseURL, ok := fairplayMap["license_url"].(string); ok {
				detail.LicenseURL = licenseURL
			}
			if offlineLicense, ok := fairplayMap["offline_license"].(bool); ok {
				detail.OfflineLicense = offlineLicense
			}
			if maxRes, ok := fairplayMap["max_resolution"].(string); ok {
				detail.MaxResolution = maxRes
			}
			if securityLevel, ok := fairplayMap["security_level"].(string); ok {
				detail.SecurityLevel = securityLevel
			}
			supported.Details = append(supported.Details, detail)
		}
	}

	return supported, nil
}

// GetDRMKeySystem returns the key system string for a DRM type
func GetDRMKeySystem(drmType string) string {
	switch strings.ToLower(drmType) {
	case "widevine":
		return "com.widevine.alpha"
	case "playready":
		return "com.microsoft.playready"
	case "fairplay":
		return "com.apple.fps"
	default:
		return ""
	}
}

// GetDRMTypeFromKeySystem returns the DRM type from a key system string
func GetDRMTypeFromKeySystem(keySystem string) string {
	switch keySystem {
	case "com.widevine.alpha":
		return "widevine"
	case "com.microsoft.playready":
		return "playready"
	case "com.apple.fps":
		return "fairplay"
	default:
		return ""
	}
}

// IsDRMSupported checks if a specific DRM system is in the supported list
func IsDRMSupported(supported *DRMSupported, drmType string) bool {
	if supported == nil {
		return false
	}
	for _, system := range supported.Systems {
		if strings.EqualFold(system, drmType) {
			return true
		}
	}
	for _, detail := range supported.Details {
		if strings.EqualFold(detail.System, drmType) && detail.Supported {
			return true
		}
	}
	return false
}
