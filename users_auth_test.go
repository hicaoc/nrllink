package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func setupUserAuthTestDB(t *testing.T) {
	t.Helper()

	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	testDB.SetMaxOpenConns(1)

	_, err = testDB.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			pid TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			phone TEXT NOT NULL DEFAULT '',
			callsign TEXT NOT NULL DEFAULT '',
			gird TEXT NOT NULL DEFAULT '',
			birthday TEXT NOT NULL DEFAULT '',
			mdcid TEXT NOT NULL DEFAULT '',
			dmrid TEXT NOT NULL DEFAULT '',
			sex INTEGER NOT NULL DEFAULT 0,
			nickname TEXT NOT NULL DEFAULT '',
			openid TEXT NOT NULL DEFAULT '',
			avatar TEXT NOT NULL DEFAULT '',
			address TEXT NOT NULL DEFAULT '',
			status INTEGER NOT NULL DEFAULT 1,
			last_login_time TEXT NOT NULL DEFAULT '',
			login_err_times INTEGER NOT NULL DEFAULT 0,
			last_login_ip TEXT NOT NULL DEFAULT '',
			alarm_msg INTEGER NOT NULL DEFAULT 0,
			roles TEXT NOT NULL DEFAULT 'ham',
			create_time TEXT NOT NULL DEFAULT '',
			update_time TEXT NOT NULL DEFAULT '',
			expire_time TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE operator_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT NOT NULL DEFAULT '',
			content TEXT,
			event_type TEXT,
			operator TEXT,
			operator_id INTEGER
		);
	`)
	if err != nil {
		testDB.Close()
		t.Fatal(err)
	}

	oldDB := db
	db = testDB
	t.Cleanup(func() {
		db = oldDB
		testDB.Close()
	})
}

func insertUserAuthTestUser(t *testing.T, id int, callsign, name, roles, password string) {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO users (id, callsign, name, roles, password) VALUES (?, ?, ?, ?, ?)`,
		id, callsign, name, roles, hash,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoginCheckDoesNotLockAccountAfterTenFailures(t *testing.T) {
	setupUserAuthTestDB(t)
	insertUserAuthTestUser(t, 1, "BG1ABC", "Test User", "ham", "correct-password")

	for i := 0; i < 10; i++ {
		if _, ok := loginCheck("wrong-password", "BG1ABC", "192.0.2.1"); ok {
			t.Fatalf("wrong password succeeded on attempt %d", i+1)
		}
	}

	var fails int
	if err := db.QueryRow(`SELECT login_err_times FROM users WHERE id=1`).Scan(&fails); err != nil {
		t.Fatal(err)
	}
	if fails != 10 {
		t.Fatalf("login_err_times = %d, want 10", fails)
	}

	roles, ok := loginCheck("correct-password", "BG1ABC", "192.0.2.1")
	if !ok {
		t.Fatal("correct password rejected after ten failures")
	}
	if len(roles) != 1 || roles[0] != "ham" {
		t.Fatalf("roles = %v, want [ham]", roles)
	}
}

func TestMasterCanResetAdminPasswordOnly(t *testing.T) {
	setupUserAuthTestDB(t)
	insertUserAuthTestUser(t, 1, "MASTER", "Master User", "master", "master-password")
	insertUserAuthTestUser(t, 2, "ADMIN", "Builtin Admin", "admin", "old-password")

	oldTokenKey := TokenKey
	TokenKey = []byte("user-auth-test-signing-key")
	t.Cleanup(func() { TokenKey = oldTokenKey })

	token, err := GenerateToken("MASTER", []string{"master"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/user/update", strings.NewReader(
		`{"id":2,"password":"new-password","name":"Changed Name","callsign":"CHANGED","roles":["ham"],"status":0}`,
	))
	req.Header.Set("x-token", token)
	resp := httptest.NewRecorder()

	(&jsonapi{}).httpUpdateUser(resp, req)

	if !strings.Contains(resp.Body.String(), "密码更新成功") {
		t.Fatalf("unexpected response: %s", resp.Body.String())
	}

	var hash, name, callsign, roles string
	var status int
	err = db.QueryRow(`SELECT password, name, callsign, roles, status FROM users WHERE id=2`).
		Scan(&hash, &name, &callsign, &roles, &status)
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("new-password")); err != nil {
		t.Fatalf("new password was not stored: %v", err)
	}
	if name != "Builtin Admin" || callsign != "ADMIN" || roles != "admin" || status != 1 {
		t.Fatalf("non-password fields changed: name=%q callsign=%q roles=%q status=%d", name, callsign, roles, status)
	}
}
