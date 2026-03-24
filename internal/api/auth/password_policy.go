// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package auth

import (
	"strings"
	"unicode"
)

// PasswordPolicy defines password validation rules
type PasswordPolicy struct {
	MinLength       int  // Minimum length (default: 8)
	MaxLength       int  // Maximum length (default: 128)
	RequireUpper    bool // Require uppercase letters
	RequireLower    bool // Require lowercase letters
	RequireDigit    bool // Require digits
	RequireSpecial  bool // Require special characters
	CheckCommon     bool // Check against common passwords
}

// DefaultPasswordPolicy returns the default password policy
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:      8,
		MaxLength:      128,
		RequireUpper:   true,
		RequireLower:   true,
		RequireDigit:   true,
		RequireSpecial: false,
		CheckCommon:    true,
	}
}

// StrictPasswordPolicy returns a strict password policy
func StrictPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:      12,
		MaxLength:      128,
		RequireUpper:   true,
		RequireLower:   true,
		RequireDigit:   true,
		RequireSpecial: true,
		CheckCommon:    true,
	}
}

// ValidatePassword validates a password against the policy
func (p *PasswordPolicy) Validate(password string) (valid bool, issues []string) {
	issues = []string{}

	// Length check
	if len(password) < p.MinLength {
		issues = append(issues, "password must be at least 8 characters")
	}
	if p.MaxLength > 0 && len(password) > p.MaxLength {
		issues = append(issues, "password must be no more than 128 characters")
	}

	// Check for empty password
	if len(password) == 0 {
		issues = append(issues, "password cannot be empty")
		return false, issues
	}

	// Character type checks
	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if p.RequireUpper && !hasUpper {
		issues = append(issues, "password must contain at least one uppercase letter")
	}
	if p.RequireLower && !hasLower {
		issues = append(issues, "password must contain at least one lowercase letter")
	}
	if p.RequireDigit && !hasDigit {
		issues = append(issues, "password must contain at least one digit")
	}
	if p.RequireSpecial && !hasSpecial {
		issues = append(issues, "password must contain at least one special character")
	}

	// Check for common passwords
	if p.CheckCommon && IsCommonPassword(password) {
		issues = append(issues, "password is too common, please choose a stronger password")
	}

	// Check for repeated characters (e.g., "aaaaaa")
	if hasRepeatedChars(password, 4) {
		issues = append(issues, "password contains too many repeated characters")
	}

	// Check for sequential characters (e.g., "12345678", "abcdefgh")
	if hasSequentialChars(password, 4) {
		issues = append(issues, "password contains too many sequential characters")
	}

	return len(issues) == 0, issues
}

// hasRepeatedChars checks if password has repeated characters
func hasRepeatedChars(password string, maxRepeat int) bool {
	if len(password) < maxRepeat {
		return false
	}

	count := 1
	for i := 1; i < len(password); i++ {
		if password[i] == password[i-1] {
			count++
			if count >= maxRepeat {
				return true
			}
		} else {
			count = 1
		}
	}
	return false
}

// hasSequentialChars checks if password has sequential characters
func hasSequentialChars(password string, maxSeq int) bool {
	if len(password) < maxSeq {
		return false
	}

	for i := 0; i <= len(password)-maxSeq; i++ {
		seq := 1
		for j := 1; j < maxSeq; j++ {
			if password[i+j] == password[i+j-1]+1 || password[i+j] == password[i+j-1]-1 {
				seq++
			} else {
				break
			}
		}
		if seq >= maxSeq {
			return true
		}
	}
	return false
}

// PasswordStrength returns a password strength score (0-100)
func PasswordStrength(password string) int {
	if len(password) == 0 {
		return 0
	}

	score := 0

	// Length scoring
	switch {
	case len(password) >= 16:
		score += 30
	case len(password) >= 12:
		score += 25
	case len(password) >= 10:
		score += 20
	case len(password) >= 8:
		score += 15
	case len(password) >= 6:
		score += 10
	default:
		score += 5
	}

	// Character variety scoring
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if hasUpper {
		score += 15
	}
	if hasLower {
		score += 15
	}
	if hasDigit {
		score += 15
	}
	if hasSpecial {
		score += 20
	}

	// Bonus for mixing character types
	typeCount := 0
	if hasUpper {
		typeCount++
	}
	if hasLower {
		typeCount++
	}
	if hasDigit {
		typeCount++
	}
	if hasSpecial {
		typeCount++
	}

	switch typeCount {
	case 4:
		score += 5 // All types
	case 3:
		score += 3 // Three types
	}

	// Penalties
	if IsCommonPassword(password) {
		score = score / 4 // Significant penalty for common passwords
	}
	if hasRepeatedChars(password, 3) {
		score -= 10
	}
	if hasSequentialChars(password, 4) {
		score -= 10
	}

	// Ensure score is within bounds
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// StrengthDescription returns a description of the password strength
func StrengthDescription(score int) string {
	switch {
	case score >= 80:
		return "strong"
	case score >= 60:
		return "good"
	case score >= 40:
		return "fair"
	case score >= 20:
		return "weak"
	default:
		return "very weak"
	}
}

// Common passwords list (top 100) - these should be checked
// In production, use a more comprehensive list
var commonPasswords = map[string]bool{
	"123456":       true,
	"password":     true,
	"12345678":     true,
	"qwerty":       true,
	"123456789":    true,
	"12345":        true,
	"1234":         true,
	"111111":       true,
	"1234567":      true,
	"dragon":       true,
	"123123":       true,
	"baseball":     true,
	"iloveyou":     true,
	"trustno1":     true,
	"sunshine":     true,
	"master":       true,
	"welcome":      true,
	"shadow":       true,
	"ashley":       true,
	"football":     true,
	"jesus":        true,
	"michael":      true,
	"ninja":        true,
	"mustang":      true,
	"password1":    true,
	"password123":  true,
	"letmein":      true,
	"abc123":       true,
	"monkey":       true,
	"master123":    true,
	"admin":        true,
	"admin123":     true,
	"root":         true,
	"root123":      true,
	"pass":         true,
	"pass123":      true,
	"test":         true,
	"test123":      true,
	"guest":        true,
	"changeme":     true,
	"default":      true,
	"welcome1":     true,
	"welcome123":   true,
	"letmein123":   true,
	"qwerty123":    true,
	"hello":        true,
	"hello123":     true,
	"secret":       true,
	"secret123":    true,
	"access":       true,
	"access123":    true,
	"login":        true,
	"login123":     true,
	"passw0rd":     true,
	"p@ssw0rd":    true,
	"p@ssword":    true,
	"password!":    true,
	"Password1":    true,
	"Password123":  true,
	"Welcome1":     true,
	"Summer2024":   true,
	"Winter2024":   true,
	"Spring2024":   true,
	"Fall2024":     true,
	"January":      true,
	"February":     true,
	"March":        true,
	"April":        true,
	"May":          true,
	"June":         true,
	"July":         true,
	"August":       true,
	"September":    true,
	"October":      true,
	"November":     true,
	"December":     true,
}

// IsCommonPassword checks if a password is in the common passwords list
func IsCommonPassword(password string) bool {
	// Check exact match (case-insensitive)
	if commonPasswords[strings.ToLower(password)] {
		return true
	}

	// Check with common number suffixes
	base := strings.TrimRight(password, "0123456789")
	if base != "" && commonPasswords[strings.ToLower(base)] {
		return true
	}

	return false
}
