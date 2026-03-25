// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package probes

import (
	"regexp"
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
)

// FuzzyMatchResult represents a fuzzy match result with score and match details
type FuzzyMatchResult struct {
	Device       *CuratedDevice `json:"device"`
	Score        float64       `json:"score"`
	MatchedOn    []string      `json:"matched_on"`
	MatchType    string        `json:"match_type"`
	YearDetected string        `json:"year_detected"`
}

// VersionMatchResult holds version-aware matching results
type VersionMatchResult struct {
	Original   string             `json:"original"`
	BestMatch  *CuratedDevice     `json:"best_match"`
	AllMatches []*FuzzyMatchResult `json:"all_matches"`
	BaseModel  string             `json:"base_model"`
	Year       string             `json:"year"`
	Variant    string             `json:"variant"`
	Confidence string             `json:"confidence"`
}

var yearPattern1 = regexp.MustCompile(`\(?\s*((?:19|20)\d{2})\s*\)?`)
var yearPattern2 = regexp.MustCompile(`(?i)(?:model|series|gen|generation)\s*[-_]?\s*((?:19|20)\d{2})`)
var yearPattern3 = regexp.MustCompile(`(?i)(?:early|late)\s*((?:19|20)\d{2})`)
var yearPatterns = []*regexp.Regexp{yearPattern1, yearPattern2, yearPattern3}

var variantPattern = regexp.MustCompile(`(?i)([A-Z]{1,3}\d{2,3}[A-Z]?)`)
var numericSuffixPattern = regexp.MustCompile(`(\d{2,4})"?\s*$`)

var manufacturerAbbreviations = map[string]string{
	"ss":        "samsung",
	"sm":        "samsung",
	"lg":        "lg",
	"sony":      "sony",
	"son":       "sony",
	"vizio":     "vizio",
	"viz":       "vizio",
	"tcl":       "tcl",
	"hisense":   "hisense",
	"his":       "hisense",
	"philips":   "philips",
	"panasonic": "panasonic",
	"pan":       "panasonic",
	"sharp":     "sharp",
	"shp":       "sharp",
	"toshiba":   "toshiba",
	"tos":       "toshiba",
	"roku":      "roku",
	"atv":       "android tv",
	"androidtv": "android tv",
}

// NormalizeDeviceName normalizes a device name for better matching
func NormalizeDeviceName(name string) string {
	normalized := strings.ToLower(name)
	normalized = regexp.MustCompile(`[-_]+`).ReplaceAllString(normalized, " ")
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)
	return normalized
}

// ExtractDeviceComponents extracts base components from a device name
func ExtractDeviceComponents(name string) (baseModel, year, variant string) {
	for _, pattern := range yearPatterns {
		if matches := pattern.FindStringSubmatch(name); len(matches) > 1 {
			year = matches[1]
			break
		}
	}

	if matches := variantPattern.FindStringSubmatch(name); len(matches) > 1 {
		variant = strings.ToUpper(matches[1])
	}

	baseModel = yearPattern1.ReplaceAllString(name, "")
	baseModel = variantPattern.ReplaceAllString(baseModel, "")
	baseModel = numericSuffixPattern.ReplaceAllString(baseModel, "")
	baseModel = regexp.MustCompile(`\s+`).ReplaceAllString(baseModel, " ")
	baseModel = strings.TrimSpace(baseModel)

	return baseModel, year, variant
}

