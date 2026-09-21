package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeneratedNativeRoutes(t *testing.T) {
	router, err := newRouter()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/api/books/1", 200}, {"/api/books/featured", 200}, {"/api/books/missing", 404}, {"/health", 204},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", tc.path, nil))
		if response.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.path, response.Code, response.Body)
		}
		if tc.status == 200 && !strings.Contains(response.Body.String(), "The Go Programming Language") {
			t.Fatal(response.Body)
		}
		if tc.path == "/api/books/featured" && response.Header().Get("X-Featured") != "true" {
			t.Fatal("generated middleware missing")
		}
	}
}
