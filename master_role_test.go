package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func issueMasterRoleTestToken(t *testing.T, username string) string {
	t.Helper()

	oldTokenKey := TokenKey
	TokenKey = []byte("master-role-test-signing-key")
	t.Cleanup(func() { TokenKey = oldTokenKey })

	token, err := GenerateToken(username, []string{"admin"})
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func requireMasterRoleProtectionResponse(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()
	if !strings.Contains(resp.Body.String(), "master 角色只能通过数据库初始化修改") {
		t.Fatalf("unexpected response: %s", resp.Body.String())
	}
}

func TestMasterRoleAssignmentCannotChangeThroughUserUpdate(t *testing.T) {
	setupUserAuthTestDB(t)
	insertUserAuthTestUser(t, 1, "ADMIN", "Admin", "admin", "admin-password")
	insertUserAuthTestUser(t, 2, "USER1", "Normal User", "ham", "user-password")
	insertUserAuthTestUser(t, 3, "MASTER", "Master User", "master", "master-password")
	token := issueMasterRoleTestToken(t, "ADMIN")

	tests := []struct {
		name      string
		targetID  int
		body      string
		wantRoles string
	}{
		{name: "grant", targetID: 2, body: `{"id":2,"roles":["master"]}`, wantRoles: "ham"},
		{name: "revoke", targetID: 3, body: `{"id":3,"roles":["ham"]}`, wantRoles: "master"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/user/update", strings.NewReader(tt.body))
			req.Header.Set("x-token", token)
			resp := httptest.NewRecorder()
			(&jsonapi{}).httpUpdateUser(resp, req)
			requireMasterRoleProtectionResponse(t, resp)

			var roles string
			if err := db.QueryRow(`SELECT roles FROM users WHERE id=?`, tt.targetID).Scan(&roles); err != nil {
				t.Fatal(err)
			}
			if roles != tt.wantRoles {
				t.Fatalf("roles = %q, want %q", roles, tt.wantRoles)
			}
		})
	}
}

func TestMasterRoleCannotBeGrantedThroughUserCreate(t *testing.T) {
	setupUserAuthTestDB(t)
	insertUserAuthTestUser(t, 1, "ADMIN", "Admin", "admin", "admin-password")
	token := issueMasterRoleTestToken(t, "ADMIN")

	req := httptest.NewRequest(http.MethodPost, "/user/create", strings.NewReader(
		`{"callsign":"NEWMASTER","name":"New Master","password":"secret","roles":["master"]}`,
	))
	req.Header.Set("x-token", token)
	resp := httptest.NewRecorder()
	(&jsonapi{}).httpAddUser(resp, req)
	requireMasterRoleProtectionResponse(t, resp)

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM users WHERE callsign='NEWMASTER'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("master user was created through the API")
	}
}

func TestMasterRoleDefinitionCannotChangeThroughRoleAPI(t *testing.T) {
	setupUserAuthTestDB(t)
	insertUserAuthTestUser(t, 1, "ADMIN", "Admin", "admin", "admin-password")
	_, err := db.Exec(`
		CREATE TABLE roles (
			id INTEGER PRIMARY KEY,
			name_key TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			routes TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO roles (id, name_key, name) VALUES (1, 'master', 'Master'), (2, 'ham', 'HAM');
	`)
	if err != nil {
		t.Fatal(err)
	}
	token := issueMasterRoleTestToken(t, "ADMIN")

	tests := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{name: "create", method: http.MethodPost, url: "/roles/create", body: `{"key":"MASTER","name":"Master"}`},
		{name: "rename master", method: http.MethodPut, url: "/roles/create", body: `{"id":1,"key":"ham","name":"Changed"}`},
		{name: "assign master key", method: http.MethodPut, url: "/roles/create", body: `{"id":2,"key":"master","name":"Changed"}`},
		{name: "delete", method: http.MethodDelete, url: "/roles/create?key=master"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-token", token)
			resp := httptest.NewRecorder()
			(&jsonapi{}).httpRole(resp, req)
			requireMasterRoleProtectionResponse(t, resp)
		})
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM roles`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("role count = %d, want 2", count)
	}
	var masterKey, hamKey string
	if err := db.QueryRow(`SELECT name_key FROM roles WHERE id=1`).Scan(&masterKey); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT name_key FROM roles WHERE id=2`).Scan(&hamKey); err != nil {
		t.Fatal(err)
	}
	if masterKey != "master" || hamKey != "ham" {
		t.Fatalf("roles changed: id1=%q id2=%q", masterKey, hamKey)
	}
}
