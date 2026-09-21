package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeneratedDeclarationsMountNativeHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, err := newRouter()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, path, body, user string
		status                   int
	}{
		{"GET", "/api/accounts/42", "", "alice", 200},
		{"POST", "/api/accounts", `{"id":"42"}`, "alice", 201},
		{"GET", "/api/accounts/42", "", "", 401},
		{"POST", "/api/accounts", `{}`, "alice", 400},
	} {
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Demo-User", tc.user)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, response.Code, response.Body)
		}
		if tc.status == 200 || tc.status == 201 {
			var account map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &account); err != nil {
				t.Fatal(err)
			}
			if account["id"] != "42" || account["owner"] != "alice" {
				t.Fatal(account)
			}
		}
		if tc.status == 200 && response.Header().Get("X-Accounts") != "true" {
			t.Fatal("route middleware did not run")
		}
	}
}
