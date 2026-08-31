package main

import (
	"strings"
	"testing"
)

func TestVirtualOIDCUserFromInfo(t *testing.T) {
	u := newVirtualOIDCUserFromInfo(&oidcUserInfo{
		Sub:  "bg1abc",
		Name: "Test User",
	})

	if u.CallSign != "BG1ABC" {
		t.Fatalf("callsign = %q, want BG1ABC", u.CallSign)
	}
	if !u.OIDCVirtual || u.ID != 0 || u.Status != 1 {
		t.Fatalf("unexpected virtual user: %+v", u)
	}
	if len(u.Roles) != 1 || u.Roles[0] != "ham" {
		t.Fatalf("roles = %v, want [ham]", u.Roles)
	}
	if u.Groups == nil || u.Groups[1] == nil || u.Groups[2] == nil || u.Groups[3] == nil {
		t.Fatal("virtual user private groups are not initialized")
	}
}

func TestGenerateAndValidateOIDCVirtualToken(t *testing.T) {
	token, err := GenerateOIDCToken("BG1ABC", "Test User", []string{"ham"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if !claims.OIDCVirtual || !strings.EqualFold(claims.Username, "BG1ABC") || claims.Name != "Test User" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	u := virtualOIDCUserFromClaims(claims)
	if !u.OIDCVirtual || u.CallSign != "BG1ABC" || u.Name != "Test User" {
		t.Fatalf("unexpected virtual user: %+v", u)
	}
}
