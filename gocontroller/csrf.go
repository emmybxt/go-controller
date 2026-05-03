package gocontroller

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

const csrfCookieName = "gocontroller_csrf"
const csrfHeaderName = "X-CSRF-Token"
const csrfFormField = "_csrf"
const csrfContextKey = "gocontroller.csrf_token"

// CSRFConfig configures CSRF protection.
type CSRFConfig struct {
	Secret        []byte
	CookieName    string
	HeaderName    string
	FormField     string
	Secure        bool
	SameSite      http.SameSite
	Path          string
	Domain        string
	MaxAge        time.Duration
	IgnoreMethods []string
	ErrorHandler  func(*Context) error
}

func (c *CSRFConfig) applyDefaults() {
	if len(c.Secret) == 0 {
		c.Secret = make([]byte, 32)
		if _, err := rand.Read(c.Secret); err != nil {
			panic("csrf: failed to generate random secret")
		}
	}
	if c.CookieName == "" {
		c.CookieName = csrfCookieName
	}
	if c.HeaderName == "" {
		c.HeaderName = csrfHeaderName
	}
	if c.FormField == "" {
		c.FormField = csrfFormField
	}
	if c.Path == "" {
		c.Path = "/"
	}
	if c.MaxAge <= 0 {
		c.MaxAge = 24 * time.Hour
	}
	if len(c.IgnoreMethods) == 0 {
		c.IgnoreMethods = []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace}
	}
	if c.ErrorHandler == nil {
		c.ErrorHandler = func(ctx *Context) error {
			return ForbiddenError("invalid CSRF token")
		}
	}
}

// CSRF returns a middleware that protects against cross-site request forgery.
func CSRF(cfg CSRFConfig) Middleware {
	cfg.applyDefaults()

	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			token := tokenFromCookie(ctx, &cfg)

			if !isMethodSafe(ctx.Request.Method, cfg.IgnoreMethods) {
				if !validateToken(ctx, submittedToken(ctx, &cfg), &cfg) {
					return cfg.ErrorHandler(ctx)
				}
			}

			if token == "" {
				token = generateCSRFToken()
			}

			ctx.Set(csrfContextKey, token)

			http.SetCookie(ctx.ResponseWriter, &http.Cookie{
				Name:     cfg.CookieName,
				Value:    maskToken(token, cfg.Secret),
				Path:     cfg.Path,
				Domain:   cfg.Domain,
				MaxAge:   int(cfg.MaxAge.Seconds()),
				Secure:   cfg.Secure,
				HttpOnly: false,
				SameSite: cfg.SameSite,
			})

			return next(ctx)
		}
	}
}

func getToken(ctx *Context, cfg *CSRFConfig) string {
	if token := submittedToken(ctx, cfg); token != "" {
		return token
	}
	return tokenFromCookie(ctx, cfg)
}

func submittedToken(ctx *Context, cfg *CSRFConfig) string {
	if token := ctx.Request.Header.Get(cfg.HeaderName); token != "" {
		return token
	}
	if token := ctx.Request.FormValue(cfg.FormField); token != "" {
		return token
	}
	return ""
}

func tokenFromCookie(ctx *Context, cfg *CSRFConfig) string {
	if cookie, err := ctx.Request.Cookie(cfg.CookieName); err == nil {
		token, err := unmaskToken(cookie.Value, cfg.Secret)
		if err == nil {
			return token
		}
	}
	return ""
}

func validateToken(ctx *Context, rawToken string, cfg *CSRFConfig) bool {
	if rawToken == "" {
		return false
	}

	cookie, err := ctx.Request.Cookie(cfg.CookieName)
	if err != nil {
		return false
	}

	actual, err := unmaskToken(cookie.Value, cfg.Secret)
	if err != nil {
		return false
	}

	if actual == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(rawToken), []byte(actual)) == 1
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("csrf: failed to generate random token")
	}
	return base64.StdEncoding.EncodeToString(b)
}

func maskToken(token string, secret []byte) string {
	mask := make([]byte, len(token))
	if _, err := rand.Read(mask); err != nil {
		panic("csrf: failed to generate mask")
	}
	masked := make([]byte, len(token))
	for i := 0; i < len(token); i++ {
		masked[i] = token[i] ^ mask[i]
	}
	result := make([]byte, len(mask)+len(masked))
	copy(result, mask)
	copy(result[len(mask):], masked)
	payload := base64.RawURLEncoding.EncodeToString(result)
	return payload + "." + signCSRFToken(token, secret)
}

func unmaskToken(masked string, secret []byte) (string, error) {
	payload, sig, ok := strings.Cut(masked, ".")
	if !ok {
		return "", nil
	}
	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", err
	}
	if len(data) < 2 || len(data)%2 != 0 {
		return "", nil
	}
	maskLen := len(data) / 2
	mask := data[:maskLen]
	token := data[maskLen:]
	unmasked := make([]byte, len(token))
	for i := 0; i < len(token); i++ {
		unmasked[i] = token[i] ^ mask[i]
	}
	value := string(unmasked)
	if !validCSRFSignature(value, sig, secret) {
		return "", nil
	}
	return value, nil
}

func signCSRFToken(token string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func validCSRFSignature(token, sig string, secret []byte) bool {
	expected, err := hex.DecodeString(signCSRFToken(token, secret))
	if err != nil {
		return false
	}
	actual, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	return hmac.Equal(actual, expected)
}

func isMethodSafe(method string, ignoreMethods []string) bool {
	for _, m := range ignoreMethods {
		if method == m {
			return true
		}
	}
	return false
}

// CSRFToken returns the current request's CSRF token.
func (c *Context) CSRFToken() string {
	v, ok := c.Get(csrfContextKey)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// CSRFTokenMiddleware adds CSRF token to response header for SPAs.
func CSRFTokenHeader() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			if token := ctx.CSRFToken(); token != "" {
				ctx.ResponseWriter.Header().Set("X-CSRF-Token", token)
			}
			return next(ctx)
		}
	}
}
