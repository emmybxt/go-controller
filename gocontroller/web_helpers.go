package gocontroller

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
)

// NotFoundHTMLOrJSON serves an HTML 404 page for browser requests and JSON for APIs.
func NotFoundHTMLOrJSON(notFoundHTMLFile string, jsonMessage string) http.HandlerFunc {
	if jsonMessage == "" {
		jsonMessage = "Route not found"
	}
	return func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if r.Method == http.MethodGet && strings.Contains(accept, "text/html") && notFoundHTMLFile != "" {
			http.ServeFile(w, r, notFoundHTMLFile)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   jsonMessage,
		})
	}
}

// ServePage returns a handler that serves a single static page from publicDir.
func ServePage(publicDir, pageFile string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(publicDir, pageFile))
	}
}

// HybridOptions controls how WebAPIHandler routes traffic to web or api handlers.
type HybridOptions struct {
	WebExactPaths              []string
	WebPathPrefixes            []string
	TreatSingleSegmentGETAsWeb bool
}

// WebAPIHandler composes web and api handlers, reducing manual path-switch boilerplate.
func WebAPIHandler(web http.Handler, api http.Handler, opts HybridOptions) http.Handler {
	exact := map[string]struct{}{}
	for _, p := range opts.WebExactPaths {
		exact[p] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if _, ok := exact[path]; ok {
			web.ServeHTTP(w, r)
			return
		}

		for _, prefix := range opts.WebPathPrefixes {
			if strings.HasPrefix(path, prefix) {
				web.ServeHTTP(w, r)
				return
			}
		}

		if opts.TreatSingleSegmentGETAsWeb {
			trimmed := strings.Trim(path, "/")
			if r.Method == http.MethodGet && trimmed != "" && !strings.Contains(trimmed, "/") {
				web.ServeHTTP(w, r)
				return
			}
		}

		api.ServeHTTP(w, r)
	})
}
