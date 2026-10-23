package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMasterCanViewFullPhoneInUserList(t *testing.T) {
	setupUserAuthTestDB(t)
	insertUserAuthTestUser(t, 1, "MASTER", "Master User", "master", "master-password")
	insertUserAuthTestUser(t, 2, "USER1", "Normal User", "ham", "user-password")
	insertUserAuthTestUser(t, 3, "TARGET", "Target User", "ham", "target-password")
	if _, err := db.Exec(`UPDATE users SET phone=?, pid=? WHERE id=3`, "13812345678", "110101199001011234"); err != nil {
		t.Fatal(err)
	}

	oldTokenKey := TokenKey
	TokenKey = []byte("user-privacy-test-signing-key")
	t.Cleanup(func() { TokenKey = oldTokenKey })
	masterToken, err := GenerateToken("MASTER", []string{"master"})
	if err != nil {
		t.Fatal(err)
	}
	userToken, err := GenerateToken("USER1", []string{"ham"})
	if err != nil {
		t.Fatal(err)
	}

	type listResponse struct {
		Data struct {
			Items []struct {
				CallSign string `json:"callsign"`
				Phone    string `json:"phone"`
				PID      string `json:"pid"`
			} `json:"items"`
		} `json:"data"`
	}
	fetchTarget := func(t *testing.T, token string) (string, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/user/list", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-token", token)
		resp := httptest.NewRecorder()
		(&jsonapi{}).httpUserList(resp, req)

		var decoded listResponse
		if err := json.Unmarshal(resp.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode response failed: %v, body: %s", err, resp.Body.String())
		}
		for _, item := range decoded.Data.Items {
			if item.CallSign == "TARGET" {
				return item.Phone, item.PID
			}
		}
		t.Fatalf("target user missing from response: %s", resp.Body.String())
		return "", ""
	}

	phone, pid := fetchTarget(t, masterToken)
	if phone != "13812345678" {
		t.Fatalf("master phone = %q, want full phone", phone)
	}
	if pid != maskPID("110101199001011234") {
		t.Fatalf("master pid = %q, want masked PID", pid)
	}

	phone, pid = fetchTarget(t, userToken)
	if phone != maskPhone("13812345678") {
		t.Fatalf("normal user phone = %q, want masked phone", phone)
	}
	if pid != maskPID("110101199001011234") {
		t.Fatalf("normal user pid = %q, want masked PID", pid)
	}
}
