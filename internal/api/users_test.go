// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 Tenkile Contributors

package api

import (
	"testing"
	"time"
)

func TestUserRoles(t *testing.T) {
	if RoleAdmin != "admin" {
		t.Errorf("RoleAdmin = %s, want admin", RoleAdmin)
	}
	if RoleUser != "user" {
		t.Errorf("RoleUser = %s, want user", RoleUser)
	}
	if RoleViewer != "viewer" {
		t.Errorf("RoleViewer = %s, want viewer", RoleViewer)
	}
}

func TestUserStatus(t *testing.T) {
	if UserStatusActive != "active" {
		t.Errorf("UserStatusActive = %s, want active", UserStatusActive)
	}
	if UserStatusLocked != "locked" {
		t.Errorf("UserStatusLocked = %s, want locked", UserStatusLocked)
	}
	if UserStatusDisabled != "disabled" {
		t.Errorf("UserStatusDisabled = %s, want disabled", UserStatusDisabled)
	}
}

func TestUserProfile(t *testing.T) {
	profile := UserProfile{
		ID:        "test-id",
		Username:  "testuser",
		Email:     "test@example.com",
		Role:      RoleAdmin,
		Status:    UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if profile.ID != "test-id" {
		t.Errorf("profile.ID = %s, want test-id", profile.ID)
	}
	if profile.Username != "testuser" {
		t.Errorf("profile.Username = %s, want testuser", profile.Username)
	}
	if profile.Role != RoleAdmin {
		t.Errorf("profile.Role = %s, want %s", profile.Role, RoleAdmin)
	}
}

func TestNullString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		valid    bool
	}{
		{"", "", false},
		{"test", "test", true},
		{"hello world", "hello world", true},
	}

	for _, tt := range tests {
		ns := nullString(tt.input)
		if ns.Valid != tt.valid {
			t.Errorf("nullString(%q).Valid = %v, want %v", tt.input, ns.Valid, tt.valid)
		}
		if ns.Valid && ns.String != tt.expected {
			t.Errorf("nullString(%q).String = %s, want %s", tt.input, ns.String, tt.expected)
		}
	}
}
