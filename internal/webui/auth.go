package webui

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const sessionCookieMaxAge = 7 * 24 * 3600

type Auth struct {
	token      string
	cookieName string
}

func NewAuth(token, cookieName string) *Auth {
	return &Auth{token: token, cookieName: cookieName}
}

func (a *Auth) ValidToken(token string) bool {
	if a.token == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a.token), []byte(token)) == 1
}

func (a *Auth) Authenticated(r *http.Request) bool {
	c, err := r.Cookie(a.cookieName)
	if err != nil {
		return false
	}
	return a.ValidToken(c.Value)
}

func (a *Auth) SetSession(w http.ResponseWriter, r *http.Request, token string) {
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   sessionCookieMaxAge,
	})
}

func (a *Auth) ClearSession(w http.ResponseWriter, r *http.Request) {
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func (a *Auth) UpdateToken(token string) {
	a.token = token
}

func ValidateStartupToken(token string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("webui.token is required when starting WebUI (set in config.yaml or SNAPCRAFT_WEBUI_TOKEN)")
	}
	return nil
}
