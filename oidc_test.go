package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestTokenFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if tokenFromRequest(req) != "" {
		t.Fatal("empty request should have no token")
	}

	req.Header.Set("Authorization", "Bearer hamid_pat_abc")
	if got := tokenFromRequest(req); got != "hamid_pat_abc" {
		t.Fatalf("bearer token = %q", got)
	}

	req.Header.Del("Authorization")
	req.Header.Set("x-token", "hamid_pat_xyz")
	if got := tokenFromRequest(req); got != "hamid_pat_xyz" {
		t.Fatalf("x-token = %q", got)
	}
}

func TestVerifyLongToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/introspect" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.Form.Get("client_id") != "nrl" || r.Form.Get("client_secret") != "secret" {
			http.Error(w, "invalid client", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"active":true,"sub":"bg1abc","username":"BG1ABC","kind":"user","scope":"api"}`))
	}))
	defer server.Close()

	oldConf := conf.OIDC
	conf.OIDC.Enabled = true
	conf.OIDC.TokenLogin = true
	conf.OIDC.Issuer = server.URL
	conf.OIDC.ClientID = "nrl"
	conf.OIDC.ClientSecret = "secret"
	t.Cleanup(func() { conf.OIDC = oldConf })

	claims, err := verifyLongToken(context.Background(), "hamid_pat_test")
	if err != nil {
		t.Fatal(err)
	}
	if claims.Username != "BG1ABC" || !claims.OIDCVirtual || len(claims.Roles) != 1 || claims.Roles[0] != "ham" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	cached, err := verifyLongToken(context.Background(), "hamid_pat_test")
	if err != nil {
		t.Fatal(err)
	}
	if cached != claims {
		t.Fatal("cached claims should be reused")
	}
	_ = time.Now
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