// MatchDevice performs fuzzy matching for a device name against the database
func (cd *CuratedDatabase) MatchDevice(deviceName string, platform string, limit int) []*FuzzyMatchResult {
	if limit <= 0 {
		limit = 10
	}

	normalized := NormalizeDeviceName(deviceName)
	baseModel, year, variant := ExtractDeviceComponents(deviceName)
	normalizedBase := NormalizeDeviceName(baseModel)

	cd.mu.RLock()
	defer cd.mu.RUnlock()

	var candidates []*CuratedDevice

	if platform != "" {
		platformLower := strings.ToLower(platform)
		if ids, ok := cd.platformIndex[platformLower]; ok {
			for _, id := range ids {
				if device, ok := cd.devices[id]; ok {
					candidates = append(candidates, device)
				}
			}
		}
	} else {
		candidates = make([]*CuratedDevice, 0, len(cd.devices))
		for _, device := range cd.devices {
			candidates = append(candidates, device)
		}
	}

	var results []*FuzzyMatchResult

	for _, device := range candidates {
		result := &FuzzyMatchResult{
			Device:       device,
			MatchedOn:    []string{},
			YearDetected: year,
		}

		score := 0.0
		deviceNameNorm := NormalizeDeviceName(device.Name)
		modelNorm := NormalizeDeviceName(device.Model)

		if deviceNameNorm == normalized {
			score = 100.0
			result.MatchType = "exact"
			result.MatchedOn = append(result.MatchedOn, "name")
		} else if modelNorm == normalized {
			score = 95.0
			result.MatchType = "exact"
			result.MatchedOn = append(result.MatchedOn, "model")
		} else if normalizedBase == NormalizeDeviceName(device.Model) || normalizedBase == deviceNameNorm {
			score = 90.0
			result.MatchType = "exact"
			result.MatchedOn = append(result.MatchedOn, "base_model")
		} else {
			versionScore, matchType, matchedFields := cd.calculateVersionMatch(deviceName, device, normalized, normalizedBase, year, variant)
			score = versionScore
			result.MatchType = matchType
			result.MatchedOn = matchedFields
		}

		if score > 0 {
			if score > 100 {
				score = 100
			}
			result.Score = score
			results = append(results, result)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		scoreI := results[i].Score
		scoreJ := results[j].Score
		if results[i].Device.Verified {
			scoreI += 1.0
		}
		if results[j].Device.Verified {
			scoreJ += 1.0
		}
		return scoreI > scoreJ
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

func (cd *CuratedDatabase) calculateVersionMatch(input string, device *CuratedDevice, normalizedInput, normalizedBase, inputYear, inputVariant string) (float64, string, []string) {
	deviceNameNorm := NormalizeDeviceName(device.Name)
	deviceModelNorm := NormalizeDeviceName(device.Model)

	matchedFields := []string{}
	score := 0.0
	matchType := "fuzzy"

	deviceManufacturerNorm := strings.ToLower(device.Manufacturer)
	inputLower := strings.ToLower(input)
	inputManufacturer := extractManufacturer(input)

	if strings.Contains(deviceManufacturerNorm, inputManufacturer) || strings.Contains(inputLower, deviceManufacturerNorm) {
		score += 20.0
		matchedFields = append(matchedFields, "manufacturer")
	}

	baseSimilarity := calculateStringSimilarity(normalizedBase, deviceModelNorm)
	if baseSimilarity > 0.7 {
		score += baseSimilarity * 30.0
		matchedFields = append(matchedFields, "model_similarity")
	}

	_, _, deviceVariant := ExtractDeviceComponents(device.Name)
	if inputVariant != "" && deviceVariant != "" {
		if inputVariant == deviceVariant {
			score += 25.0
			matchedFields = append(matchedFields, "variant")
		} else if strings.HasPrefix(inputVariant, deviceVariant) || strings.HasPrefix(deviceVariant, inputVariant) {
			score += 15.0
			matchedFields = append(matchedFields, "variant_prefix")
		}
	}

	if inputYear != "" {
		deviceBase, deviceYear, _ := ExtractDeviceComponents(device.Name)
		if deviceYear != "" {
			if inputYear == deviceYear {
				score += 20.0
				matchedFields = append(matchedFields, "year")
			} else {
				score += 10.0
				matchedFields = append(matchedFields, "year_approx")
			}
		} else if deviceBase == normalizedBase {
			score += 5.0
			matchedFields = append(matchedFields, "base_model_only")
		}
	}

	chars := []string{deviceNameNorm, deviceModelNorm}
	for _, ch := range chars {
		matches := fuzzy.Find(normalizedInput, []string{ch})
		if len(matches) > 0 && matches[0].Score > 0 {
			fuzzyScore := float64(matches[0].Score) / float64(len(deviceNameNorm)+len(normalizedInput))
			score += fuzzyScore * 15.0
			if fuzzyScore > 0.5 {
				matchedFields = append(matchedFields, "fuzzy")
			}
		}
	}

	// Cap score at 100 to keep it in [0,100] range
	if score > 100 {
		score = 100
	}

	if score >= 80 {
		matchType = "version"
	} else if score >= 50 {
		matchType = "fuzzy"
	} else if score >= 30 {
		matchType = "partial"
	}

	return score, matchType, matchedFields
}

func calculateStringSimilarity(a, b string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	aBigrams := make(map[string]int)
	bBigrams := make(map[string]int)

	for i := 0; i < len(a)-1; i++ {
		bg := a[i : i+2]
		aBigrams[bg]++
	}

	for i := 0; i < len(b)-1; i++ {
		bg := b[i : i+2]
		bBigrams[bg]++
	}

	common := 0
	for bg, count := range aBigrams {
		if c, ok := bBigrams[bg]; ok {
			common += min(count, c)
		}
	}

	if len(aBigrams)+len(bBigrams) == 0 {
		return 0.0
	}
	dice := 2.0 * float64(common) / float64(len(aBigrams)+len(bBigrams))
	return dice
}

func extractManufacturer(name string) string {
	normalized := NormalizeDeviceName(name)

	manufacturers := []string{
		"samsung", "lg", "sony", "vizio", "tcl", "hisense",
		"philips", "panasonic", "sharp", "toshiba", "roku",
		"insignia", "onida", "sanyo", "element", "westinghouse",
	}

	for _, mfr := range manufacturers {
		if strings.Contains(normalized, mfr) {
			return mfr
		}
	}

	for abbr, full := range manufacturerAbbreviations {
		if strings.Contains(normalized, abbr) {
			return full
		}
	}

	return ""
}

// FindBestMatch returns the best matching device for a given name
func (cd *CuratedDatabase) FindBestMatch(deviceName string, platform string) (*CuratedDevice, string) {
	results := cd.MatchDevice(deviceName, platform, 1)

	if len(results) > 0 && results[0].Score >= 50 {
		confidence := "low"
		if results[0].Score >= 80 {
			confidence = "high"
		} else if results[0].Score >= 60 {
			confidence = "medium"
		}
		return results[0].Device, confidence
	}

	return nil, "none"
}

// VersionAwareMatch performs version-aware matching for device names with years/variants
func (cd *CuratedDatabase) VersionAwareMatch(deviceName string, platform string) *VersionMatchResult {
	result := &VersionMatchResult{
		Original:   deviceName,
		AllMatches: cd.MatchDevice(deviceName, platform, 5),
	}

	baseModel, year, variant := ExtractDeviceComponents(deviceName)
	result.BaseModel = baseModel
	result.Year = year
	result.Variant = variant

	if len(result.AllMatches) == 0 {
		result.Confidence = "none"
		return result
	}

	result.BestMatch = result.AllMatches[0].Device

	score := result.AllMatches[0].Score
	if score >= 80 {
		result.Confidence = "high"
	} else if score >= 60 {
		result.Confidence = "medium"
	} else if score >= 40 {
		result.Confidence = "low"
	} else {
		result.Confidence = "none"
	}

	return result
}

// SearchWithFuzzy performs fuzzy search across all device fields
func (cd *CuratedDatabase) SearchWithFuzzy(query string, platform string, limit int) []*CuratedDevice {
	if limit <= 0 {
		limit = 20
	}

	normalizedQuery := NormalizeDeviceName(query)

	cd.mu.RLock()
	defer cd.mu.RUnlock()

	type scoredDevice struct {
		device *CuratedDevice
		score  int
	}

	var scored []scoredDevice

	type searchableDevice struct {
		device     *CuratedDevice
		searchText string
	}

	var searchCorpus []searchableDevice
	for _, device := range cd.devices {
		if platform != "" && !strings.EqualFold(device.Platform, platform) {
			continue
		}

		searchText := strings.Join([]string{
			device.Name,
			device.Model,
			device.Manufacturer,
			device.Platform,
		}, " ")

		searchCorpus = append(searchCorpus, searchableDevice{
			device:     device,
			searchText: NormalizeDeviceName(searchText),
		})
	}

	strs := make([]string, len(searchCorpus))
	deviceMap := make(map[string]*CuratedDevice)
	for i, sd := range searchCorpus {
		strs[i] = sd.searchText
		deviceMap[sd.searchText] = sd.device
	}

	matches := fuzzy.Find(normalizedQuery, strs)

	for _, match := range matches {
		if match.Index < len(searchCorpus) {
			scored = append(scored, scoredDevice{
				device: searchCorpus[match.Index].device,
				score:  match.Score,
			})
		}
	}

	for _, sd := range searchCorpus {
		if strings.Contains(sd.searchText, normalizedQuery) {
			found := false
			for _, s := range scored {
				if s.device.ID == sd.device.ID {
					found = true
					break
				}
			}
			if !found {
				scored = append(scored, scoredDevice{
					device: sd.device,
					score:  len(normalizedQuery) * 2,
				})
			}
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	devices := make([]*CuratedDevice, 0, len(scored))
	for _, s := range scored {
		devices = append(devices, s.device)
		if len(devices) >= limit {
			break
		}
	}

	return devices
}
