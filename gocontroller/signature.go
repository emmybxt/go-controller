package gocontroller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const signatureHeaderName = "X-Signature-256"
const timestampHeaderName = "X-Timestamp"

// SignatureConfig configures request signature verification.
type SignatureConfig struct {
	Secret           []byte
	HeaderName       string
	TimestampHeader  string
	MaxAge           time.Duration
	IncludeBody      bool
	IncludeMethod    bool
	IncludePath      bool
	IncludeQuery     bool
	IncludeHost      bool
	IncludeTimestamp bool
	SkipPaths        []string
	ErrorHandler     func(*Context) error
}

func (c *SignatureConfig) applyDefaults() {
	if c.HeaderName == "" {
		c.HeaderName = signatureHeaderName
	}
	if c.TimestampHeader == "" {
		c.TimestampHeader = timestampHeaderName
	}
	if c.MaxAge <= 0 {
		c.MaxAge = 5 * time.Minute
	}
	if !c.IncludeBody && !c.IncludeMethod && !c.IncludePath {
		c.IncludeMethod = true
		c.IncludePath = true
	}
	if c.ErrorHandler == nil {
		c.ErrorHandler = func(ctx *Context) error {
			return UnauthorizedError("invalid request signature")
		}
	}
}

// RequestSignature returns a middleware that verifies HMAC-SHA256 request signatures.
func RequestSignature(cfg SignatureConfig) Middleware {
	cfg.applyDefaults()

	skipSet := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skipSet[p] = struct{}{}
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			if _, skip := skipSet[ctx.Request.URL.Path]; skip {
				return next(ctx)
			}

			sigHeader := ctx.Request.Header.Get(cfg.HeaderName)
			if sigHeader == "" {
				return cfg.ErrorHandler(ctx)
			}

			sig, err := hex.DecodeString(strings.TrimPrefix(sigHeader, "sha256="))
			if err != nil {
				return cfg.ErrorHandler(ctx)
			}

			tsStr := ctx.Request.Header.Get(cfg.TimestampHeader)
			if tsStr != "" {
				ts, err := time.Parse(time.RFC3339, tsStr)
				if err != nil {
					return cfg.ErrorHandler(ctx)
				}
				if time.Since(ts) > cfg.MaxAge {
					return cfg.ErrorHandler(ctx)
				}
			}

			body, err := signatureBody(ctx.Request, cfg.IncludeBody)
			if err != nil {
				return fmt.Errorf("request signature body: %w", err)
			}

			expected := computeSignature(ctx.Request, &cfg, body)
			if !hmac.Equal(sig, expected) {
				return cfg.ErrorHandler(ctx)
			}

			return next(ctx)
		}
	}
}

// ComputeSignature computes the HMAC-SHA256 signature for a request.
// Use this in client code to generate signatures.
func ComputeSignature(req *http.Request, secret []byte, includeBody, includeMethod, includePath, includeQuery, includeHost bool) []byte {
	cfg := &SignatureConfig{
		Secret:        secret,
		IncludeBody:   includeBody,
		IncludeMethod: includeMethod,
		IncludePath:   includePath,
		IncludeQuery:  includeQuery,
		IncludeHost:   includeHost,
	}
	body, _ := signatureBody(req, includeBody)
	return computeSignature(req, cfg, body)
}

func computeSignature(req *http.Request, cfg *SignatureConfig, body []byte) []byte {
	mac := hmac.New(sha256.New, cfg.Secret)

	if cfg.IncludeTimestamp {
		mac.Write([]byte(req.Header.Get(cfg.TimestampHeader)))
	}
	if cfg.IncludeMethod {
		mac.Write([]byte(req.Method))
	}
	if cfg.IncludeHost {
		mac.Write([]byte(req.Host))
	}
	if cfg.IncludePath {
		mac.Write([]byte(req.URL.Path))
	}
	if cfg.IncludeQuery {
		mac.Write([]byte(req.URL.RawQuery))
	}
	if cfg.IncludeBody {
		mac.Write(body)
	}

	mac.Write([]byte{0})

	return mac.Sum(nil)
}

func signatureBody(req *http.Request, include bool) ([]byte, error) {
	if !include || req == nil || req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// GenerateSignatureHeader generates a signature header value for a request.
func GenerateSignatureHeader(req *http.Request, secret []byte, opts ...func(*SignatureConfig)) string {
	cfg := &SignatureConfig{
		Secret:        secret,
		IncludeMethod: true,
		IncludePath:   true,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	cfg.applyDefaults()

	body, _ := signatureBody(req, cfg.IncludeBody)
	sig := computeSignature(req, cfg, body)
	return "sha256=" + hex.EncodeToString(sig)
}

// WithBodySigning includes the request body in signature computation.
func WithBodySigning(cfg *SignatureConfig) {
	cfg.IncludeBody = true
}

// WithQuerySigning includes the query string in signature computation.
func WithQuerySigning(cfg *SignatureConfig) {
	cfg.IncludeQuery = true
}

// WithHostSigning includes the host in signature computation.
func WithHostSigning(cfg *SignatureConfig) {
	cfg.IncludeHost = true
}

// WithTimestampValidation enables timestamp validation with max age.
func WithTimestampValidation(maxAge time.Duration) func(*SignatureConfig) {
	return func(cfg *SignatureConfig) {
		cfg.MaxAge = maxAge
		cfg.IncludeTimestamp = true
	}
}
