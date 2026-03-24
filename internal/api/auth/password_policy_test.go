// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package auth

import (
	"strings"
	"testing"
)

func TestPasswordPolicyValidate(t *testing.T) {
	policy := DefaultPasswordPolicy()

	tests := []struct {
		name      string
		password  string
		wantValid bool
	}{
		{
			name:      "valid password",
			password:  "MyStr0ng!Pass",
			wantValid: true,
		},
		{
			name:      "too short",
			password:  "Pass1",
			wantValid: false,
		},
		{
			name:      "no uppercase",
			password:  "password123",
			wantValid: false,
		},
		{
			name:      "no lowercase",
			password:  "PASSWORD123",
			wantValid: false,
		},
		{
			name:      "no digit",
			password:  "PasswordABC",
			wantValid: false,
		},
		{
			name:      "repeated characters",
			password:  "Passsssss1",
			wantValid: false,
		},
		{
			name:      "sequential characters",
			password:  "Password1234",
			wantValid: false, // "1234" is sequential
		},
		{
			name:      "common password base",
			password:  "Password!",
			wantValid: false, // "password" is common
		},
		{
			name:      "long enough with all requirements",
			password:  "MyStr0ng!Password",
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, _ := policy.Validate(tt.password)
			if valid != tt.wantValid {
				t.Errorf("Validate(%q) = %v, want %v", tt.password, valid, tt.wantValid)
			}
		})
	}
}

func TestPasswordPolicyValidateIssues(t *testing.T) {
	policy := DefaultPasswordPolicy()

	valid, issues := policy.Validate("abc")
	if valid {
		t.Error("expected 'abc' to be invalid")
	}
	if len(issues) == 0 {
		t.Error("expected issues for 'abc'")
	}
}

func TestPasswordStrength(t *testing.T) {
	tests := []struct {
		password string
		minScore int
		maxScore int
	}{
		{"", 0, 0},
		{"x", 5, 20},
		{"password", 5, 20},
		{"Password1", 10, 60},
		{"Password123!", 40, 100},
		{"MyStr0ng!P@ssw0rd", 50, 100},
		{"TrulyRandom#2024!Str0ng", 60, 100},
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			score := PasswordStrength(tt.password)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("PasswordStrength(%q) = %d, want between %d and %d", tt.password, score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestStrengthDescription(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{0, "very weak"},
		{10, "very weak"},
		{19, "very weak"},
		{20, "weak"},
		{30, "weak"},
		{39, "weak"},
		{40, "fair"},
		{50, "fair"},
		{59, "fair"},
		{60, "good"},
		{70, "good"},
		{79, "good"},
		{80, "strong"},
		{90, "strong"},
		{100, "strong"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			desc := StrengthDescription(tt.score)
			if desc != tt.expected {
				t.Errorf("StrengthDescription(%d) = %s, want %s", tt.score, desc, tt.expected)
			}
		})
	}
}

func TestIsCommonPassword(t *testing.T) {
	tests := []struct {
		password string
		expected bool
	}{
		{"password", true},
		{"PASSWORD", true},
		{"Password123", true},
		{"password1", true},
		{"admin", true},
		{"letmein", true},
		{"qwerty", true},
		{"abcdefg", false},
		{"xyz12345", false},
		{"MyStr0ngP@ssw0rd", false},
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			result := IsCommonPassword(tt.password)
			if result != tt.expected {
				t.Errorf("IsCommonPassword(%q) = %v, want %v", tt.password, result, tt.expected)
			}
		})
	}
}

func TestIsCommonPasswordWithSuffix(t *testing.T) {
	// "letmein" is common, "letmein1", "letmein123" should also be detected
	tests := []string{
		"letmein1",
		"letmein123",
		"admin1",
		"admin123",
		"password99",
	}

	for _, pw := range tests {
		if !IsCommonPassword(pw) {
			t.Errorf("expected IsCommonPassword(%q) to be true", pw)
		}
	}
}

func TestStrictPasswordPolicy(t *testing.T) {
	policy := StrictPasswordPolicy()

	tests := []struct {
		password  string
		wantValid bool
	}{
		{"Comp1ex!Pass", true},      // valid strict password
		{"MySecureP@ss1", true},
		{"Xy7#zQ9!Abc", false},       // check if special chars are counted correctly
		{"P@ssw0rd!", false},   // common password base
		{"SimplePass1!", true},        // has special, all requirements
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			valid, issues := policy.Validate(tt.password)
			if valid != tt.wantValid {
				t.Logf("issues for %q: %v", tt.password, issues)
				t.Errorf("StrictPasswordPolicy.Validate(%q) = %v, want %v", tt.password, valid, tt.wantValid)
			}
		})
	}
}

func TestPasswordPolicyCustomConfig(t *testing.T) {
	policy := PasswordPolicy{
		MinLength:      6,
		MaxLength:      20,
		RequireUpper:   false,
		RequireLower:   false,
		RequireDigit:   false,
		RequireSpecial: false,
		CheckCommon:    false,
	}

	valid, issues := policy.Validate("abc123")
	if !valid {
		t.Errorf("expected 'abc123' to be valid with custom policy, got issues: %v", issues)
	}

	valid, issues = policy.Validate("ab")
	if valid {
		t.Error("expected 'ab' to be invalid (too short)")
	}
}

func TestPasswordStrengthBonus(t *testing.T) {
	// Password with all character types should score higher
	onlyLower := PasswordStrength("password")
	withUpper := PasswordStrength("Password")
	withDigits := PasswordStrength("Password1")
	withSpecial := PasswordStrength("Password1!")

	if withUpper <= onlyLower {
		t.Error("uppercase should increase score")
	}
	if withDigits <= withUpper {
		t.Error("digits should increase score")
	}
	if withSpecial <= withDigits {
		t.Error("special chars should increase score")
	}
}

func TestPasswordWithRepeatedChars(t *testing.T) {
	tests := []struct {
		password string
		repeated bool
	}{
		{"aaaaaa", true},
		{"aaaabbb", true}, // 4 a's in a row
		{"Password1", false},
		{"Paass", false}, // only 3 repeats, not 4
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			result := hasRepeatedChars(tt.password, 4)
			if result != tt.repeated {
				t.Errorf("hasRepeatedChars(%q) = %v, want %v", tt.password, result, tt.repeated)
			}
		})
	}
}

func TestPasswordWithSequentialChars(t *testing.T) {
	tests := []struct {
		password string
		sequential bool
	}{
		{"abcd1234", true},   // abcd is sequential
		{"12345678", true},    // all sequential
		{"password1", false},
		{"abc12345", true},  // abc is 3 chars, and 123 is also 3 chars - let me check
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			result := hasSequentialChars(tt.password, 4)
			if result != tt.sequential {
				t.Errorf("hasSequentialChars(%q) = %v, want %v", tt.password, result, tt.sequential)
			}
		})
	}
}

func TestCommonPasswordsCoverage(t *testing.T) {
	// Ensure common passwords list has entries
	if len(commonPasswords) == 0 {
		t.Error("commonPasswords map should not be empty")
	}

	// Check a few known passwords
	knownCommon := []string{"123456", "password", "qwerty", "admin"}
	for _, pw := range knownCommon {
		if !commonPasswords[strings.ToLower(pw)] {
			t.Errorf("expected %q to be in commonPasswords", pw)
		}
	}
}
