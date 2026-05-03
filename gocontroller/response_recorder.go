package gocontroller

import (
	"bytes"
	"net/http"
)

// responseRecorder captures the response for caching or inspection.
type responseRecorder struct {
	http.ResponseWriter
	statusCode  int
	body        bytes.Buffer
	headers     http.Header
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.statusCode = code
	r.headers = make(http.Header)
	for k, vals := range r.ResponseWriter.Header() {
		for _, v := range vals {
			r.headers.Add(k, v)
		}
	}
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.body.Write(data)
}
