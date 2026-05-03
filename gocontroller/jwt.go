package gocontroller

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type jwtContextKey string

const jwtClaimsContextKey jwtContextKey = "gocontroller.jwt_claims"

// JWTClaims represents decoded JWT payload.
type JWTClaims map[string]any

// Subject returns the "sub" claim.
func (c JWTClaims) Subject() string {
	v, _ := c["sub"].(string)
	return v
}

// Issuer returns the "iss" claim.
func (c JWTClaims) Issuer() string {
	v, _ := c["iss"].(string)
	return v
}

// Audience returns the "aud" claim.
func (c JWTClaims) Audience() []string {
	switch v := c["aud"].(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, a := range v {
			if s, ok := a.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// ExpiresAt returns the "exp" claim as time.Time.
func (c JWTClaims) ExpiresAt() (time.Time, bool) {
	v, ok := c["exp"].(float64)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(int64(v), 0), true
}

// IssuedAt returns the "iat" claim as time.Time.
func (c JWTClaims) IssuedAt() (time.Time, bool) {
	v, ok := c["iat"].(float64)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(int64(v), 0), true
}

// JWTConfig configures JWT verification.
type JWTConfig struct {
	SecretKey       []byte
	PublicKey       *rsa.PublicKey
	Algorithm       string
	ClaimsValidator func(JWTClaims) error
	HeaderName      string
	SchemePrefix    string
}

func (c *JWTConfig) applyDefaults() {
	if c.Algorithm == "" {
		c.Algorithm = "HS256"
	}
	if c.HeaderName == "" {
		c.HeaderName = "Authorization"
	}
	if c.SchemePrefix == "" {
		c.SchemePrefix = "Bearer"
	}
}

// JWT returns a middleware that validates JWT tokens from the Authorization header.
func JWT(cfg JWTConfig) Middleware {
	cfg.applyDefaults()

	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			tokenStr := extractBearerToken(ctx.Request.Header.Get(cfg.HeaderName), cfg.SchemePrefix)
			if tokenStr == "" {
				return UnauthorizedError("missing or invalid authorization header")
			}

			claims, err := verifyJWT(tokenStr, cfg)
			if err != nil {
				return UnauthorizedError("invalid token")
			}

			if cfg.ClaimsValidator != nil {
				if err := cfg.ClaimsValidator(claims); err != nil {
					return UnauthorizedError(err.Error())
				}
			}

			ctx.Request = ctx.Request.WithContext(newContextWithClaims(ctx.Request.Context(), jwtClaimsContextKey, claims))
			return next(ctx)
		}
	}
}

func extractBearerToken(header, scheme string) string {
	if header == "" {
		return ""
	}
	prefix := scheme + " "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func verifyJWT(tokenStr string, cfg JWTConfig) (JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	headerJSON, err := decodeSegment(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}

	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}

	if header.Alg != cfg.Algorithm {
		return nil, fmt.Errorf("unexpected algorithm: %s", header.Alg)
	}

	claimsJSON, err := decodeSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}

	signingInput := []byte(parts[0] + "." + parts[1])
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	if err := verifySignature(signingInput, signature, header.Alg, cfg); err != nil {
		return nil, fmt.Errorf("verify signature: %w", err)
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	if exp, ok := claims.ExpiresAt(); ok && time.Now().After(exp) {
		return nil, errors.New("token expired")
	}

	return claims, nil
}

func verifySignature(signingInput, signature []byte, alg string, cfg JWTConfig) error {
	switch alg {
	case "HS256":
		if len(cfg.SecretKey) < 32 {
			return fmt.Errorf("secret key must be at least 32 bytes for HS256")
		}
		return verifyHS256(signingInput, signature, cfg.SecretKey)
	case "RS256":
		return verifyRS256(signingInput, signature, cfg.PublicKey)
	default:
		return fmt.Errorf("unsupported algorithm: %s", alg)
	}
}

func decodeSegment(seg string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(seg)
}

// JWTClaimsFromContext extracts JWT claims from request context.
func JWTClaimsFromContext(ctx *Context) (JWTClaims, bool) {
	return ContextValue[JWTClaims](ctx, jwtClaimsContextKey)
}

// RequireJWT ensures a valid JWT is present before proceeding.
func RequireJWT(cfg JWTConfig) Middleware {
	return JWT(cfg)
}
