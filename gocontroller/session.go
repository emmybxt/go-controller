package gocontroller

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const sessionCookieName = "gocontroller_session"

// Session stores key-value pairs for a user session.
type Session struct {
	ID        string
	Data      map[string]any
	IsNew     bool
	createdAt time.Time
	maxAge    time.Duration
}

func (s *Session) Set(key string, value any) {
	s.Data[key] = value
}

func (s *Session) Get(key string) (any, bool) {
	v, ok := s.Data[key]
	return v, ok
}

func (s *Session) GetString(key string) (string, bool) {
	v, ok := s.Data[key]
	if !ok {
		return "", false
	}
	str, ok := v.(string)
	return str, ok
}

func (s *Session) Delete(key string) {
	delete(s.Data, key)
}

func (s *Session) IsExpired() bool {
	return time.Since(s.createdAt) > s.maxAge
}

func (s *Session) GenerateID() error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	s.ID = base64.URLEncoding.EncodeToString(b)
	return nil
}

// SessionConfig configures session management.
type SessionConfig struct {
	Secret     []byte
	CookieName string
	MaxAge     time.Duration
	Secure     bool
	HttpOnly   bool
	SameSite   http.SameSite
	Path       string
	Domain     string
}

func (c *SessionConfig) applyDefaults() {
	if len(c.Secret) == 0 || len(c.Secret) < 32 {
		c.Secret = make([]byte, 32)
		if _, err := rand.Read(c.Secret); err != nil {
			panic("session: failed to generate secret")
		}
	}
	if c.CookieName == "" {
		c.CookieName = sessionCookieName
	}
	if c.MaxAge <= 0 {
		c.MaxAge = 24 * time.Hour
	}
	if c.Path == "" {
		c.Path = "/"
	}
	c.HttpOnly = true
}

// SessionMiddleware returns a middleware that manages encrypted cookie sessions.
func SessionMiddleware(cfg SessionConfig) Middleware {
	cfg.applyDefaults()

	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			session := loadSession(ctx, &cfg)

			ctx.Set("gocontroller.session", session)

			err := next(ctx)

			if err == nil && !session.IsExpired() {
				saveSession(ctx, session, &cfg)
			}

			return err
		}
	}
}

func loadSession(ctx *Context, cfg *SessionConfig) *Session {
	cookie, err := ctx.Request.Cookie(cfg.CookieName)
	if err != nil {
		return &Session{
			Data:      make(map[string]any),
			IsNew:     true,
			createdAt: time.Now(),
			maxAge:    cfg.MaxAge,
		}
	}

	data, err := decryptCookie(cookie.Value, cfg.Secret)
	if err != nil {
		return &Session{
			Data:      make(map[string]any),
			IsNew:     true,
			createdAt: time.Now(),
			maxAge:    cfg.MaxAge,
		}
	}

	var sessionData map[string]any
	if err := json.Unmarshal(data, &sessionData); err != nil {
		return &Session{
			Data:      make(map[string]any),
			IsNew:     true,
			createdAt: time.Now(),
			maxAge:    cfg.MaxAge,
		}
	}

	return &Session{
		ID:        cookie.Value[:16],
		Data:      sessionData,
		IsNew:     false,
		createdAt: time.Now(),
		maxAge:    cfg.MaxAge,
	}
}

func saveSession(ctx *Context, session *Session, cfg *SessionConfig) {
	data, err := json.Marshal(session.Data)
	if err != nil {
		return
	}

	encrypted, err := encryptCookie(data, cfg.Secret)
	if err != nil {
		return
	}

	http.SetCookie(ctx.ResponseWriter, &http.Cookie{
		Name:     cfg.CookieName,
		Value:    encrypted,
		Path:     cfg.Path,
		Domain:   cfg.Domain,
		MaxAge:   int(cfg.MaxAge.Seconds()),
		Secure:   cfg.Secure,
		HttpOnly: cfg.HttpOnly,
		SameSite: cfg.SameSite,
	})
}

func encryptCookie(plaintext []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func decryptCookie(encrypted string, key []byte) ([]byte, error) {
	data, err := base64.URLEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// GetSession retrieves the current session from context.
func GetSession(ctx *Context) *Session {
	v, ok := ctx.Get("gocontroller.session")
	if !ok {
		return nil
	}
	s, _ := v.(*Session)
	return s
}

// RegenerateSession creates a new session ID and copies existing data.
func RegenerateSession(ctx *Context, cfg SessionConfig) error {
	cfg.applyDefaults()
	session := GetSession(ctx)
	if session == nil {
		return nil
	}
	if err := session.GenerateID(); err != nil {
		return err
	}
	session.IsNew = false
	saveSession(ctx, session, &cfg)
	return nil
}

// DestroySession clears session data and removes the cookie.
func DestroySession(ctx *Context, cfg SessionConfig) {
	cfg.applyDefaults()
	http.SetCookie(ctx.ResponseWriter, &http.Cookie{
		Name:     cfg.CookieName,
		Value:    "",
		Path:     cfg.Path,
		Domain:   cfg.Domain,
		MaxAge:   -1,
		Secure:   cfg.Secure,
		HttpOnly: cfg.HttpOnly,
		SameSite: cfg.SameSite,
	})
	ctx.Set("gocontroller.session", &Session{
		Data:      make(map[string]any),
		IsNew:     true,
		createdAt: time.Now(),
		maxAge:    cfg.MaxAge,
	})
}
